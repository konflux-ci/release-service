---
name: local-dev-setup
description: Use when onboarding to this repo, cloning for the first time, resetting a dev environment, or an agent needs to set up a working local instance.
---

# Local Development Setup

## Prerequisites

- [Go](https://go.dev/dl/) (check `go.mod` for minimum version)
- [Docker](https://docs.docker.com/get-docker/)
- [Git](https://git-scm.com/)

## Clone & Setup

```bash
git clone https://github.com/konflux-ci/release-service.git
cd release-service
```

## Install Dependencies

```bash
go mod tidy
```

## Build

```bash
go build ./...
```

## Verify Setup

Run the test suite to confirm everything works:

```bash
go test ./...
```

## Makefile

This project has a Makefile. List available targets:
```bash
make help
```

## Developer Scripts

Useful scripts in `hack/`:

- `hack/boilerplate.go.txt`

## IDE Setup

If using Cursor or VS Code, the repo may include:
- `.cursor/rules/` -- AI coding rules
- `.vscode/` -- editor settings
- `.editorconfig` -- formatting rules
