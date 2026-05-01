# AGENTS.md

> Agent guidance for **konflux-ci/release-service**

Kubernetes controller to manage releases in Stonesoup

## Repository Map

```
konflux-ci/release-service/
  .github/ # GitHub config & workflows
  .tekton/ 
  api/ # API definitions
  cache/ 
  config/ # configuration
  controllers/ # controllers
  e2e-tests/ 
  git/ 
  hack/ # development scripts
  integration-tests/ 
  kubearchive/ 
  loader/ 
  metadata/ 
  metrics/ 
  syncer/ 
  tekton/ 
```

## Lint

- `go vet ./...`
- `make lint`

## See Also

- Workflow: `CONTRIBUTING.md`
- CI: `.github/workflows/`
