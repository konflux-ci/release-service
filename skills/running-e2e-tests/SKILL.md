---
name: running-e2e-tests
description: Use when verifying cross-component behavior, debugging CI e2e failures, or testing deployment pipelines. Use when integration tests fail or a PR changes multiple services.
---

# Running E2E / Integration Tests

## E2E Test Locations

- `integration-tests/`

## Makefile Targets

Check available targets:
```bash
make help  # or: grep -E '^[a-zA-Z_-]+:' Makefile
```

Common e2e-related targets to look for: `make e2e`, `make integration`, `make test-e2e`

## Hack Scripts

- `hack/boilerplate.go.txt`

Check these scripts for cluster setup, test data seeding, or environment preparation.

## Running Go Integration Tests

```bash
# Many Go projects use build tags for integration tests
go test -tags=integration ./...
```

## Tips

- E2E tests may require a running cluster or external services
- Check CI workflow files for the exact setup steps used in automated runs
- Clean up test resources after running to avoid state leakage

## Output Format

When reporting e2e results, use:

```markdown
## E2E Test Results

**Test suite:** <name>
**Status:** PASS / FAIL
**Failed tests:** <test names>
**Category:** A (test code) / B (app code) / C (infra/flaky)
**Root cause:** <analysis>
**Suggested fix:** <file and change>
```
