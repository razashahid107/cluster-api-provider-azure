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

package scope

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	aadpodid "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity"
	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/go-autorest/autorest"
	"github.com/jongio/azidext/go/azidext"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/identity"
	"sigs.k8s.io/cluster-api-provider-azure/util/system"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctl "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzureSecretKey is the value for they client secret key.
const AzureSecretKey = "clientSecret"

// CredentialsProvider defines the behavior for azure identity based credential providers.
type CredentialsProvider interface {
	GetAuthorizer(ctx context.Context, tokenCredential azcore.TokenCredential, tokenAudience string) (autorest.Authorizer, error)
	GetClientID() string
	GetClientSecret(ctx context.Context) (string, error)
	GetTenantID() string
	GetTokenCredential(ctx context.Context, resourceManagerEndpoint, activeDirectoryEndpoint, tokenAudience string) (azcore.TokenCredential, error)
}

// AzureCredentialsProvider represents a credential provider with azure cluster identity.
type AzureCredentialsProvider struct {
	Client   client.Client
	Identity *infrav1.AzureClusterIdentity
}

// AzureClusterCredentialsProvider wraps AzureCredentialsProvider with AzureCluster.
type AzureClusterCredentialsProvider struct {
	AzureCredentialsProvider
	AzureCluster *infrav1.AzureCluster
}

// ManagedControlPlaneCredentialsProvider wraps AzureCredentialsProvider with AzureManagedControlPlane.
type ManagedControlPlaneCredentialsProvider struct {
	AzureCredentialsProvider
	AzureManagedControlPlane *infrav1.AzureManagedControlPlane
}

var _ CredentialsProvider = (*AzureClusterCredentialsProvider)(nil)
var _ CredentialsProvider = (*ManagedControlPlaneCredentialsProvider)(nil)

// NewAzureClusterCredentialsProvider creates a new AzureClusterCredentialsProvider from the supplied inputs.
func NewAzureClusterCredentialsProvider(ctx context.Context, kubeClient client.Client, azureCluster *infrav1.AzureCluster) (*AzureClusterCredentialsProvider, error) {
	if azureCluster.Spec.IdentityRef == nil {
		return nil, errors.New("failed to generate new AzureClusterCredentialsProvider from empty identityName")
	}

	ref := azureCluster.Spec.IdentityRef
	// if the namespace isn't specified then assume it's in the same namespace as the AzureCluster
	namespace := ref.Namespace
	if namespace == "" {
		namespace = azureCluster.Namespace
	}
	identity := &infrav1.AzureClusterIdentity{}
	key := client.ObjectKey{Name: ref.Name, Namespace: namespace}
	if err := kubeClient.Get(ctx, key, identity); err != nil {
		return nil, errors.Errorf("failed to retrieve AzureClusterIdentity external object %q/%q: %v", key.Namespace, key.Name, err)
	}

	return &AzureClusterCredentialsProvider{
		AzureCredentialsProvider{
			Client:   kubeClient,
			Identity: identity,
		},
		azureCluster,
	}, nil
}

// GetAuthorizer returns an Azure authorizer based on the provided azure identity. It delegates to AzureCredentialsProvider with AzureCluster metadata.
func (p *AzureClusterCredentialsProvider) GetAuthorizer(ctx context.Context, tokenCredential azcore.TokenCredential, tokenAudience string) (autorest.Authorizer, error) {
	return p.AzureCredentialsProvider.GetAuthorizer(ctx, tokenCredential, tokenAudience)
}

// GetTokenCredential returns an Azure TokenCredential based on the provided azure identity.
func (p *AzureClusterCredentialsProvider) GetTokenCredential(ctx context.Context, resourceManagerEndpoint, activeDirectoryEndpoint, tokenAudience string) (azcore.TokenCredential, error) {
	return p.AzureCredentialsProvider.GetTokenCredential(ctx, resourceManagerEndpoint, activeDirectoryEndpoint, tokenAudience, p.AzureCluster.ObjectMeta)
}

