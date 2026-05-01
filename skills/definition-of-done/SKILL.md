---
name: definition-of-done
description: Use when preparing a PR for review, checking if a change is complete, or an agent needs to verify its own work before submitting. Use when you need the PR checklist.
---

# Definition of Done

Before submitting a PR, ensure all items are checked:

## Code Quality

- [ ] Code follows existing patterns and conventions in this repository
- [ ] No commented-out code or debug statements left behind
- [ ] Functions and variables have clear, descriptive names

## Testing

- [ ] All tests pass: `go test ./...`
- [ ] New functionality has corresponding test coverage
- [ ] Edge cases and error paths are tested

## Linting

- [ ] Linter passes: `go vet ./...`

## CI Checks

- [ ] All CI workflows pass:
  - Code Coverage Report
  - Go Test on Pull Requests
  - PR AI Labeler
  - Assign PR
  - Go Test on Pull Requests
  - Dependabot Updates
  - Dependency Graph

## Documentation

- [ ] Public APIs or user-facing changes are documented
- [ ] README updated if behavior changes
- [ ] Commit messages are clear and follow project conventions

## PR Hygiene

- [ ] PR is focused on a single concern
- [ ] PR description explains WHAT changed and WHY
- [ ] No unrelated changes bundled in
