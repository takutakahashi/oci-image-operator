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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var AnnotationImageFlowTemplateDefaultDetect string = "build.takutakahashi.dev/default-template-detect"
var AnnotationImageFlowTemplateDefaultCheck string = "build.takutakahashi.dev/default-template-check"
var AnnotationImageFlowTemplateDefaultUpload string = "build.takutakahashi.dev/default-template-upload"
var AnnotationImageFlowTemplateDefaultAll string = "build.takutakahashi.dev/default-template-all"

// ImageFlowTemplateSpec defines the desired state of ImageFlowTemplate
type ImageFlowTemplateSpec struct {
	Detect ImageFlowTemplateSpecTemplate `json:"detect,omitempty"`
	Check  ImageFlowTemplateSpecTemplate `json:"check,omitempty"`
	Upload ImageFlowTemplateSpecTemplate `json:"upload,omitempty"`
}

type ImageFlowTemplateSpecTemplate struct {
	PodSpec v1.PodSpec `json:"podSpec,omitempty"`
}

// ImageFlowTemplateStatus defines the observed state of ImageFlowTemplate
type ImageFlowTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ImageFlowTemplate is the Schema for the imageflowtemplates API
type ImageFlowTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImageFlowTemplateSpec   `json:"spec,omitempty"`
	Status ImageFlowTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ImageFlowTemplateList contains a list of ImageFlowTemplate
type ImageFlowTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImageFlowTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ImageFlowTemplate{}, &ImageFlowTemplateList{})
}