// NewManagedControlPlaneCredentialsProvider creates a new ManagedControlPlaneCredentialsProvider from the supplied inputs.
func NewManagedControlPlaneCredentialsProvider(ctx context.Context, kubeClient client.Client, managedControlPlane *infrav1.AzureManagedControlPlane) (*ManagedControlPlaneCredentialsProvider, error) {
	if managedControlPlane.Spec.IdentityRef == nil {
		return nil, errors.New("failed to generate new ManagedControlPlaneCredentialsProvider from empty identityName")
	}

	ref := managedControlPlane.Spec.IdentityRef
	// if the namespace isn't specified then assume it's in the same namespace as the AzureManagedControlPlane
	namespace := ref.Namespace
	if namespace == "" {
		namespace = managedControlPlane.Namespace
	}
	identity := &infrav1.AzureClusterIdentity{}
	key := client.ObjectKey{Name: ref.Name, Namespace: namespace}
	if err := kubeClient.Get(ctx, key, identity); err != nil {
		return nil, errors.Errorf("failed to retrieve AzureClusterIdentity external object %q/%q: %v", key.Namespace, key.Name, err)
	}

	return &ManagedControlPlaneCredentialsProvider{
		AzureCredentialsProvider{
			Client:   kubeClient,
			Identity: identity,
		},
		managedControlPlane,
	}, nil
}

// GetAuthorizer returns an Azure authorizer based on the provided azure identity. It delegates to AzureCredentialsProvider with AzureManagedControlPlane metadata.
func (p *ManagedControlPlaneCredentialsProvider) GetAuthorizer(ctx context.Context, tokenCredential azcore.TokenCredential, tokenAudience string) (autorest.Authorizer, error) {
	return p.AzureCredentialsProvider.GetAuthorizer(ctx, tokenCredential, tokenAudience)
}

// GetTokenCredential returns an Azure TokenCredential based on the provided azure identity.
func (p *ManagedControlPlaneCredentialsProvider) GetTokenCredential(ctx context.Context, resourceManagerEndpoint, activeDirectoryEndpoint, tokenAudience string) (azcore.TokenCredential, error) {
	return p.AzureCredentialsProvider.GetTokenCredential(ctx, resourceManagerEndpoint, activeDirectoryEndpoint, tokenAudience, p.AzureManagedControlPlane.ObjectMeta)
}

// GetTokenCredential returns an Azure TokenCredential based on the provided azure identity.
func (p *AzureCredentialsProvider) GetTokenCredential(ctx context.Context, resourceManagerEndpoint, activeDirectoryEndpoint, tokenAudience string, clusterMeta metav1.ObjectMeta) (azcore.TokenCredential, error) {
	var authErr error
	var cred azcore.TokenCredential

	switch p.Identity.Spec.Type {
	case infrav1.WorkloadIdentity:
		azwiCredOptions, err := NewWorkloadIdentityCredentialOptions().
			WithTenantID(p.Identity.Spec.TenantID).
			WithClientID(p.Identity.Spec.ClientID).
			WithDefaults()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to setup azwi options for identity %s", p.Identity.Name)
		}
		cred, authErr = NewWorkloadIdentityCredential(azwiCredOptions)

	case infrav1.ServicePrincipal, infrav1.ServicePrincipalCertificate, infrav1.UserAssignedMSI:
		if err := createAzureIdentityWithBindings(ctx, p.Identity, resourceManagerEndpoint, activeDirectoryEndpoint, clusterMeta, p.Client); err != nil {
			return nil, err
		}

		options := azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(p.Identity.Spec.ClientID),
		}
		cred, authErr = azidentity.NewManagedIdentityCredential(&options)

	case infrav1.ManualServicePrincipal:
		clientSecret, err := p.GetClientSecret(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get client secret")
		}

		options := azidentity.ClientSecretCredentialOptions{
			ClientOptions: azcore.ClientOptions{
				Cloud: cloud.Configuration{
					ActiveDirectoryAuthorityHost: activeDirectoryEndpoint,
					Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
						cloud.ResourceManager: {
							Audience: tokenAudience,
							Endpoint: resourceManagerEndpoint,
						},
					},
				},
			},
		}
		cred, authErr = azidentity.NewClientSecretCredential(p.GetTenantID(), p.Identity.Spec.ClientID, clientSecret, &options)

	default:
		return nil, errors.Errorf("identity type %s not supported", p.Identity.Spec.Type)
	}

	if authErr != nil {
		return nil, errors.Errorf("failed to get token from service principal identity: %v", authErr)
	}

	return cred, nil
}

