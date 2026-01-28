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
```

## Test Labels

| Label | Description |
|-------|-------------|
| `release-service` | All release service tests |
| `happy-path` | Full release flow with managed pipeline |
| `tenant` | Tenant-only pipeline (no managed namespace) |
| `release_plan_and_admission` | ReleasePlan ↔ ReleasePlanAdmission matching |
| `release-neg` | Negative/error scenarios |

## Writing Tests

See [Ginkgo documentation](https://onsi.github.io/ginkgo/) for writing tests. Use existing tests in `tests/release/service/` as examples.
