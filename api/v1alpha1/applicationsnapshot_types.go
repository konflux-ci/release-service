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

// Images defines the various component images associated with the ApplicationSnapshot
type Image struct {
	// Component is the component name of the image
	// +required
	Component string `json:"component"`

	// PullSpec is the particular version of the component's container image
	// +required
	PullSpec string `json:"pullSpec"`
}

// ApplicationSnapshotSpec defines the desired state of ApplicationSnapshot
type ApplicationSnapshotSpec struct {
	// Application is a reference to the application that this ApplicationSnapshot belongs to
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Application string `json:"application"`

	// Images is the list of component images that makes up the snapshot
	// +required
	Images []Image `json:"images"`
}

type ApplicationSnapshotStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.application`

// ApplicationSnapshot is the Schema for the applicationsnapshots API
type ApplicationSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSnapshotSpec   `json:"spec,omitempty"`
	Status ApplicationSnapshotStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationSnapshotList contains a list of ApplicationSnapshot
type ApplicationSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationSnapshot{}, &ApplicationSnapshotList{})
}
