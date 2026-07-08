# Konflux Release service

Release service is a Kubernetes operator to control the life cycle of Konflux-managed releases.

## Development

All development tasks use the [Makefile](Makefile).

**Setup from fresh clone**:
```shell
$ make setup  # One command to set up development environment
$ make test   # Run tests to verify setup
```

**Run locally** (e.g., CRC cluster):
```shell
$ make manifests generate  # After code changes
$ make run install
```

**Build and push image**:
```shell
$ make docker-build docker-push
$ TAG=my-tag make docker-build docker-push  # Custom tag
$ IMG=quay.io/user/release:my-tag make docker-build docker-push  # Custom repo
```

**Run tests**:
```shell
$ make test  # Includes coverage report
```

**Disable webhooks** (local development):
```shell
$ ENABLE_WEBHOOKS=false make run install
```

## Metrics

Apart from the [metrics provided by controller-runtime](https://book.kubebuilder.io/reference/metrics-reference.html)
by default, this operator exports the following custom metrics:

| Metric name                                      | Type      | Description                                                         |
|--------------------------------------------------|-----------|---------------------------------------------------------------------|
| release_concurrent_total                         | Gauge     | Total number of concurrent release attempts.                        |
| release_concurrent_post_actions_executions_total | Gauge     | Total number of concurrent release post actions executions attempts |
| release_concurrent_processings_total             | Gauge     | Total number of concurrent release processing attempts.             |
| release_duration_seconds                         | Histogram | How long in seconds a Release takes to complete.                    |
| release_mitigation_success_total                 | Counter   | Total number of successful release retry mitigations.               |
| release_post_actions_execution_duration_seconds  | Histogram | How long in seconds Release post-actions take to complete.          |
| release_processing_duration_seconds              | Histogram | How long in seconds a Release processing takes to complete.         |
| release_pre_processing_duration_seconds          | Histogram | How long in seconds a Release takes to start processing             |
| release_validation_duration_seconds              | Histogram | How long in seconds a Release takes to validate                     |
| release_total                                    | Counter   | Total number of releases reconciled by the operator.                |
