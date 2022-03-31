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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseLinkSpec defines the desired state of ReleaseLink.
type ReleaseLinkSpec struct {
	// DisplayName refers to the name of the ReleaseLink to link a user and managed workspace together
	// +required
	DisplayName string `json:"displayName"`

	// Application is a reference to the application to be released in the managed workspace
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Application string `json:"application"`

	// Target is a reference to the workspace to establish a link with
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Target string `json:"target"`

	// Release Strategy defines which strategy will be used to release the application in the managed workspace. This field has no effect for ReleaseLink resources in unmanaged workspaces
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	ReleaseStrategy string `json:"releaseStrategy,omitempty"`
}

// ReleaseLinkStatus defines the observed state of ReleaseLink.
type ReleaseLinkStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display Name",type=string,priority=1,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.application`
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target`
// +kubebuilder:printcolumn:name="Release Strategy",type=string,JSONPath=`.spec.releaseStrategy`

// ReleaseLink is the Schema for the releaselinks API.
type ReleaseLink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseLinkSpec   `json:"spec,omitempty"`
	Status ReleaseLinkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReleaseLinkList contains a list of ReleaseLink.
type ReleaseLinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReleaseLink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReleaseLink{}, &ReleaseLinkList{})
}
