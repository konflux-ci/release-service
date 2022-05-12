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
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseSpec defines the desired state of Release
type ReleaseSpec struct {
	// ApplicationSnapshot to be released
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	ApplicationSnapshot string `json:"applicationSnapshot"`

	// ReleaseLink referencing the workspace where the snapshot will be released
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	ReleaseLink string `json:"releaseLink"`
}

// ReleaseReason represents a reason for the release "Succeeded" condition
type ReleaseReason string

const (
	// releaseConditionType is the type used when setting a release status condition
	releaseConditionType string = "Succeeded"

	// ReleaseReasonValidationError is the reason set when the Release validation failed
	ReleaseReasonValidationError ReleaseReason = "ReleaseValidationError"

	// ReleaseReasonPipelineFailed is the reason set when the release PipelineRun failed
	ReleaseReasonPipelineFailed ReleaseReason = "ReleasePipelineFailed"

	// ReleaseReasonRunning is the reason set when the release PipelineRun starts running
	ReleaseReasonRunning ReleaseReason = "Running"

	// ReleaseReasonSucceeded is the reason set when the release PipelineRun has succeeded
	ReleaseReasonSucceeded ReleaseReason = "Succeeded"
)

func (rr ReleaseReason) String() string {
	return string(rr)
}

// ReleaseStatus defines the observed state of Release
type ReleaseStatus struct {
	// StartTime is the time when the Release PipelineRun was created and set to run
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the Release PipelineRun completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Conditions represent the latest available observations for the release
	// +optional
	Conditions []metav1.Condition `json:"conditions"`

	// ReleasePipelineRun contains the namespaced name of the release PipelineRun executed as part of this release
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?\/[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	ReleasePipelineRun string `json:"releasePipelineRun,omitempty"`
}

// MarkFailed registers the completion time and changes the Succeeded condition to False with
// the provided reason and message.
func (rs *ReleaseStatus) MarkFailed(reason ReleaseReason, message string) {
	rs.CompletionTime = &metav1.Time{Time: time.Now()}
	rs.setStatusConditionWithMessage(metav1.ConditionFalse, reason, message)
}

// MarkInvalid changes the Succeeded condition to False with the provided reason and message.
func (rs *ReleaseStatus) MarkInvalid(reason ReleaseReason, message string) {
	rs.setStatusConditionWithMessage(metav1.ConditionFalse, reason, message)
}

// MarkRunning registers the start time and changes the Succeeded condition to Unknown.
func (rs *ReleaseStatus) MarkRunning() {
	rs.StartTime = &metav1.Time{Time: time.Now()}
	rs.setStatusCondition(metav1.ConditionUnknown, ReleaseReasonRunning)
}

// MarkSucceeded registers the completion time and changes the Succeeded condition to True.
func (rs *ReleaseStatus) MarkSucceeded() {
	rs.CompletionTime = &metav1.Time{Time: time.Now()}
	rs.setStatusCondition(metav1.ConditionTrue, ReleaseReasonSucceeded)
}

// SetCondition creates a new condition with the given status and reason. Then, it sets this new condition,
// unsetting previous conditions with the same type as necessary.
func (rs *ReleaseStatus) setStatusCondition(status metav1.ConditionStatus, reason ReleaseReason) {
	rs.setStatusConditionWithMessage(status, reason, "")
}

// SetCondition creates a new condition with the given status, reason and message. Then, it sets this new condition,
// unsetting previous conditions with the same type as necessary.
func (rs *ReleaseStatus) setStatusConditionWithMessage(status metav1.ConditionStatus, reason ReleaseReason, message string) {
	meta.SetStatusCondition(&rs.Conditions, metav1.Condition{
		Type:    releaseConditionType,
		Status:  status,
		Reason:  reason.String(),
		Message: message,
	})
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Snapshot",type=string,JSONPath=`.spec.applicationSnapshot`
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

	Spec   ReleaseSpec   `json:"spec,omitempty"`
	Status ReleaseStatus `json:"status,omitempty"`
}

// HasStarted checks whether the Release has a valid start time set in its status.
func (r *Release) HasStarted() bool {
	return r.Status.StartTime != nil && !r.Status.StartTime.IsZero()
}

// IsDone returns true if the Release's status indicates that it is done.
func (r *Release) IsDone() bool {
	return !meta.IsStatusConditionPresentAndEqual(r.Status.Conditions, releaseConditionType, metav1.ConditionUnknown)
}

// +kubebuilder:object:root=true

// ReleaseList contains a list of Release
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Release `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Release{}, &ReleaseList{})
}
