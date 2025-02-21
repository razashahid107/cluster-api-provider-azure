/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scalesets

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/util/generators"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// ScaleSetSpec defines the specification for a Scale Set.
type ScaleSetSpec struct {
	Name                         string
	ResourceGroup                string
	Size                         string
	Capacity                     int64
	SSHKeyData                   string
	OSDisk                       infrav1.OSDisk
	DataDisks                    []infrav1.DataDisk
	SubnetName                   string
	VNetName                     string
	VNetResourceGroup            string
	PublicLBName                 string
	PublicLBAddressPoolName      string
	AcceleratedNetworking        *bool
	TerminateNotificationTimeout *int
	Identity                     infrav1.VMIdentity
	UserAssignedIdentities       []infrav1.UserAssignedIdentity
	SecurityProfile              *infrav1.SecurityProfile
	SpotVMOptions                *infrav1.SpotVMOptions
	AdditionalCapabilities       *infrav1.AdditionalCapabilities
	DiagnosticsProfile           *infrav1.Diagnostics
	FailureDomains               []string
	VMExtensions                 []infrav1.VMExtension
	NetworkInterfaces            []infrav1.NetworkInterface
	IPv6Enabled                  bool
	OrchestrationMode            infrav1.OrchestrationModeType
	Location                     string
	SubscriptionID               string
	SKU                          resourceskus.SKU
	VMSSExtensionSpecs           []azure.ResourceSpecGetter
	VMImage                      *infrav1.Image
	BootstrapData                string
	VMSSInstances                []compute.VirtualMachineScaleSetVM
	MaxSurge                     int
	ClusterName                  string
	ShouldPatchCustomData        bool
	HasReplicasExternallyManaged bool
	AdditionalTags               infrav1.Tags
}

