## Summary

<!-- Describe the changes in this PR -->

## Related Issues

Fixes #

## Testing

- [ ] Tests added/updated
- [ ] All tests pass (`make test`)
- [ ] Changes verified locally

## Checklist

- [ ] Code follows Kubernetes coding conventions
- [ ] Commit messages follow conventional format: `type(RELEASE-NNNN): description`
- [ ] Documentation updated if needed
- [ ] Generated manifests updated (`make manifests generate`)

## Controller Promotion (skip if this is a Mintmaker/bot PR)

Once your infra-deployments development PR is merged, you are responsible for
promoting the release-service controller to staging and production.

- [ ] Infra-deployments `development` PR merged (automated by pipeline)
- [ ] Controller promoted to `staging` — [promote-overlay script](https://github.com/konflux-ci/release-service-utils/tree/main/ci/promote-overlay)
- [ ] Controller promoted to `production` — [promote-overlay script](https://github.com/konflux-ci/release-service-utils/tree/main/ci/promote-overlay)
