package v1alpha1

import "github.com/konflux-ci/operator-toolkit/conditions"

const (
	// managedProcessedConditionType is the type used to track the status of a Release Managed Pipeline processing
	managedProcessedConditionType conditions.ConditionType = "ManagedPipelineProcessed"

	// postActionsExecutedConditionType is the type used to track the status of Release post-actions
	postActionsExecutedConditionType conditions.ConditionType = "PostActionsExecuted"

	// tenantProcessedConditionType is the type used to track the status of a Release Tenant Pipeline processing
	tenantProcessedConditionType conditions.ConditionType = "TenantPipelineProcessed"

	// releasedConditionType is the type used to track the status of a Release
	releasedConditionType conditions.ConditionType = "Released"

	// validatedConditionType is the type used to track the status of a Release validation
	validatedConditionType conditions.ConditionType = "Validated"
)

const (
	// FailedReason is the reason set when a failure occurs
	FailedReason conditions.ConditionReason = "Failed"

	// ProgressingReason is the reason set when a phase is progressing
	ProgressingReason conditions.ConditionReason = "Progressing"

	// SkippedReason is the reason set when a phase is skipped
	SkippedReason conditions.ConditionReason = "Skipped"

	// SucceededReason is the reason set when a phase succeeds
	SucceededReason conditions.ConditionReason = "Succeeded"
)
