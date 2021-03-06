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
	"fmt"

	"github.com/IBM/integrity-enforcer/enforcer/pkg/control/common"
	"github.com/jinzhu/copier"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PolicyType string

const (
	UnknownPolicy PolicyType = ""
	DefaultPolicy PolicyType = "DefaultPolicy"
	IEPolicy      PolicyType = "IEPolicy"
	SignerPolicy  PolicyType = "SignerPolicy"
	CustomPolicy  PolicyType = "CustomPolicy"
)

/**********************************************

					Policy

***********************************************/

type Policy struct {
	Enforce                   []RequestMatchPattern      `json:"enforce,omitempty"`
	AllowUnverified           []AllowUnverifiedCondition `json:"allowUnverified,omitempty"`
	IgnoreRequest             []RequestMatchPattern      `json:"ignoreRequest,omitempty"`
	AllowedSigner             []SignerMatchPattern       `json:"allowedSigner,omitempty"`
	AllowedForInternalRequest []RequestMatchPattern      `json:"allowedForInternalRequest,omitempty"`
	AllowedByRule             []RequestMatchPattern      `json:"allowedByRule,omitempty"`
	AllowedChange             []AllowedChangeCondition   `json:"allowedChange,omitempty"`
	PermitIfVerifiedOwner     []AllowedUserPattern       `json:"permitIfVerifiedOwner,omitempty"`
	PermitIfCreator           []AllowedUserPattern       `json:"permitIfCreator,omitempty"`
	Namespace                 string                     `json:"namespace,omitempty"`
	PolicyType                PolicyType                 `json:"policyType,omitempty"`
}

func (self *Policy) CheckFormat() (bool, string) {
	pType := self.PolicyType
	ns := self.Namespace

	if pType == UnknownPolicy {
		return false, "\"policyType\" must be set for any Policy"
	}

	if ns != "" && (pType == DefaultPolicy || pType == IEPolicy || pType == SignerPolicy) {
		return false, fmt.Sprintf("\"namespace\" must be empty for %s", pType)
	}
	if ns == "" && pType == CustomPolicy {
		return false, fmt.Sprintf("\"namespace\" must be specified for %s", pType)
	}
	if pType == SignerPolicy {
		hasEnforce := len(self.Enforce) > 0
		hasIgnore := len(self.IgnoreRequest) > 0
		hasInternal := len(self.AllowedForInternalRequest) > 0
		hasAllowRule := len(self.AllowedByRule) > 0
		hasAllowChange := len(self.AllowedChange) > 0
		hasVOwner := len(self.PermitIfVerifiedOwner) > 0
		hasFUser := len(self.PermitIfCreator) > 0
		if hasEnforce || hasIgnore || hasInternal || hasAllowRule || hasAllowChange || hasVOwner || hasFUser {
			return false, fmt.Sprintf("%s must contain only AllowedSigner rule", pType)
		}
	}
	if pType == CustomPolicy {
		hasSigner := len(self.AllowedSigner) > 0
		if hasSigner {
			return false, fmt.Sprintf("%s must not contain AllowedSigner rule", pType)
		}
	}
	return true, ""
}

type AllowedChangeCondition struct {
	Request RequestMatchPattern `json:"request,omitempty"`
	Key     []string            `json:"key,omitempty"`
	Owner   OwnerMatchCondition `json:"owner,omitempty"`
}

type OwnerMatchCondition struct {
	Kind       string `json:"kind,omitempty"`
	ApiVersion string `json:"apiVersion,omitempty"`
	Name       string `json:"name,omitempty"`
}

type SubjectMatchPattern struct {
	Email string `json:"email,omitempty"`
	Uid   string `json:"uid,omitempty"`
}

type AllowUnverifiedCondition struct {
	Namespace string `json:"namespace,omitempty"`
}

type SignerMatchPattern struct {
	Request RequestMatchPattern `json:"request,omitempty"`
	Subject SubjectMatchPattern `json:"subject,omitempty"`
}

type AllowedUserPattern struct {
	AllowChangesBySignedServiceAccount bool                `json:"allowChangesBySignedServiceAccount,omitempty"`
	AuthorizedServiceAccount           []string            `json:"authorizedServiceAccount,omitempty"`
	Request                            RequestMatchPattern `json:"request,omitempty"`
}

type RequestMatchPattern struct {
	Namespace    string `json:"namespace,omitempty"`
	Name         string `json:"name,omitempty"`
	Operation    string `json:"operation,omitempty"`
	ApiVersion   string `json:"apiVersion,omitempty"`
	Kind         string `json:"kind,omitempty"`
	UserName     string `json:"username,omitempty"`
	Type         string `json:"type,omitempty"`
	K8sCreatedBy string `json:"k8screatedby,omitempty"`
	UserGroup    string `json:"usergroup,omitempty"`
}

func (v *RequestMatchPattern) Match(reqc *common.ReqContext) bool {
	gv := schema.GroupVersion{
		Group:   reqc.ApiGroup,
		Version: reqc.ApiVersion,
	}
	apiVersion := gv.String()

	return MatchPattern(v.Namespace, reqc.Namespace) &&
		MatchPattern(v.Name, reqc.Name) &&
		MatchPattern(v.Operation, reqc.Operation) &&
		MatchPattern(v.Kind, reqc.Kind) &&
		MatchPattern(v.ApiVersion, apiVersion) &&
		MatchPattern(v.UserName, reqc.UserName) &&
		MatchPattern(v.Type, reqc.Type) &&
		MatchPattern(v.K8sCreatedBy, reqc.OrgMetadata.K8sCreatedBy) &&
		MatchPatternWithArray(v.UserGroup, reqc.UserGroups)

}

func (p *Policy) DeepCopyInto(p2 *Policy) {
	copier.Copy(&p2, &p)
}

func (p *Policy) DeepCopy() *Policy {
	p2 := &Policy{}
	p.DeepCopyInto(p2)
	return p2
}

func (p *Policy) Merge(p2 *Policy) *Policy {
	return &Policy{
		Enforce:                   append(p.Enforce, p2.Enforce...),
		IgnoreRequest:             append(p.IgnoreRequest, p2.IgnoreRequest...),
		AllowedSigner:             append(p.AllowedSigner, p2.AllowedSigner...),
		AllowedForInternalRequest: append(p.AllowedForInternalRequest, p2.AllowedForInternalRequest...),
		AllowedByRule:             append(p.AllowedByRule, p2.AllowedByRule...),
		AllowedChange:             append(p.AllowedChange, p2.AllowedChange...),
		PermitIfVerifiedOwner:     append(p.PermitIfVerifiedOwner, p2.PermitIfVerifiedOwner...),
		PermitIfCreator:           append(p.PermitIfCreator, p2.PermitIfCreator...),
		AllowUnverified:           append(p.AllowUnverified, p2.AllowUnverified...),
	}
}