// ResourceName returns the name of the Scale Set.
func (s *ScaleSetSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group for this Scale Set.
func (s *ScaleSetSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for Scale Sets.
func (s *ScaleSetSpec) OwnerResourceName() string {
	return ""
}

func (s *ScaleSetSpec) existingParameters(ctx context.Context, existing interface{}) (parameters interface{}, err error) {
	existingVMSS, ok := existing.(compute.VirtualMachineScaleSet)
	if !ok {
		return nil, errors.Errorf("%T is not a compute.VirtualMachineScaleSet", existing)
	}

	existingInfraVMSS := converters.SDKToVMSS(existingVMSS, s.VMSSInstances)

	params, err := s.Parameters(ctx, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate scale set update parameters for %s", s.Name)
	}

	vmss, ok := params.(compute.VirtualMachineScaleSet)
	if !ok {
		return nil, errors.Errorf("%T is not a compute.VirtualMachineScaleSet", existing)
	}

	vmss.VirtualMachineProfile.NetworkProfile = nil
	vmss.ID = existingVMSS.ID

	hasModelChanges := hasModelModifyingDifferences(&existingInfraVMSS, vmss)
	isFlex := s.OrchestrationMode == infrav1.FlexibleOrchestrationMode
	updated := true
	if !isFlex {
		updated = existingInfraVMSS.HasEnoughLatestModelOrNotMixedModel()
	}
	if s.MaxSurge > 0 && (hasModelChanges || !updated) && !s.HasReplicasExternallyManaged {
		// surge capacity with the intention of lowering during instance reconciliation
		surge := s.Capacity + int64(s.MaxSurge)
		vmss.Sku.Capacity = ptr.To[int64](surge)
	}

	// If there are no model changes and no increase in the replica count, do not update the VMSS.
	// Decreases in replica count is handled by deleting AzureMachinePoolMachine instances in the MachinePoolScope
	if *vmss.Sku.Capacity <= existingInfraVMSS.Capacity && !hasModelChanges && !s.ShouldPatchCustomData {
		// up to date, nothing to do
		return nil, nil
	}

	return vmss, nil
}

// Parameters returns the parameters for the Scale Set.
func (s *ScaleSetSpec) Parameters(ctx context.Context, existing interface{}) (parameters interface{}, err error) {
	if existing != nil {
		return s.existingParameters(ctx, existing)
	}

	if s.AcceleratedNetworking == nil {
		// set accelerated networking to the capability of the VMSize
		accelNet := s.SKU.HasCapability(resourceskus.AcceleratedNetworking)
		s.AcceleratedNetworking = &accelNet
	}

	extensions, err := s.generateExtensions(ctx)
	if err != nil {
		return compute.VirtualMachineScaleSet{}, err
	}

	storageProfile, err := s.generateStorageProfile(ctx)
	if err != nil {
		return compute.VirtualMachineScaleSet{}, err
	}

	securityProfile, err := s.getSecurityProfile()
	if err != nil {
		return compute.VirtualMachineScaleSet{}, err
	}

	priority, evictionPolicy, billingProfile, err := converters.GetSpotVMOptions(s.SpotVMOptions, s.OSDisk.DiffDiskSettings)
	if err != nil {
		return compute.VirtualMachineScaleSet{}, errors.Wrapf(err, "failed to get Spot VM options")
	}

	diagnosticsProfile := converters.GetDiagnosticsProfile(s.DiagnosticsProfile)

	osProfile, err := s.generateOSProfile(ctx)
	if err != nil {
		return compute.VirtualMachineScaleSet{}, err
	}

	orchestrationMode := converters.GetOrchestrationMode(s.OrchestrationMode)

	vmss := compute.VirtualMachineScaleSet{
		Location: ptr.To(s.Location),
		Sku: &compute.Sku{
			Name:     ptr.To(s.Size),
			Tier:     ptr.To("Standard"),
			Capacity: ptr.To[int64](s.Capacity),
		},
		Zones: &s.FailureDomains,
		Plan:  s.generateImagePlan(ctx),
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			OrchestrationMode:    orchestrationMode,
			SinglePlacementGroup: ptr.To(false),
			VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
				OsProfile:          osProfile,
				StorageProfile:     storageProfile,
				SecurityProfile:    securityProfile,
				DiagnosticsProfile: diagnosticsProfile,
				NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: s.getVirtualMachineScaleSetNetworkConfiguration(),
				},
				Priority:       priority,
				EvictionPolicy: evictionPolicy,
				BillingProfile: billingProfile,
				ExtensionProfile: &compute.VirtualMachineScaleSetExtensionProfile{
					Extensions: &extensions,
				},
			},
		},
	}

	// Set properties specific to VMSS orchestration mode
	// See https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-orchestration-modes for more details
	switch orchestrationMode {
	case compute.OrchestrationModeUniform: // Uniform VMSS
		vmss.VirtualMachineScaleSetProperties.Overprovision = ptr.To(false)
		vmss.VirtualMachineScaleSetProperties.UpgradePolicy = &compute.UpgradePolicy{Mode: compute.UpgradeModeManual}
	case compute.OrchestrationModeFlexible: // VMSS Flex, VMs are treated as individual virtual machines
		vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.NetworkProfile.NetworkAPIVersion =
			compute.NetworkAPIVersionTwoZeroTwoZeroHyphenMinusOneOneHyphenMinusZeroOne
		vmss.VirtualMachineScaleSetProperties.PlatformFaultDomainCount = ptr.To[int32](1)
		if len(s.FailureDomains) > 1 {
			vmss.VirtualMachineScaleSetProperties.PlatformFaultDomainCount = ptr.To[int32](int32(len(s.FailureDomains)))
		}
	}

	// Assign Identity to VMSS
	if s.Identity == infrav1.VMIdentitySystemAssigned {
		vmss.Identity = &compute.VirtualMachineScaleSetIdentity{
			Type: compute.ResourceIdentityTypeSystemAssigned,
		}
	} else if s.Identity == infrav1.VMIdentityUserAssigned {
		userIdentitiesMap, err := converters.UserAssignedIdentitiesToVMSSSDK(s.UserAssignedIdentities)
		if err != nil {
			return vmss, errors.Wrapf(err, "failed to assign identity %q", s.Name)
		}
		vmss.Identity = &compute.VirtualMachineScaleSetIdentity{
			Type:                   compute.ResourceIdentityTypeUserAssigned,
			UserAssignedIdentities: userIdentitiesMap,
		}
	}

	// Provisionally detect whether there is any Data Disk defined which uses UltraSSDs.
	// If that's the case, enable the UltraSSD capability.
	for _, dataDisk := range s.DataDisks {
		if dataDisk.ManagedDisk != nil && dataDisk.ManagedDisk.StorageAccountType == string(compute.StorageAccountTypesUltraSSDLRS) {
			vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{
				UltraSSDEnabled: ptr.To(true),
			}
		}
	}

	// Set Additional Capabilities if any is present on the spec.
	if s.AdditionalCapabilities != nil {
		// Set UltraSSDEnabled if a specific value is set on the spec for it.
		if s.AdditionalCapabilities.UltraSSDEnabled != nil {
			vmss.AdditionalCapabilities.UltraSSDEnabled = s.AdditionalCapabilities.UltraSSDEnabled
		}
	}

	if s.TerminateNotificationTimeout != nil {
		vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.ScheduledEventsProfile = &compute.ScheduledEventsProfile{
			TerminateNotificationProfile: &compute.TerminateNotificationProfile{
				NotBeforeTimeout: ptr.To(fmt.Sprintf("PT%dM", *s.TerminateNotificationTimeout)),
				Enable:           ptr.To(true),
			},
		}
	}

	tags := infrav1.Build(infrav1.BuildParams{
		ClusterName: s.ClusterName,
		Lifecycle:   infrav1.ResourceLifecycleOwned,
		Name:        ptr.To(s.Name),
		Role:        ptr.To(infrav1.Node),
		Additional:  s.AdditionalTags,
	})

	vmss.Tags = converters.TagsToMap(tags)
	return vmss, nil
}

