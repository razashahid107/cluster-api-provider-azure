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

package securitygroups

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

var (
	sshRule = infrav1.SecurityRule{
		Name:             "allow_ssh",
		Description:      "Allow SSH",
		Priority:         2200,
		Protocol:         infrav1.SecurityGroupProtocolTCP,
		Direction:        infrav1.SecurityRuleDirectionInbound,
		Source:           ptr.To("*"),
		SourcePorts:      ptr.To("*"),
		Destination:      ptr.To("*"),
		DestinationPorts: ptr.To("22"),
	}
	otherRule = infrav1.SecurityRule{
		Name:             "other_rule",
		Description:      "Test Rule",
		Priority:         500,
		Protocol:         infrav1.SecurityGroupProtocolTCP,
		Direction:        infrav1.SecurityRuleDirectionInbound,
		Source:           ptr.To("*"),
		SourcePorts:      ptr.To("*"),
		Destination:      ptr.To("*"),
		DestinationPorts: ptr.To("80"),
	}
	customRule = infrav1.SecurityRule{
		Name:             "custom_rule",
		Description:      "Test Rule",
		Priority:         501,
		Protocol:         infrav1.SecurityGroupProtocolTCP,
		Direction:        infrav1.SecurityRuleDirectionOutbound,
		Source:           ptr.To("*"),
		SourcePorts:      ptr.To("*"),
		Destination:      ptr.To("*"),
		DestinationPorts: ptr.To("80"),
	}
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *NSGSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name: "NSG already exists with all rules present",
			spec: &NSGSpec{
				Name:     "test-nsg",
				Location: "test-location",
				SecurityRules: infrav1.SecurityRules{
					sshRule,
					otherRule,
				},
				ResourceGroup: "test-group",
				ClusterName:   "my-cluster",
			},
			existing: network.SecurityGroup{
				Name: ptr.To("test-nsg"),
				SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
					SecurityRules: &[]network.SecurityRule{
						converters.SecurityRuleToSDK(sshRule),
						converters.SecurityRuleToSDK(otherRule),
					},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name: "NSG already exists but missing a rule",
			spec: &NSGSpec{
				Name:     "test-nsg",
				Location: "test-location",
				SecurityRules: infrav1.SecurityRules{
					sshRule,
					otherRule,
				},
				ResourceGroup: "test-group",
				ClusterName:   "my-cluster",
			},
			existing: network.SecurityGroup{
				Name:     ptr.To("test-nsg"),
				Location: ptr.To("test-location"),
				Etag:     ptr.To("fake-etag"),
				SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
					SecurityRules: &[]network.SecurityRule{
						converters.SecurityRuleToSDK(sshRule),
						converters.SecurityRuleToSDK(customRule),
					},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.SecurityGroup{}))
				g.Expect(result).To(Equal(network.SecurityGroup{
					Location: ptr.To("test-location"),
					Etag:     ptr.To("fake-etag"),
					SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
						SecurityRules: &[]network.SecurityRule{
							converters.SecurityRuleToSDK(otherRule),
							converters.SecurityRuleToSDK(sshRule),
							converters.SecurityRuleToSDK(customRule),
						},
					},
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
						"Name": ptr.To("test-nsg"),
					},
				}))
			},
		},
		{
			name: "NSG already exists and a rule is deleted",
			spec: &NSGSpec{
				Name:     "test-nsg",
				Location: "test-location",
				SecurityRules: infrav1.SecurityRules{
					sshRule,
					customRule,
				},
				ResourceGroup: "test-group",
				ClusterName:   "my-cluster",
				LastAppliedSecurityRules: map[string]interface{}{
					"allow_ssh":   sshRule,
					"custom_rule": customRule,
					"other_rule":  otherRule,
				},
			},
			existing: network.SecurityGroup{
				Name:     ptr.To("test-nsg"),
				Location: ptr.To("test-location"),
				Etag:     ptr.To("fake-etag"),
				SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
					SecurityRules: &[]network.SecurityRule{
						converters.SecurityRuleToSDK(sshRule),
						converters.SecurityRuleToSDK(customRule),
						converters.SecurityRuleToSDK(otherRule),
					},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.SecurityGroup{}))
				g.Expect(result).To(Equal(network.SecurityGroup{
					Location: ptr.To("test-location"),
					Etag:     ptr.To("fake-etag"),
					SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
						SecurityRules: &[]network.SecurityRule{
							converters.SecurityRuleToSDK(sshRule),
							converters.SecurityRuleToSDK(customRule),
						},
					},
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
						"Name": ptr.To("test-nsg"),
					},
				}))
			},
		},
		{
			name: "NSG already exists and a rule not owned by CAPZ is present",
			spec: &NSGSpec{
				Name:     "test-nsg",
				Location: "test-location",
				SecurityRules: infrav1.SecurityRules{
					sshRule,
					customRule,
				},
				ResourceGroup: "test-group",
				ClusterName:   "my-cluster",
				LastAppliedSecurityRules: map[string]interface{}{
					"allow_ssh":   sshRule,
					"custom_rule": customRule,
				},
			},
			existing: network.SecurityGroup{
				Name:     ptr.To("test-nsg"),
				Location: ptr.To("test-location"),
				Etag:     ptr.To("fake-etag"),
				SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
					SecurityRules: &[]network.SecurityRule{
						converters.SecurityRuleToSDK(sshRule),
						converters.SecurityRuleToSDK(customRule),
						converters.SecurityRuleToSDK(otherRule),
					},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name: "NSG does not exist",
			spec: &NSGSpec{
				Name:     "test-nsg",
				Location: "test-location",
				SecurityRules: infrav1.SecurityRules{
					sshRule,
					otherRule,
				},
				ResourceGroup: "test-group",
				ClusterName:   "my-cluster",
			},
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.SecurityGroup{}))
				g.Expect(result).To(Equal(network.SecurityGroup{
					SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
						SecurityRules: &[]network.SecurityRule{
							converters.SecurityRuleToSDK(sshRule),
							converters.SecurityRuleToSDK(otherRule),
						},
					},
					Location: ptr.To("test-location"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
						"Name": ptr.To("test-nsg"),
					},
				}))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}

func TestRuleExists(t *testing.T) {
	testcases := []struct {
		name     string
		rules    []network.SecurityRule
		rule     network.SecurityRule
		expected bool
	}{
		{
			name:     "rule doesn't exitst",
			rules:    []network.SecurityRule{ruleA},
			rule:     ruleB,
			expected: false,
		},
		{
			name:     "rule exists",
			rules:    []network.SecurityRule{ruleA, ruleB},
			rule:     ruleB,
			expected: true,
		},
		{
			name:     "rule exists but has been modified",
			rules:    []network.SecurityRule{ruleA, ruleB},
			rule:     ruleBModified,
			expected: false,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			result := ruleExists(tc.rules, tc.rule)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}
