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
$ TAG_NAME=my-tag make docker-build docker-push  # Custom tag
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

## Distributed tracing

The operator emits OpenTelemetry spans for each release PipelineRun (tenant-collectors, managed-collectors, tenant, managed, final) on completion. Tracing is enabled by pointing the controller at an OTLP/gRPC collector; when the endpoint env var is unset, the operator installs a noop tracer provider and emits nothing.

### Configuration

| Env var | Purpose | Default |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP/gRPC collector URL. Unset disables tracing. `http://` sends spans in plaintext; `https://` uses TLS. | *(unset)* |
| `OTEL_TRACES_SAMPLER` | `always_on`, `always_off`, `traceidratio`, `parentbased_always_off`, `parentbased_traceidratio`. | `parentbased_always_on` |
| `OTEL_TRACES_SAMPLER_ARG` | Ratio for ratio-based samplers (e.g. `0.1`). | *(unused unless a ratio sampler is selected)* |
| `TRACING_LABEL_ACTION` | Label key read from the `PipelineRun` or `Release` to populate `cicd.pipeline.action.name`. Empty string disables the attribute. | `delivery.tekton.dev/action` |
| `TRACING_LABEL_APPLICATION` | Label key read from the `PipelineRun` or `Release` to populate `delivery.tekton.dev.application`. Empty string disables the attribute. | `delivery.tekton.dev/application` |
| `TRACING_LABEL_COMPONENT` | Label key read from the `PipelineRun` or `Release` to populate `delivery.tekton.dev.component`. Empty string disables the attribute. | `delivery.tekton.dev/component` |

### Emitted spans

Per completed PipelineRun:

- `waitDuration`: `pr.CreationTimestamp` to `pr.Status.StartTime`
- `executeDuration`: `pr.Status.StartTime` to `pr.Status.CompletionTime`

When a Release is rejected at validation and no PipelineRun is created, a single
`waitDuration` span is emitted instead, spanning the Release's `CreationTimestamp`
through the Validated condition's `LastTransitionTime` and carrying
`cicd.pipeline.result=error` so the rejection still surfaces in the trace.

Parenting follows the W3C Trace Context in the `tekton.dev/pipelinerunSpanContext`
annotation. The annotation is copied forward from the Release CR onto each
PipelineRun the controller creates, so all release-stage spans land under a
single trace when a parent context is present upstream.

### Span attributes

Emitted spans may carry failure messages from PipelineRun and TaskRun conditions (`delivery.tekton.dev.result_message`). These can echo task parameters or step output; treat the OTLP collector at the same access-control level as controller logs.

Span column values: `wait` = PipelineRun `waitDuration`, `execute` = PipelineRun `executeDuration`, `validation wait` = the waitDuration emitted on validation rejection.

| Attribute | Span | Source |
|---|---|---|
| `namespace` | all | `PipelineRun` namespace (wait, execute); `Release` namespace (validation wait) |
| `pipelinerun` | wait, execute | `PipelineRun` name |
| `delivery.tekton.dev.pipelinerun_uid` | wait, execute | `PipelineRun` UID |
| `release` | validation wait | `Release` name |
| `cicd.pipeline.action.name` | all | Label on `PipelineRun` (wait, execute) or `Release` (validation wait); label key configurable via `TRACING_LABEL_ACTION` |
| `delivery.tekton.dev.application` | all | Label on `PipelineRun` (wait, execute) or `Release` (validation wait); label key configurable via `TRACING_LABEL_APPLICATION` |
| `delivery.tekton.dev.component` | all | Label on `PipelineRun` (wait, execute) or `Release` (validation wait); label key configurable via `TRACING_LABEL_COMPONENT` |
| `cicd.pipeline.result` | execute, validation wait | execute: `Succeeded` condition mapped to the semconv `cicd.pipeline.result` enum (`success` / `failure` / `timeout` / `cancellation` / `error`). validation wait: always `error`. |
| `delivery.tekton.dev.result_message` | execute, validation wait | execute: earliest failing TaskRun's `Succeeded` condition message, falling back to the PipelineRun's own condition message. validation wait: the Validated condition's message. Truncated to 1024 bytes (UTF-8 safe); omitted when empty. |