// GetAuthorizer returns an Azure authorizer based on the provided azure identity, cluster metadata, and tokenCredential.
func (p *AzureCredentialsProvider) GetAuthorizer(ctx context.Context, cred azcore.TokenCredential, tokenAudience string) (autorest.Authorizer, error) {
	// We must use TokenAudience for StackCloud, otherwise we get an
	// AADSTS500011 error from the API
	scope := tokenAudience
	if !strings.HasSuffix(scope, "/.default") {
		scope += "/.default"
	}
	authorizer := azidext.NewTokenCredentialAdapter(cred, []string{scope})
	return authorizer, nil
}

// GetClientID returns the Client ID associated with the AzureCredentialsProvider's Identity.
func (p *AzureCredentialsProvider) GetClientID() string {
	return p.Identity.Spec.ClientID
}

// GetClientSecret returns the Client Secret associated with the AzureCredentialsProvider's Identity.
// NOTE: this only works if the Identity references a Service Principal Client Secret.
// If using another type of credentials, such a Certificate, we return an empty string.
func (p *AzureCredentialsProvider) GetClientSecret(ctx context.Context) (string, error) {
	if p.hasClientSecret() {
		secretRef := p.Identity.Spec.ClientSecret
		key := types.NamespacedName{
			Namespace: secretRef.Namespace,
			Name:      secretRef.Name,
		}
		secret := &corev1.Secret{}

		if err := p.Client.Get(ctx, key, secret); err != nil {
			return "", errors.Wrap(err, "Unable to fetch ClientSecret")
		}
		return string(secret.Data[AzureSecretKey]), nil
	}
	return "", nil
}

// GetTenantID returns the Tenant ID associated with the AzureCredentialsProvider's Identity.
func (p *AzureCredentialsProvider) GetTenantID() string {
	return p.Identity.Spec.TenantID
}

// hasClientSecret returns true if the identity has a Service Principal Client Secret.
// This does not include service principals with certificates or managed identities.
func (p *AzureCredentialsProvider) hasClientSecret() bool {
	return p.Identity.Spec.Type == infrav1.ServicePrincipal || p.Identity.Spec.Type == infrav1.ManualServicePrincipal
}

