//
// Copyright 2020 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package policy

import (
	"strconv"
	"strings"

	"github.com/IBM/integrity-enforcer/enforcer/pkg/control/common"
	"github.com/IBM/integrity-enforcer/enforcer/pkg/kubeutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1cli "k8s.io/client-go/kubernetes/typed/core/v1"
)

/**********************************************

				PolicyChecker

***********************************************/

type PolicyChecker interface {
	IsTrustStateEnforcementDisabled() bool
	IsEnforceResult() bool
	IsIgnoreRequest() bool
	IsAllowedForInternalRequest() bool
	IsAllowedByRule() bool
	PermitIfVerifiedOwner() bool
	PermitIfCreator() bool
}

func NewPolicyChecker(policy *Policy, reqc *common.ReqContext) PolicyChecker {
	return &concretePolicyChecker{
		policy: policy,
		reqc:   reqc,
	}
}

type concretePolicyChecker struct {
	policy *Policy
	reqc   *common.ReqContext
}

func (self *concretePolicyChecker) check(patterns []RequestMatchPattern) bool {

	reqc := self.reqc

	isInScope := false
	for _, v := range patterns {
		if v.Match(reqc) {
			isInScope = true
			break
		}
	}
	return isInScope
}

func (self *concretePolicyChecker) IsTrustStateEnforcementDisabled() bool {

	if self.policy != nil && self.policy.AllowUnverified != nil {
		for _, pattern := range self.policy.AllowUnverified {
			if pattern.Namespace == self.reqc.Namespace {
				return true
			}
		}
		return false
	} else {
		return false
	}

}

func (self *concretePolicyChecker) IsIgnoreRequest() bool {
	if self.policy != nil && self.policy.IgnoreRequest != nil {
		return self.check(self.policy.IgnoreRequest)
	} else {
		return false
	}
}

func (self *concretePolicyChecker) IsEnforceResult() bool {
	if self.IsIgnoreRequest() {
		return false
	} else if self.policy != nil && self.policy.Enforce != nil {
		return self.check(self.policy.Enforce)
	} else {
		return false
	}
}

func (self *concretePolicyChecker) IsAllowedForInternalRequest() bool {
	if self.policy != nil && self.policy.AllowedForInternalRequest != nil {
		return self.check(self.policy.AllowedForInternalRequest)
	} else {
		return false
	}
}

func (self *concretePolicyChecker) IsAllowedByRule() bool {
	if self.policy != nil && self.policy.AllowedByRule != nil {
		return self.check(self.policy.AllowedByRule)
	} else {
		return false
	}
}

func (self *concretePolicyChecker) PermitIfCreator() bool {
	if self.reqc.IsCreator() {
		if self.policy != nil && self.policy.PermitIfCreator != nil {
			return self.isAuthorizedServiceAccount(self.policy.PermitIfCreator)
		} else {
			return false
		}
	} else {
		return false
	}
}

func (self *concretePolicyChecker) PermitIfVerifiedOwner() bool {
	if self.policy != nil && self.policy.PermitIfVerifiedOwner != nil {
		return self.isAuthorizedServiceAccount(self.policy.PermitIfVerifiedOwner)
	} else {
		return false
	}
}

func (self *concretePolicyChecker) isAuthorizedServiceAccount(patterns []AllowedUserPattern) bool {
	if patterns == nil {
		return false
	}
	for _, p := range patterns {
		request := p.Request.Match(self.reqc)
		if !request {
			continue
		}
		if len(p.AuthorizedServiceAccount) != 0 {
			for _, au := range p.AuthorizedServiceAccount {
				userName := self.reqc.UserName
				if strings.Contains(userName, ":") {
					name := strings.Split(userName, ":")
					userName = name[len(name)-1]
				}
				result := MatchPattern(au, userName)
				if result {
					return result
				}
			}
		}
		if p.AllowChangesBySignedServiceAccount {
			var sa *v1.ServiceAccount
			if self.reqc.ServiceAccount == nil {
				if !strings.HasPrefix(self.reqc.UserName, "system:") || !strings.Contains(self.reqc.UserName, ":") {
					continue
				}
				name := strings.Split(self.reqc.UserName, ":")
				saName := name[len(name)-1]
				namespace := name[len(name)-2]
				serviceAccount, err := GetServiceAccount(saName, namespace)
				if err != nil {
					continue
				}
				sa = serviceAccount
				self.reqc.ServiceAccount = serviceAccount
			} else {
				sa = self.reqc.ServiceAccount
			}
			if s, ok := sa.Annotations["integrityVerified"]; ok {
				if b, err := strconv.ParseBool(s); err != nil {
					continue
				} else {
					if b {
						return b
					}
				}
			}
			if s, ok := sa.Annotations["integrityUnverified"]; ok {
				if b, err := strconv.ParseBool(s); err != nil {
					continue
				} else {
					if b {
						return b
					}
				}
			}
		}
	}
	return false
}

/**********************************************

				Common Functions

***********************************************/

func MatchPattern(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return true
	} else if pattern == "*" {
		return true
	} else if pattern == "-" && value == "" {
		return true
	} else if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimRight(pattern, "*"))
	} else if pattern == value {
		return true
	} else if strings.Contains(pattern, ",") {
		patterns := SplitRule(pattern)
		return MatchWithPatternArray(value, patterns)
	} else {
		return false
	}
}

func MatchPatternWithArray(pattern string, valueArray []string) bool {
	for _, value := range valueArray {
		if MatchPattern(pattern, value) {
			return true
		}
	}
	return false
}

func MatchWithPatternArray(value string, patternArray []string) bool {
	for _, pattern := range patternArray {
		if MatchPattern(pattern, value) {
			return true
		}
	}
	return false
}

func SplitRule(rules string) []string {
	result := []string{}
	slice := strings.Split(rules, ",")
	for _, s := range slice {
		rule := strings.TrimSpace(s)
		result = append(result, rule)
	}
	return result
}

func GetServiceAccount(name, namespace string) (*v1.ServiceAccount, error) {
	config, err := kubeutil.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	v1client := v1cli.NewForConfigOrDie(config)

	serviceAccount, err := v1client.ServiceAccounts(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return serviceAccount, nil
}
