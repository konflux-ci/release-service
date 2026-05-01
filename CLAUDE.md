# konflux-ci/release-service

go mod download
go mod tidy
make build
make test
go vet ./...
make lint

## Rules

- Follow existing code patterns in this repository
- Write tests for all changes
- Handle all errors; never use blank identifier for errors
- Use table-driven tests with t.Run()
- Run `go vet ./...` before committing
