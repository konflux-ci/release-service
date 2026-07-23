---
name: create-operation
description: Conventions for adding a new operation to a controller's reconciliation pipeline.
---

# Creating a New Operation

An **operation** is a method on the adapter struct that represents one step in the controller's reconciliation pipeline. Operations are executed sequentially by `controller.ReconcileHandler()` from operator-toolkit. Each operation must be idempotent — safe to re-run at any point if the reconciliation is interrupted and restarted.

## Operation signature

Every operation must have this exact signature:

```go
func (a *adapter) EnsureSomethingHappens() (controller.OperationResult, error)
```

The name must start with `Ensure` and describe the desired end state, not the action taken. Examples: `EnsureFinalizerIsAdded`, `EnsureReleaseIsCompleted`, `EnsureManagedPipelineIsProcessed`.

## Return values

Use the result helpers from `github.com/konflux-ci/operator-toolkit/controller`:

| Helper | When to use |
|---|---|
| `controller.ContinueProcessing()` | Work done or skipped — proceed to next operation |
| `controller.StopProcessing()` | Reconciliation should end (e.g. resource already finished) |
| `controller.Requeue()` | Requeue immediately, stop remaining operations |
| `controller.RequeueWithError(err)` | Requeue due to error |
| `controller.RequeueOnErrorOrContinue(err)` | If err is non-nil, requeue; otherwise continue. **Most common for status patches** |
| `controller.RequeueOnErrorOrStop(err)` | If err is non-nil, requeue; otherwise stop |
| `controller.RequeueAfter(delay, err)` | Requeue after a specific delay |

## Operation structure

Every operation follows this skeleton:

```go
// EnsureXxxIsYyy is an operation that will ensure that <describe the desired state>.
func (a *adapter) EnsureXxxIsYyy() (controller.OperationResult, error) {
    // 1. Gate condition — skip if already done or prerequisites not met
    if a.release.HasXxxFinished() || !a.release.HasPrerequisiteFinished() {
        return controller.ContinueProcessing()
    }

    // 2. Failure skip — if the release has already failed, mark this phase as skipped
    if a.release.IsFailed() {
        patch := client.MergeFrom(a.release.DeepCopy())
        a.release.MarkXxxSkipped()
        return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
    }

    // 3. Load resources via the loader
    resource, err := a.loader.GetSomeResource(a.ctx, a.client, a.release)
    if err != nil {
        return controller.RequeueWithError(err)
    }

    // 4. Perform the work (create objects, update state, etc.)

    // 5. Patch status and return
    patch := client.MergeFrom(a.release.DeepCopy())
    a.release.MarkXxxDone()
    return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
}
```

## Gate conditions and idempotency

Every operation must start with a gate condition that checks whether the work has already been done. This makes reconciliation safe to restart at any point.

Common gate patterns:

```go
// Already done — skip
if a.release.HasXxxFinished() {
    return controller.ContinueProcessing()
}

// Prerequisite not met — skip (will run on next reconcile when prerequisite completes)
if !a.release.HasPrerequisiteFinished() {
    return controller.ContinueProcessing()
}

// Combined: already done OR prerequisite not met
if a.release.HasXxxFinished() || !a.release.HasPrerequisiteFinished() {
    return controller.ContinueProcessing()
}

// Tracking operation — only runs while actively processing
if !a.release.IsXxxProcessing() || a.release.HasXxxFinished() {
    return controller.ContinueProcessing()
}
```

If the release has already failed (e.g. a previous operation marked it as failed), downstream operations must mark their own phase as **skipped** and continue. They must NOT stop processing — stopping would leave downstream pipeline conditions permanently unset because status-only patches don't trigger `GenerationChangedPredicate`:

```go
if a.release.IsFailed() {
    patch := client.MergeFrom(a.release.DeepCopy())
    a.release.MarkXxxSkipped()
    return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
}
```

## Status patching discipline

Always use the merge-from-deep-copy pattern:

```go
patch := client.MergeFrom(a.release.DeepCopy())
// ... mutate a.release.Status fields ...
return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
```

Rules:

