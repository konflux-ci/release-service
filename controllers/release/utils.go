package release

import (
	"context"

	"github.com/konflux-ci/operator-toolkit/controller"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/loader"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// maxConditionMessageLength is the maximum length for a Kubernetes condition message.
// This limit is enforced by the API server validation (MaxLength=32768).
const maxConditionMessageLength = 31000

// handlePipelineCreationError handles errors returned when creating a PipelineRun or RoleBinding.
// For retriable (transient) errors such as network timeouts or conflicts, it returns the original
// error so the caller can requeue. For permanent errors (e.g., admission webhook denials), it
// marks the pipeline and release as failed and continues reconciliation so that downstream
// Ensure* operations can mark their own phases as skipped via their IsFailed() guards. Without
// continuing, no future reconcile is triggered (no PipelineRun was created, and
// GenerationChangedPredicate ignores status-only patches), leaving downstream pipeline conditions
// permanently unset.
//
// The optional extraStatusUpdates callbacks run after the patch baseline is taken, so their
// changes are included in the same atomic status patch as the failure marks. This is used to
// persist RoleBinding refs that were created before the failure, so that
// EnsureCollectorsProcessingResourcesAreCleanedUp can find and delete them on the next reconcile.

// pipelineCreationResult converts the (retryable, err) pair returned by handlePipelineCreationError
// into the appropriate OperationResult: RequeueWithError for retryable errors, RequeueOnErrorOrContinue
// for permanent errors (where err is the patch error, which may be nil on success).
func pipelineCreationResult(retryable bool, err error) (controller.OperationResult, error) {
	if retryable {
		return controller.RequeueWithError(err)
	}
	return controller.RequeueOnErrorOrContinue(err)
}

// handlePipelineCreationError returns (true, originalErr) for retryable errors so the caller
// can requeue via RequeueWithError. For permanent errors it marks the pipeline and release as
// failed, patches the status, and returns (false, patchErr) so the caller can continue via
// RequeueOnErrorOrContinue.
func handlePipelineCreationError(ctx context.Context, c client.Client, release *v1alpha1.Release, err error, markProcessing func(), markPipelineFailed func(string), releaseFailedMsg string, extraStatusUpdates ...func()) (bool, error) {
	if loader.IsRetryableCreationError(err) {
		return true, err
	}
	patch := client.MergeFrom(release.DeepCopy())
	for _, update := range extraStatusUpdates {
		update()
	}
	markProcessing()
	errMsg := err.Error()
	if len(errMsg) > maxConditionMessageLength {
		errMsg = errMsg[:maxConditionMessageLength]
	}
	markPipelineFailed(errMsg)
	release.MarkReleaseFailed(releaseFailedMsg)
	return false, c.Status().Patch(ctx, release, patch)
}
