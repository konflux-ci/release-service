# AGENTS.md

Release Service is a Kubernetes operator that orchestrates software release pipelines in Konflux CI. It watches Release CRs, validates them, creates Tekton PipelineRuns, tracks their progress, and records results.

## Technology Stack

- **Language**: Go 
- **Framework:** controller-runtime
- **CRDs**: Release, ReleasePlan, ReleasePlanAdmission, ReleaseServiceConfig
- **Pipeline engine**: Tekton PipelineRuns
- **Testing**: Ginkgo/Gomega + envtest (local K8s API server)
- **Build**: `make test` (unit), `make manifests generate` (codegen)

## Repository Structure

```
api/v1alpha1/          # CRD types and webhooks
controllers/           # Reconcilers: release/, releaseplan/, releaseplanadmission/
loader/                # ObjectLoader interface — abstracts K8s resource fetching
syncer/                # Cross-namespace Snapshot syncing
tekton/                # PipelineRun builders, status helpers, watch predicates
metadata/              # Label/annotation/finalizer constants and utilities
metrics/               # Prometheus gauges, histograms, counters for release lifecycle
config/                # Kustomize manifests (CRDs, RBAC, webhooks, samples)
main.go                # Entry point — registers controllers and webhooks
```

## Architecture

### Adapter Pattern

Each controller delegates to an **adapter** (`adapter.go`) that holds the K8s client, resource under reconciliation, an `ObjectLoader`, and a `Syncer`. All domain logic lives in adapter methods, not in the controller.

### Reconciliation Pipeline

The Release controller runs ~20 sequential **operations** via `controller.ReconcileHandler()` from operator-toolkit. Each returns `(OperationResult, error)` — failure requeues, success continues. The pipeline stages are:

1. Finalizer management and config loading
2. Validation (author, application, pipeline source, Enterprise Contract)
3. Pipeline processing in order: tenant collectors, managed collectors, tenant, managed, final
4. Completion and cleanup

PipelineRuns are watched via `EnqueueRequestForAnnotation` — when a PipelineRun updates, the owning Release is re-reconciled.

### Resource Loading

`loader.ObjectLoader` centralizes all K8s Gets with error classification (retriable vs permanent) and KubeArchive fallback for deleted resources. A mock implementation exists for tests.

## Development Guidelines

- **Git**: conventional commits with Jira ticket as scope — `type(RELEASE-NNNN): description` (e.g. `feat(RELEASE-2119): add failure inspection`)
- Follow [Kubernetes coding conventions](https://github.com/kubernetes/community/blob/master/contributors/guide/coding-conventions.md)
- Log via `ctrl.LoggerFrom(ctx)`, wrap errors with `fmt.Errorf("context: %w", err)`
- **API changes**: edit types in `api/v1alpha1/`, then `make generate manifests`
- **Controller changes**: implement in `controllers/<resource>/adapter.go`
- **Webhooks**: add to `api/v1alpha1/webhooks/<resource>/`
- **Tests**: unit tests alongside code using Ginkgo + envtest; E2E in dedicated repos

## Key Patterns

- **PipelineRunBuilder** (`tekton/utils/`): fluent API — `.WithParams()`, `.WithWorkspace()`, `.WithPipelineRef()`, `.WithTimeouts()`
- **Metadata utilities** (`metadata/`): prefix-based label filtering, safe-copy helpers that don't clobber existing keys
- **Metrics**: registered during reconciliation — `RegisterNewRelease()`, `RegisterCompletedRelease()`, `RegisterValidatedRelease()` with start/completion times
- **Syncer**: copies Snapshot metadata to target namespace idempotently (ignores AlreadyExists)
