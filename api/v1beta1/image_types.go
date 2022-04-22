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
}

type ImageRepository struct {
	URL         string           `json:"url"`
	TagPolicies []ImageTagPolicy `json:"tagPolicies"`
}

type ImageTagPolicy struct {
	Policy   ImageTagPolicyType `json:"policy"`
	Revision string             `json:"revision"`
}

type ImageTagPolicyType string

var (
	ImageTagPolicyTypeBranchHash ImageTagPolicyType = "branchHash"
	ImageTagPolicyTypeBranchName ImageTagPolicyType = "branchName"
	ImageTagPolicyTypeTagHash    ImageTagPolicyType = "tagHash"
	ImageTagPolicyTypeTagName    ImageTagPolicyType = "tagName"
)

type ImageTarget struct {
	Name string    `json:"name"`
	Auth ImageAuth `json:"auth,omitempty"`
}

type ImageAuth struct {
	Type       ImageAuthType `json:"type"`
	SecretName string        `json:"secretName"`
}

type ImageAuthType string

var ImageAuthTypeBasic ImageAuthType = "basic"

// ImageStatus defines the observed state of Image
type ImageStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

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
