//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

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

package e2e

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSUpgradeSpecInput struct {
	Cluster                    *clusterv1.Cluster
	MachinePools               []*expv1.MachinePool
	KubernetesVersionUpgradeTo string
	WaitForControlPlane        []interface{}
	WaitForMachinePools        []interface{}
}

func AKSUpgradeSpec(ctx context.Context, inputGetter func() AKSUpgradeSpecInput) {
	input := inputGetter()

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := azureutil.GetAuthorizer(settings)
	Expect(err).NotTo(HaveOccurred())

	managedClustersClient := containerservice.NewManagedClustersClient(subscriptionID)
	managedClustersClient.Authorizer = auth

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	By("Upgrading the control plane")
	var infraControlPlane = &infrav1.AzureManagedControlPlane{}
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
		g.Expect(err).NotTo(HaveOccurred())
		infraControlPlane.Spec.Version = input.KubernetesVersionUpgradeTo
		g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
	}, inputGetter().WaitForControlPlane...).Should(Succeed())

	Eventually(func(g Gomega) {
		aksCluster, err := managedClustersClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aksCluster.ManagedClusterProperties).NotTo(BeNil())
		g.Expect(aksCluster.ManagedClusterProperties.KubernetesVersion).NotTo(BeNil())
		g.Expect("v" + *aksCluster.KubernetesVersion).To(Equal(input.KubernetesVersionUpgradeTo))
	}, input.WaitForControlPlane...).Should(Succeed())

	By("Upgrading the machinepool instances")
	framework.UpgradeMachinePoolAndWait(ctx, framework.UpgradeMachinePoolAndWaitInput{
		ClusterProxy:                   bootstrapClusterProxy,
		Cluster:                        input.Cluster,
		UpgradeVersion:                 input.KubernetesVersionUpgradeTo,
		WaitForMachinePoolToBeUpgraded: input.WaitForMachinePools,
		MachinePools:                   input.MachinePools,
	})
}
