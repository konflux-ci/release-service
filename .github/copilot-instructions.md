# Copilot Instructions for konflux-ci/release-service

Run tests: `go test ./...`

Lint: `go vet ./...`

## Review Guidelines

- Verify error handling is complete
- Check test coverage for new code
- Ensure backward compatibility
- Verify all errors are checked (no `_ = err`)
- Check for goroutine leaks and proper context cancellation
