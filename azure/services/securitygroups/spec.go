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
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// NSGSpec defines the specification for a security group.
type NSGSpec struct {
	Name                     string
	SecurityRules            infrav1.SecurityRules
	Location                 string
	ClusterName              string
	ResourceGroup            string
	AdditionalTags           infrav1.Tags
	LastAppliedSecurityRules map[string]interface{}
}

// ResourceName returns the name of the security group.
func (s *NSGSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *NSGSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for security groups.
func (s *NSGSpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the security group.
func (s *NSGSpec) Parameters(ctx context.Context, existing interface{}) (interface{}, error) {
	securityRules := make([]network.SecurityRule, 0)
	newAnnotation := map[string]string{}
	var etag *string

	if existing != nil {
		existingNSG, ok := existing.(network.SecurityGroup)
		if !ok {
			return nil, errors.Errorf("%T is not a network.SecurityGroup", existing)
		}
		// security group already exists
		// We append the existing NSG etag to the header to ensure we only apply the updates if the NSG has not been modified.
		etag = existingNSG.Etag
		// Check if the expected rules are present
		update := false

		for _, rule := range s.SecurityRules {
			sdkRule := converters.SecurityRuleToSDK(rule)
			if !ruleExists(*existingNSG.SecurityRules, sdkRule) {
				update = true
				securityRules = append(securityRules, sdkRule)
			}
			newAnnotation[rule.Name] = rule.Description
		}

		for _, oldRule := range *existingNSG.SecurityRules {
			_, tracked := s.LastAppliedSecurityRules[*oldRule.Name]
			// If rule is owned by CAPZ and applied last, and not found in the new rules, then it has been deleted
			if _, ok := newAnnotation[*oldRule.Name]; !ok && tracked {
				// Rule has been deleted
				update = true
				continue
			}

			// Add previous rules that haven't been deleted
			securityRules = append(securityRules, oldRule)
		}

		if !update {
			// Skip update for NSG as the required default rules are present
			return nil, nil
		}
	} else {
		// new security group
		for _, rule := range s.SecurityRules {
			securityRules = append(securityRules, converters.SecurityRuleToSDK(rule))
		}
	}

	return network.SecurityGroup{
		Location: ptr.To(s.Location),
		SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
			SecurityRules: &securityRules,
		},
		Etag: etag,
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        ptr.To(s.Name),
			Additional:  s.AdditionalTags,
		})),
	}, nil
}

// TODO: review this logic and make sure it is what we want. It seems incorrect to skip rules that don't have a certain protocol, etc.
func ruleExists(rules []network.SecurityRule, rule network.SecurityRule) bool {
	for _, existingRule := range rules {
		if !strings.EqualFold(ptr.Deref(existingRule.Name, ""), ptr.Deref(rule.Name, "")) {
			continue
		}
		if !strings.EqualFold(ptr.Deref(existingRule.DestinationPortRange, ""), ptr.Deref(rule.DestinationPortRange, "")) {
			continue
		}
		if existingRule.Protocol != network.SecurityRuleProtocolTCP &&
			existingRule.Access != network.SecurityRuleAccessAllow &&
			existingRule.Direction != network.SecurityRuleDirectionInbound {
			continue
		}
		if !strings.EqualFold(ptr.Deref(existingRule.SourcePortRange, ""), "*") &&
			!strings.EqualFold(ptr.Deref(existingRule.SourceAddressPrefix, ""), "*") &&
			!strings.EqualFold(ptr.Deref(existingRule.DestinationAddressPrefix, ""), "*") {
			continue
		}
		return true
	}
	return false
}
