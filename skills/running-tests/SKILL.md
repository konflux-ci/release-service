---
name: running-tests
description: Use when writing, modifying, or verifying tests. Use when a PR needs test validation, CI is failing, or you need to run a specific test file or function.
---

# Running Tests

## Quick Start

```bash
make test
```

## Makefile Targets

- `make envtest`
- `make test`

## Running Specific Tests

```bash
go test ./path/to/package/...
go test -v -run TestFunctionName ./...
go test -race ./...  # recommended for CI
```

## Test Locations

- `integration-tests/`

## Linting (run before committing)

```bash
go vet ./...
```

```bash
make lint
```

## Output Format

When reporting test results, use:

```markdown
## Test Results

**Command:** `<command run>`
**Status:** PASS / FAIL
**Failed tests:** <list if any>
**Root cause:** <brief analysis if failing>
```
