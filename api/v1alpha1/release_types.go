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
	"github.com/redhat-appstudio/application-service/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReleaseSpec defines the desired state of Release
type ReleaseSpec struct {
	// Component to be released
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Component string `json:"component"`

	// ReleaseLink referencing the workspace where the component will be released
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	ReleaseLink string `json:"releaseLink"`
}

// ReleaseStatus defines the observed state of Release
type ReleaseStatus struct {
	// Trigger reflects whether the Release is manual or automated
	// +kubebuilder:validation:Enum=automated;manual
	// +optional
	Trigger string `json:"trigger,omitempty"`

	// StartTime is the time when the Release PipelineRun was created and set to run
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the Release PipelineRun completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Conditions represent the latest available observations for the release
	// +optional
	Conditions []metav1.Condition `json:"conditions"`

	// ContractTaskRun references the TaskRun executed as part of this release to validate the enterprise contract
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	ContractTaskRun string `json:"contractTaskRun,omitempty"`

	// ReleasePipelineRun references the Release PipelineRun executed as part of this release
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	ReleasePipelineRun string `json:"releasePipelineRun,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Component",type=string,JSONPath=`.spec.component`
// -kubebuilder:printcolumn:name="Trigger",type=string,priority=1,JSONPath=`.status.trigger`
// +kubebuilder:printcolumn:name="Succeeded",type=string,JSONPath=`.status.conditions[?(@.type=="Succeeded")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Succeeded")].reason`
// +kubebuilder:printcolumn:name="PipelineRun",type=string,priority=1,JSONPath=`.status.releasePipelineRun`
// +kubebuilder:printcolumn:name="Start Time",type=date,priority=1,JSONPath=`.status.startTime`
// +kubebuilder:printcolumn:name="Completion Time",type=date,priority=1,JSONPath=`.status.completionTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Release is the Schema for the releases API
type Release struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ReleaseSpec `json:"spec,omitempty"`

	//+kubebuilder:default={trigger: "manual"}
	Status ReleaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReleaseList contains a list of Release
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Release `json:"items"`
}

// NewRelease returns a new Release object created in the same namespace that the given Component.
func NewRelease(component *v1alpha1.Component, releaseLink *ReleaseLink) *Release {
	return &Release{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "release-",
			Namespace:    component.Namespace,
		},
		Spec: ReleaseSpec{
			Component:   component.Name,
			ReleaseLink: releaseLink.Name,
		},
	}
}

func init() {
	SchemeBuilder.Register(&Release{}, &ReleaseList{})
}
