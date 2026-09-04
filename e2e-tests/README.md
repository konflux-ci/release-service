# Release Service E2E Tests

End-to-end tests for the [Release Service](https://github.com/konflux-ci/release-service) using Ginkgo/Gomega.

## Quick Start

```bash
# Set required tokens
export QUAY_TOKEN='{"auths":{"quay.io":{"auth":"..."}}}'
export RELEASE_CATALOG_TA_QUAY_TOKEN='{"auths":{"quay.io":{"auth":"..."}}}'

# Run all e2e tests (from project root)
make test-e2e
```

## Prerequisites

| Requirement | Version |
|-------------|---------|
| Go | 1.23+ |
| Kubernetes/OpenShift cluster | With Release Service deployed |
| `kubectl` / `oc` | Configured with cluster access |

## Environment Variables

### Required

| Variable | Format | Description |
|----------|--------|-------------|
| `QUAY_TOKEN` | `dockerconfigjson` | Quay.io auth for image push/pull |
| `RELEASE_CATALOG_TA_QUAY_TOKEN` | `dockerconfigjson` | Quay auth for trusted artifacts |

**Token Format Example:**

```json
{
  "auths": {
    "quay.io/your-org/your-repo": {
      "auth": "base64-encoded-credentials"
    }
  }
}
```

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `RELEASE_SERVICE_CATALOG_URL` | `https://github.com/konflux-ci/release-service-catalog` | Release pipeline catalog repo |
| `RELEASE_SERVICE_CATALOG_REVISION` | `development` | Catalog git branch/tag |
| `RELEASE_SERVICE_OPERATOR_URL` | `https://github.com/konflux-ci/release-service.git` | Release Service Operator repo |
| `RELEASE_SERVICE_OPERATOR_REVISION` | `main` | Git branch/tag |
| `E2E_APPLICATIONS_NAMESPACE` | *auto-generated* | Override test namespace |

## Running Tests

Run from the **project root** directory:

```bash
make test-e2e                          # Run all tests
make test-e2e LABEL=happy-path         # Run by label
make test-e2e FOCUS="tenant"           # Run by name pattern
make test-e2e SKIP="negative"          # Skip tests matching pattern
make test-e2e E2E_TIMEOUT=120m         # Custom timeout
make test-e2e-list                     # List all tests
```

**Combine options:**

```bash
make test-e2e LABEL=happy-path E2E_TIMEOUT=90m
make test-e2e LABEL="release-service && !release-neg"
make test-e2e LABEL=negManagedPipelineRunCreationDenied
```

## Test Labels

| Label | Description |
|-------|-------------|
| `release-service` | All release service tests |
| `happy-path` | Full release flow with managed pipeline |
| `tenant` | Tenant-only pipeline (no managed namespace) |
| `release_plan_and_admission` | ReleasePlan ↔ ReleasePlanAdmission matching |
| `release-neg` | Negative/error scenarios |
| `negMissingReleasePlan` | Release fails when no matching `ReleasePlan` or `ReleasePlanAdmission` is found |
| `negBlockReleases` | Release fails when `block-releases: "true"` is set on the `ReleasePlanAdmission` |
| `negManagedPipelineRunCreationDenied` | Managed release `PipelineRun` create denied by quota; failure is surfaced on `Release` status |
| `negTenantPipelineInvalidGitRef` | Tenant pipeline with invalid git resolver config (empty URL/revision/pathInRepo); failure is surfaced on `Release` status |
| `retry-managed` | All managed pipeline retry scenarios |
| `managed-pipeline-oom-retry` | OOM failure on the managed pipeline is retried with memory limit mitigation |
| `managed-pipeline-task-timeout-retry` | `TaskRunTimeout` on the managed pipeline is retried with task timeout mitigation |
| `managed-pipeline-pipeline-timeout-retry` | `PipelineRunTimeout` on the managed pipeline is retried with pipeline/tasks timeout mitigation |
| `managed-pipeline-error-no-retry` | Generic error failure on the managed pipeline is not retried |
| `managed-pipeline-max-retries-zero` | OOM failure is not retried when `MaxRetries=0` is set on the RPA, overriding the RSC retry policy |
| `managed-pipeline-taskrunspecs-oom-retry` | OOM failure on a task whose memory limit is set via RPA `TaskRunSpecs` is retried with the mitigation applied to the overridden limit |
| `managed-pipeline-retry-final-pipeline-ordering` | Verifies that the final pipeline does not start until all managed pipeline retries have completed in a task-timeout retry scenario |
| `managed-pipeline-disabled-by-tag-no-retry` | OOM failure is not retried when the RPA mapping data carries a tag matched by the RSC `RetryPolicy.DisableOn.Tags` |
| `managed-pipeline-retry-exhausted` | Repeated OOM failures exhaust all configured retries (mitigation capped below the OOM threshold), and the Release ultimately fails |
| `final` | Final pipeline execution and finalizer test |

## Writing Tests

See [Ginkgo documentation](https://onsi.github.io/ginkgo/) for writing tests. Use existing tests in `tests/release/service/` as examples.
