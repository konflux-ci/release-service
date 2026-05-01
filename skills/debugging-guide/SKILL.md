---
name: debugging-guide
description: Use when investigating bugs, unexpected behavior, test failures, or CI errors. Use when logs show errors, tests fail intermittently, or the application crashes.
---

# Debugging Guide

## General Approach

1. Reproduce the issue with a minimal test case
2. Check logs for error messages and stack traces
3. Use a debugger or add targeted logging
4. Verify the fix with a test before submitting

## Go Debugging

Use delve for interactive debugging:
```bash
dlv test ./path/to/package -- -test.run TestName
```

Enable verbose logging:
```bash
go test -v -count=1 ./...
```

Profile a slow test:
```bash
go test -cpuprofile cpu.prof -memprofile mem.prof ./...
```

## Container Debugging

Build and run locally:
```bash
docker build -t debug-image .
docker run -it --entrypoint /bin/sh debug-image
```

Check container logs:
```bash
docker logs <container-id>
```

## Kubernetes Debugging

Check pod status and logs:
```bash
kubectl get pods -n <namespace>
kubectl logs -f <pod-name> -n <namespace>
kubectl describe pod <pod-name> -n <namespace>
```

Exec into a running pod:
```bash
kubectl exec -it <pod-name> -n <namespace> -- /bin/sh
```

## Common Issues

- **Tests pass locally but fail in CI**: Check for environment differences, missing env vars, or timing-dependent tests
- **Flaky tests**: Look for shared state, race conditions, or network dependencies
- **Build failures**: Verify dependency versions match what CI uses
