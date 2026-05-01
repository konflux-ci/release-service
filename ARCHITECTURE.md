# Architecture

## konflux-ci/release-service

Kubernetes controller to manage releases in Stonesoup

**Languages:** Go (99%), Makefile (1%), Dockerfile (0%)

## Directory Structure

```
release-service/
  .github/ (9 files)
  .tekton/ (2 files)
  api/ (34 files)
  cache/ (1 files)
  config/ (101 files)
  controllers/ (28 files)
  e2e-tests/ (25 files)
  git/ (3 files)
  hack/ (1 files)
  integration-tests/ (2 files)
  kubearchive/ (3 files)
  loader/ (5 files)
  metadata/ (4 files)
  metrics/ (3 files)
  syncer/ (3 files)
  tekton/ (12 files)
```

## Components

### `.github/`

Github config & workflows (9 files)

### `api/`

Api definitions (34 files)

### `config/`

Configuration (101 files)

### `controllers/`

Controllers (28 files)

### `hack/`

Development scripts (1 files)

## CI/CD

This project uses GitHub Actions with 7 workflow(s):

- Code Coverage Report
- Go Test on Pull Requests
- PR AI Labeler
- Assign PR
- Go Test on Pull Requests
- Dependabot Updates
- Dependency Graph

## Dependencies

Managed via:

- `go.mod`
- `go.sum`

## Design Decisions

<!-- Document key architectural decisions here -->
<!-- Format: ### Decision Title -->
<!-- **Context:** Why was this decision needed? -->
<!-- **Decision:** What was decided? -->
<!-- **Consequences:** What are the trade-offs? -->