func hasModelModifyingDifferences(infraVMSS *azure.VMSS, vmss compute.VirtualMachineScaleSet) bool {
	other := converters.SDKToVMSS(vmss, []compute.VirtualMachineScaleSetVM{})
	return infraVMSS.HasModelChanges(other)
}

func (s *ScaleSetSpec) generateExtensions(ctx context.Context) ([]compute.VirtualMachineScaleSetExtension, error) {
	extensions := make([]compute.VirtualMachineScaleSetExtension, len(s.VMSSExtensionSpecs))
	for i, extensionSpec := range s.VMSSExtensionSpecs {
		extensionSpec := extensionSpec
		parameters, err := extensionSpec.Parameters(ctx, nil)
		if err != nil {
			return nil, err
		}
		vmssextension, ok := parameters.(compute.VirtualMachineScaleSetExtension)
		if !ok {
			return nil, errors.Errorf("%T is not a compute.VirtualMachineScaleSetExtension", parameters)
		}
		extensions[i] = vmssextension
	}

	return extensions, nil
}

func (s *ScaleSetSpec) getVirtualMachineScaleSetNetworkConfiguration() *[]compute.VirtualMachineScaleSetNetworkConfiguration {
	var backendAddressPools []compute.SubResource
	if s.PublicLBName != "" {
		if s.PublicLBAddressPoolName != "" {
			backendAddressPools = append(backendAddressPools,
				compute.SubResource{
					ID: ptr.To(azure.AddressPoolID(s.SubscriptionID, s.ResourceGroup, s.PublicLBName, s.PublicLBAddressPoolName)),
				})
		}
	}
	nicConfigs := []compute.VirtualMachineScaleSetNetworkConfiguration{}
	for i, n := range s.NetworkInterfaces {
		nicConfig := compute.VirtualMachineScaleSetNetworkConfiguration{}
		nicConfig.VirtualMachineScaleSetNetworkConfigurationProperties = &compute.VirtualMachineScaleSetNetworkConfigurationProperties{}
		nicConfig.Name = ptr.To(s.Name + "-nic-" + strconv.Itoa(i))
		nicConfig.EnableIPForwarding = ptr.To(true)
		if n.AcceleratedNetworking != nil {
			nicConfig.VirtualMachineScaleSetNetworkConfigurationProperties.EnableAcceleratedNetworking = n.AcceleratedNetworking
		} else {
			// If AcceleratedNetworking is not specified, use the value from the VMSS spec.
			// It will be set to true if the VMSS SKU supports it.
			nicConfig.VirtualMachineScaleSetNetworkConfigurationProperties.EnableAcceleratedNetworking = s.AcceleratedNetworking
		}

		// Create IPConfigs
		ipconfigs := []compute.VirtualMachineScaleSetIPConfiguration{}
		for j := 0; j < n.PrivateIPConfigs; j++ {
			ipconfig := compute.VirtualMachineScaleSetIPConfiguration{
				Name: ptr.To(fmt.Sprintf("ipConfig" + strconv.Itoa(j))),
				VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
					PrivateIPAddressVersion: compute.IPVersionIPv4,
					Subnet: &compute.APIEntityReference{
						ID: ptr.To(azure.SubnetID(s.SubscriptionID, s.VNetResourceGroup, s.VNetName, n.SubnetName)),
					},
				},
			}

			if j == 0 {
				// Always use the first IPConfig as the Primary
				ipconfig.Primary = ptr.To(true)
			}
			ipconfigs = append(ipconfigs, ipconfig)
		}
		if s.IPv6Enabled {
			ipv6Config := compute.VirtualMachineScaleSetIPConfiguration{
				Name: ptr.To("ipConfigv6"),
				VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
					PrivateIPAddressVersion: compute.IPVersionIPv6,
					Primary:                 ptr.To(false),
					Subnet: &compute.APIEntityReference{
						ID: ptr.To(azure.SubnetID(s.SubscriptionID, s.VNetResourceGroup, s.VNetName, n.SubnetName)),
					},
				},
			}
			ipconfigs = append(ipconfigs, ipv6Config)
		}
		if i == 0 {
			ipconfigs[0].LoadBalancerBackendAddressPools = &backendAddressPools
			nicConfig.VirtualMachineScaleSetNetworkConfigurationProperties.Primary = ptr.To(true)
		}
		nicConfig.VirtualMachineScaleSetNetworkConfigurationProperties.IPConfigurations = &ipconfigs
		nicConfigs = append(nicConfigs, nicConfig)
	}
	return &nicConfigs
}

