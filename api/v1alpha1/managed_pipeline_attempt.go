package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	AttemptSucceededReason      = "Succeeded"
	AttemptFailedReason         = "Failed"
	AttemptProgressingReason    = "Progressing"
	AttemptFailureErrorReason   = "Error"
	AttemptFailureOOMKillReason = "OOMKill"
	AttemptFailureTimeoutReason = "Timeout"
)

// ManagedPipelineAttempt defines the observed state of a managed pipeline processing attempt
type ManagedPipelineAttempt struct {
	// PipelineRun contains the namespaced name of the managed Release PipelineRun executed as part of this attempt
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?\/[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	PipelineRun string `json:"pipelineRun,omitempty"`

	// RoleBindings defines the roleBindings for accessing resources during the managed pipeline attempt
	// +optional
	RoleBindings RoleBindingType `json:"roleBindings,omitempty"`

	// StartTime is the time when the managed pipeline attempt started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time when the managed pipeline attempt completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Status is the outcome of the managed pipeline attempt
	// +optional
	Status string `json:"status,omitempty"`

	// FailureReason is the failure type when the PipelineRun fails
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// LastTask is the name of the last task that executed or failed
	// +optional
	LastTask string `json:"lastTask,omitempty"`

	// LastStep is the name of the last step that executed or failed within the last task
	// +optional
	LastStep string `json:"lastStep,omitempty"`

	// SuccessfulTasks is the number of tasks that completed successfully
	// +optional
	SuccessfulTasks int `json:"successfulTasks,omitempty"`
}