- Use `a.client.Status().Patch()` for status subresource updates.
- Use `a.client.Patch()` for metadata updates (finalizers, labels, annotations).
- Only patch in operations that change state. Read-only and tracking operations that find no changes must NOT patch.
- Take the `DeepCopy()` **before** any mutations — this is the baseline for the merge patch.

## Error handling for resource creation

When an operation creates Kubernetes resources (PipelineRuns, RoleBindings), use `handlePipelineCreationError` from `controllers/release/utils.go` to classify errors:

- **Retryable errors** (network timeouts, conflicts): requeue with the original error.
- **Permanent errors** (forbidden, invalid, bad request): mark the pipeline and release as failed, then **continue** (not stop) so downstream operations can mark their phases as skipped.

```go
pipelineRun, err := a.createSomePipelineRun(...)
if err != nil {
    return pipelineCreationResult(handlePipelineCreationError(a.ctx, a.client, a.release, err,
        a.release.MarkXxxProcessing,
        a.release.MarkXxxFailed,
        "Release processing failed on xxx pipelineRun creation"))
}
```

If resources were created before the failure (e.g. RoleBindings created before PipelineRun creation fails), pass `extraStatusUpdates` callbacks to persist their references in the same atomic patch. This allows cleanup operations to find and delete them on the next reconcile:

```go
return pipelineCreationResult(handlePipelineCreationError(a.ctx, a.client, a.release, err,
    a.release.MarkXxxProcessing,
    a.release.MarkXxxFailed,
    "Release processing failed on xxx pipelineRun creation",
    func() {
        if capturedRoleBinding != nil {
            a.release.Status.XxxProcessing.RoleBindings.TenantRoleBinding =
                fmt.Sprintf("%s%c%s", capturedRoleBinding.Namespace, types.Separator, capturedRoleBinding.Name)
        }
    }))
```

## Loading resources

Always load resources through `a.loader` (the `loader.ObjectLoader` interface), never directly via `a.client.Get()`. The loader provides:

- Error classification (retriable vs permanent)
- KubeArchive fallback for deleted resources
- A mock implementation for tests

## Registering the operation

After implementing the operation, add it to the operation slice in the controller's `Reconcile` method (`controllers/<resource>/controller.go`):

```go
return controller.ReconcileHandler([]controller.Operation{
    // ... existing operations ...
    adapter.EnsureXxxIsYyy,  // Add in the correct position
    // ... remaining operations ...
})
```

**Order matters.** Operations execute sequentially and any one can short-circuit the pipeline. Place the new operation:

- After its prerequisites (operations whose results it depends on)
- Before operations that depend on its results
- Processing operations before their corresponding tracking operations
- All processing/tracking before `EnsureReleaseIsCompleted`

## Writing tests

Tests use Ginkgo/Gomega with envtest. Each test file lives alongside the code it tests in the same package.

### What to test

For each operation, test:

1. **Gate condition**: returns `ContinueProcessing` when the work is already done or prerequisites aren't met.
2. **Failure skip**: marks the phase as skipped and continues when `IsFailed()`.
3. **Loader errors**: requeues with error when resource loading fails.
4. **Happy path**: performs the work and returns the expected result.
5. **Creation errors** (if applicable): retryable errors requeue, permanent errors mark failure and continue.
6. **Status patch**: the correct status fields are set after the operation completes.

## Verification

After implementing the operation and its tests:

```bash
go vet ./controllers/<resource>/
go build ./controllers/<resource>/
go test ./controllers/<resource>/
```

Use `go vet` and `go build` for fast iteration on a single package, then run `go test` for the full test suite of that package.

## Checklist

- [ ] Operation method on adapter with `Ensure` prefix and correct signature
- [ ] Doc comment describing the desired end state
- [ ] Gate condition at the top (idempotency)
- [ ] Failure skip with `MarkXxxSkipped` if applicable
- [ ] Resources loaded via `a.loader`, not `a.client.Get()`
- [ ] Status patches use `client.MergeFrom(a.release.DeepCopy())`
- [ ] Creation errors handled via `handlePipelineCreationError` if applicable
- [ ] Operation registered in the controller's `Reconcile` method in the correct position
- [ ] Tests cover gate conditions, error paths, and happy path
- [ ] `go vet`, `go build`, and `go test` pass for the package
