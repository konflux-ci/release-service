package v1alpha1

import (
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"time"
)

// SetCreatedCondition updates the state of the release with a "Created" state.
func (rs *ReleaseStatus) SetCreatedCondition() {
	rs.setStatusCondition(metav1.ConditionUnknown, "Created", "")
}

// SetErrorCondition updates the state of the release with an "Error" state.
// The message field in the condition will contain the error message.
func (rs *ReleaseStatus) SetErrorCondition(err error) {
	rs.setStatusCondition(metav1.ConditionFalse, "Error", err.Error())
}

// SetFailedCondition records the CompletionTime and updates the state of the release with a "Failed" state.
func (rs *ReleaseStatus) SetFailedCondition(pipelineRun *tektonv1beta1.PipelineRun) {
	rs.CompletionTime = &metav1.Time{Time: time.Now()}

	rs.setStatusCondition(metav1.ConditionFalse,
		"Failed",
		pipelineRun.Status.GetCondition(apis.ConditionSucceeded).Message)
}

// SetRunningCondition records the StartTime, the name of the Release PipelineRun associated with this Release
// and updates the state of the release with a "Running" state.
func (rs *ReleaseStatus) SetRunningCondition(pipelineRun *tektonv1beta1.PipelineRun) {
	rs.StartTime = &metav1.Time{Time: time.Now()}
	rs.ReleasePipelineRun = pipelineRun.Name

	rs.setStatusCondition(metav1.ConditionUnknown, "Running", "")
}

// SetSucceededCondition records the CompletionTime and updates the state of the release with a "Succeeded" state.
func (rs *ReleaseStatus) SetSucceededCondition() {
	rs.CompletionTime = &metav1.Time{Time: time.Now()}

	rs.setStatusCondition(metav1.ConditionTrue, "Succeeded", "")
}

// setStatusCondition is a utility function to create a new Condition.
func (rs *ReleaseStatus) setStatusCondition(status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&rs.Conditions, metav1.Condition{
		Type:    "Succeeded",
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}

func (rs *ReleaseStatus) TrackReleasePipelineStatus(pipelineRun *tektonv1beta1.PipelineRun) {
	if pipelineRun.IsDone() {
		condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
		if condition.IsTrue() {
			rs.SetSucceededCondition()
		} else {
			rs.SetFailedCondition(pipelineRun)
		}
	} else if rs.ReleasePipelineRun == "" {
		rs.SetRunningCondition(pipelineRun)
	}
}
