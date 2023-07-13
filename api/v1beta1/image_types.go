/*
Copyright 2022.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// 3 phase: check, exists, upload

// ImageSpec defines the desired state of Image
type ImageSpec struct {
	TemplateName string          `json:"templateName,omitempty"`
	Repository   ImageRepository `json:"repository"`
	Targets      []ImageTarget   `json:"targets"`
	Env          []corev1.EnvVar `json:"env,omitempty"`
}

type ImageRepository struct {
	URL         string           `json:"url"`
	Auth        ImageAuth        `json:"auth,omitempty"`
	TagPolicies []ImageTagPolicy `json:"tagPolicies,omitempty"`
}

type ImageTagPolicy struct {
	Policy   ImageTagPolicyType `json:"policy,omitempty"`
	Revision string             `json:"revision,omitempty"`
}

type ImageTagPolicyType string

var (
	ImageTagPolicyTypeBranchHash ImageTagPolicyType = "branchHash"
	ImageTagPolicyTypeBranchName ImageTagPolicyType = "branchName"
	ImageTagPolicyTypeTagHash    ImageTagPolicyType = "tagHash"
	ImageTagPolicyTypeTagName    ImageTagPolicyType = "tagName"
	ImageTagPolicyTypeUnused     ImageTagPolicyType = "unused"
)

type ImageTarget struct {
	Name string    `json:"name"`
	Auth ImageAuth `json:"auth,omitempty"`
	// TODO: add context and dockerfile path for building monorepo
}

type ImageAuth struct {
	Type       ImageAuthType `json:"type"`
	SecretName string        `json:"secretName"`
}

type ImageAuthType string

var ImageAuthTypeBasic ImageAuthType = "basic"

// ImageStatus defines the observed state of Image
type ImageStatus struct {
	Conditions []ImageCondition `json:"conditions,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

type ImageCondition struct {

	// Last time the condition transitioned from one status to another.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Type of Condition. ex: Detected, Checked, Uploaded
	Type ImageConditionType `json:"type,omitempty"`

	// Status is the status of the condition. Can be True, False, Unknown.
	Status           ImageConditionStatus `json:"status,omitempty"`
	Revision         string               `json:"revision,omitempty"`
	ResolvedRevision string               `json:"resolvedRevision,omitempty"`
	TagPolicy        ImageTagPolicyType   `json:"tagPolicy,omitempty"`
}

type ImageConditionType string

var (
	ImageConditionTypeDetected ImageConditionType = "detected"
	ImageConditionTypeChecked  ImageConditionType = "checked"
	ImageConditionTypeUploaded ImageConditionType = "uploaded"
)

type ImageConditionStatus string

var (
	ImageConditionStatusTrue    ImageConditionStatus = "True"
	ImageConditionStatusFalse   ImageConditionStatus = "False"
	ImageConditionStatusFailed  ImageConditionStatus = "failed"
	ImageConditionStatusUnknown ImageConditionStatus = "Unknown"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Image is the Schema for the images API
type Image struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImageSpec   `json:"spec,omitempty"`
	Status ImageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ImageList contains a list of Image
type ImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Image `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Image{}, &ImageList{})
}