// generateStorageProfile generates a pointer to a compute.VirtualMachineScaleSetStorageProfile which can utilized for VM creation.
func (s *ScaleSetSpec) generateStorageProfile(ctx context.Context) (*compute.VirtualMachineScaleSetStorageProfile, error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "scalesets.ScaleSetSpec.generateStorageProfile")
	defer done()

	storageProfile := &compute.VirtualMachineScaleSetStorageProfile{
		OsDisk: &compute.VirtualMachineScaleSetOSDisk{
			OsType:       compute.OperatingSystemTypes(s.OSDisk.OSType),
			CreateOption: compute.DiskCreateOptionTypesFromImage,
			DiskSizeGB:   s.OSDisk.DiskSizeGB,
		},
	}

	// enable ephemeral OS
	if s.OSDisk.DiffDiskSettings != nil {
		if !s.SKU.HasCapability(resourceskus.EphemeralOSDisk) {
			return nil, fmt.Errorf("vm size %s does not support ephemeral os. select a different vm size or disable ephemeral os", s.Size)
		}

		storageProfile.OsDisk.DiffDiskSettings = &compute.DiffDiskSettings{
			Option: compute.DiffDiskOptions(s.OSDisk.DiffDiskSettings.Option),
		}
	}

	if s.OSDisk.ManagedDisk != nil {
		storageProfile.OsDisk.ManagedDisk = &compute.VirtualMachineScaleSetManagedDiskParameters{}
		if s.OSDisk.ManagedDisk.StorageAccountType != "" {
			storageProfile.OsDisk.ManagedDisk.StorageAccountType = compute.StorageAccountTypes(s.OSDisk.ManagedDisk.StorageAccountType)
		}
		if s.OSDisk.ManagedDisk.DiskEncryptionSet != nil {
			storageProfile.OsDisk.ManagedDisk.DiskEncryptionSet = &compute.DiskEncryptionSetParameters{ID: ptr.To(s.OSDisk.ManagedDisk.DiskEncryptionSet.ID)}
		}
	}

	if s.OSDisk.CachingType != "" {
		storageProfile.OsDisk.Caching = compute.CachingTypes(s.OSDisk.CachingType)
	}

	dataDisks := make([]compute.VirtualMachineScaleSetDataDisk, len(s.DataDisks))
	for i, disk := range s.DataDisks {
		dataDisks[i] = compute.VirtualMachineScaleSetDataDisk{
			CreateOption: compute.DiskCreateOptionTypesEmpty,
			DiskSizeGB:   ptr.To[int32](disk.DiskSizeGB),
			Lun:          disk.Lun,
			Name:         ptr.To(azure.GenerateDataDiskName(s.Name, disk.NameSuffix)),
		}

		if disk.ManagedDisk != nil {
			dataDisks[i].ManagedDisk = &compute.VirtualMachineScaleSetManagedDiskParameters{
				StorageAccountType: compute.StorageAccountTypes(disk.ManagedDisk.StorageAccountType),
			}

			if disk.ManagedDisk.DiskEncryptionSet != nil {
				dataDisks[i].ManagedDisk.DiskEncryptionSet = &compute.DiskEncryptionSetParameters{ID: ptr.To(disk.ManagedDisk.DiskEncryptionSet.ID)}
			}
		}
	}
	storageProfile.DataDisks = &dataDisks

	if s.VMImage == nil {
		return nil, errors.Errorf("vm image is nil")
	}
	imageRef, err := converters.ImageToSDK(s.VMImage)
	if err != nil {
		return nil, err
	}

	storageProfile.ImageReference = imageRef

	return storageProfile, nil
}

