/*
Copyright 2020 The Kubernetes Authors.

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

package converters

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

// ErrUserAssignedIdentitiesNotFound is the error thrown when user assigned identities is not passed with the identity type being UserAssigned.
var ErrUserAssignedIdentitiesNotFound = errors.New("the user-assigned identity provider ids must not be null or empty for 'UserAssigned' identity type")

// VMIdentityToVMSDK converts CAPZ VM identity to Azure SDK identity.
func VMIdentityToVMSDK(identity infrav1.VMIdentity, uami []infrav1.UserAssignedIdentity) (*compute.VirtualMachineIdentity, error) {
	if identity == infrav1.VMIdentitySystemAssigned {
		return &compute.VirtualMachineIdentity{
			Type: compute.ResourceIdentityTypeSystemAssigned,
		}, nil
	}

	if identity == infrav1.VMIdentityUserAssigned {
		userIdentitiesMap, err := UserAssignedIdentitiesToVMSDK(uami)
		if err != nil {
			return nil, errors.Wrap(err, "failed to assign VM identity")
		}

		return &compute.VirtualMachineIdentity{
			Type:                   compute.ResourceIdentityTypeUserAssigned,
			UserAssignedIdentities: userIdentitiesMap,
		}, nil
	}

	return nil, nil
}

// UserAssignedIdentitiesToVMSDK converts CAPZ user assigned identities associated with the Virtual Machine to Azure SDK identities
// The user identity dictionary key references will be ARM resource ids in the form:
// '/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{identityName}'.
func UserAssignedIdentitiesToVMSDK(identities []infrav1.UserAssignedIdentity) (map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, error) {
	if len(identities) == 0 {
		return nil, ErrUserAssignedIdentitiesNotFound
	}
	userIdentitiesMap := make(map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, len(identities))
	for _, id := range identities {
		key := sanitized(id.ProviderID)
		userIdentitiesMap[key] = &compute.VirtualMachineIdentityUserAssignedIdentitiesValue{}
	}

	return userIdentitiesMap, nil
}

// UserAssignedIdentitiesToVMSSSDK converts CAPZ user assigned identities associated with the Virtual Machine Scale Set to Azure SDK identities
// Similar to UserAssignedIdentitiesToVMSDK.
func UserAssignedIdentitiesToVMSSSDK(identities []infrav1.UserAssignedIdentity) (map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue, error) {
	if len(identities) == 0 {
		return nil, ErrUserAssignedIdentitiesNotFound
	}
	userIdentitiesMap := make(map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue, len(identities))
	for _, id := range identities {
		key := sanitized(id.ProviderID)
		userIdentitiesMap[key] = &compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue{}
	}

	return userIdentitiesMap, nil
}

// sanitized removes "azure://" prefix from the given id.
func sanitized(id string) string {
	return strings.TrimPrefix(id, azureutil.ProviderIDPrefix)
}