func createAzureIdentityWithBindings(ctx context.Context, azureIdentity *infrav1.AzureClusterIdentity, resourceManagerEndpoint, activeDirectoryEndpoint string, clusterMeta metav1.ObjectMeta,
	kubeClient client.Client) error {
	azureIdentityType, err := getAzureIdentityType(azureIdentity)
	if err != nil {
		return err
	}

	// AzureIdentity and AzureIdentityBinding will no longer have an OwnerRef starting from capz release v0.5.0 because of the following:
	// In Kubernetes v1.20+, if the garbage collector detects an invalid cross-namespace ownerReference, or a cluster-scoped dependent with
	// an ownerReference referencing a namespaced kind, a warning Event with a reason of OwnerRefInvalidNamespace and an involvedObject
	// of the invalid dependent is reported. You can check for that kind of Event by running kubectl get events -A --field-selector=reason=OwnerRefInvalidNamespace.

	copiedIdentity := &aadpodv1.AzureIdentity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AzureIdentity",
			APIVersion: "aadpodidentity.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      identity.GetAzureIdentityName(clusterMeta.Name, clusterMeta.Namespace, azureIdentity.Name),
			Namespace: system.GetManagerNamespace(),
			Annotations: map[string]string{
				aadpodv1.BehaviorKey: "namespaced",
			},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:              clusterMeta.Name,
				infrav1.ClusterLabelNamespace:           clusterMeta.Namespace,
				clusterctl.ClusterctlMoveHierarchyLabel: "true",
			},
		},
		Spec: aadpodv1.AzureIdentitySpec{
			Type:           azureIdentityType,
			TenantID:       azureIdentity.Spec.TenantID,
			ClientID:       azureIdentity.Spec.ClientID,
			ClientPassword: azureIdentity.Spec.ClientSecret,
			ResourceID:     azureIdentity.Spec.ResourceID,
			ADResourceID:   resourceManagerEndpoint,
			ADEndpoint:     activeDirectoryEndpoint,
		},
	}
	err = kubeClient.Create(ctx, copiedIdentity)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Errorf("failed to create copied AzureIdentity %s in %s: %v", copiedIdentity.Name, system.GetManagerNamespace(), err)
	}

	azureIdentityBinding := &aadpodv1.AzureIdentityBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AzureIdentityBinding",
			APIVersion: "aadpodidentity.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-binding", copiedIdentity.Name),
			Namespace: copiedIdentity.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:              clusterMeta.Name,
				infrav1.ClusterLabelNamespace:           clusterMeta.Namespace,
				clusterctl.ClusterctlMoveHierarchyLabel: "true",
			},
		},
		Spec: aadpodv1.AzureIdentityBindingSpec{
			AzureIdentity: copiedIdentity.Name,
			Selector:      infrav1.AzureIdentityBindingSelector, // should be same as selector added on controller
		},
	}
	err = kubeClient.Create(ctx, azureIdentityBinding)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Errorf("failed to create AzureIdentityBinding %s in %s: %v", copiedIdentity.Name, system.GetManagerNamespace(), err)
	}

	return nil
}

func getAzureIdentityType(identity *infrav1.AzureClusterIdentity) (aadpodv1.IdentityType, error) {
	switch identity.Spec.Type {
	case infrav1.UserAssignedMSI:
		return aadpodv1.UserAssignedMSI, nil
	case infrav1.ServicePrincipal:
		return aadpodv1.ServicePrincipal, nil
	case infrav1.ServicePrincipalCertificate:
		return aadpodv1.IdentityType(aadpodid.ServicePrincipalCertificate), nil
	}

	return -1, errors.New("AzureIdentity does not have a valid type")
}

// IsClusterNamespaceAllowed indicates if the cluster namespace is allowed.
func IsClusterNamespaceAllowed(ctx context.Context, k8sClient client.Client, allowedNamespaces *infrav1.AllowedNamespaces, namespace string) bool {
	if allowedNamespaces == nil {
		return false
	}

	// empty value matches with all namespaces
	if reflect.DeepEqual(*allowedNamespaces, infrav1.AllowedNamespaces{}) {
		return true
	}

	for _, v := range allowedNamespaces.NamespaceList {
		if v == namespace {
			return true
		}
	}

	// Check if clusterNamespace is in the namespaces selected by the identity's allowedNamespaces selector.
	namespaces := &corev1.NamespaceList{}
	selector, err := metav1.LabelSelectorAsSelector(allowedNamespaces.Selector)
	if err != nil {
		return false
	}

	// If a Selector has a nil or empty selector, it should match nothing.
	if selector.Empty() {
		return false
	}

	if err := k8sClient.List(ctx, namespaces, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return false
	}

	for _, n := range namespaces.Items {
		if n.Name == namespace {
			return true
		}
	}

	return false
}