func (s *ScaleSetSpec) generateOSProfile(_ context.Context) (*compute.VirtualMachineScaleSetOSProfile, error) {
	sshKey, err := base64.StdEncoding.DecodeString(s.SSHKeyData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode ssh public key")
	}

	osProfile := &compute.VirtualMachineScaleSetOSProfile{
		ComputerNamePrefix: ptr.To(s.Name),
		AdminUsername:      ptr.To(azure.DefaultUserName),
		CustomData:         ptr.To(s.BootstrapData),
	}

	switch s.OSDisk.OSType {
	case string(compute.OperatingSystemTypesWindows):
		// Cloudbase-init is used to generate a password.
		// https://cloudbase-init.readthedocs.io/en/latest/plugins.html#setting-password-main
		//
		// We generate a random password here in case of failure
		// but the password on the VM will NOT be the same as created here.
		// Access is provided via SSH public key that is set during deployment
		// Azure also provides a way to reset user passwords in the case of need.
		osProfile.AdminPassword = ptr.To(generators.SudoRandomPassword(123))
		osProfile.WindowsConfiguration = &compute.WindowsConfiguration{
			EnableAutomaticUpdates: ptr.To(false),
		}
	default:
		osProfile.LinuxConfiguration = &compute.LinuxConfiguration{
			DisablePasswordAuthentication: ptr.To(true),
			SSH: &compute.SSHConfiguration{
				PublicKeys: &[]compute.SSHPublicKey{
					{
						Path:    ptr.To(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
						KeyData: ptr.To(string(sshKey)),
					},
				},
			},
		}
	}

	return osProfile, nil
}

func (s *ScaleSetSpec) generateImagePlan(ctx context.Context) *compute.Plan {
	_, log, done := tele.StartSpanWithLogger(ctx, "scalesets.ScaleSetSpec.generateImagePlan")
	defer done()

	if s.VMImage == nil {
		log.V(2).Info("no vm image found, disabling plan")
		return nil
	}

	if s.VMImage.SharedGallery != nil && s.VMImage.SharedGallery.Publisher != nil && s.VMImage.SharedGallery.SKU != nil && s.VMImage.SharedGallery.Offer != nil {
		return &compute.Plan{
			Publisher: s.VMImage.SharedGallery.Publisher,
			Name:      s.VMImage.SharedGallery.SKU,
			Product:   s.VMImage.SharedGallery.Offer,
		}
	}

	if s.VMImage.Marketplace == nil || !s.VMImage.Marketplace.ThirdPartyImage {
		return nil
	}

	if s.VMImage.Marketplace.Publisher == "" || s.VMImage.Marketplace.SKU == "" || s.VMImage.Marketplace.Offer == "" {
		return nil
	}

	return &compute.Plan{
		Publisher: ptr.To(s.VMImage.Marketplace.Publisher),
		Name:      ptr.To(s.VMImage.Marketplace.SKU),
		Product:   ptr.To(s.VMImage.Marketplace.Offer),
	}
}

func (s *ScaleSetSpec) getSecurityProfile() (*compute.SecurityProfile, error) {
	if s.SecurityProfile == nil {
		return nil, nil
	}

	if !s.SKU.HasCapability(resourceskus.EncryptionAtHost) {
		return nil, azure.WithTerminalError(errors.Errorf("encryption at host is not supported for VM type %s", s.Size))
	}

	return &compute.SecurityProfile{
		EncryptionAtHost: ptr.To(*s.SecurityProfile.EncryptionAtHost),
	}, nil
}
