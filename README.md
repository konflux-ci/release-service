# Konflux Release service

Release service is a Kubernetes operator to control the life cycle of Konflux-managed releases.

## Running, building and testing the operator

This operator provides a [Makefile](Makefile) to run all the usual development tasks. This file can be used by cloning
the repository and running `make` over any of the provided targets.

### Running the operator locally

When testing locally (eg. a CRC cluster), the command `make run install` can be used to deploy and run the operator. 
If any change has been done in the code, `make manifests generate` should be executed before to generate the new resources
and build the operator.

### Build and push a new image

To build the operator and push a new image to the registry, the following commands can be used: 

```shell
$ make docker-build
$ make docker-push
```

These commands will use the default image and tag. To modify them, new values for `TAG` and `IMG` environment variables
can be passed. For example, to override the tag:

```shell
$ TAG=my-tag make docker-build
$ TAG=my-tag make docker-push
```

Or, in the case the image should be pushed to a different repository:

```shell
$ IMG=quay.io/user/release:my-tag make docker-build
$ IMG=quay.io/user/release:my-tag make docker-push
```

### Running tests

To test the code, simply run `make test`. This command will fetch all the required dependencies and test the code. The
test coverage will be reported at the end, once all the tests have been executed.

## Disabling Webhooks for local development

Webhooks require self-signed certificates to validate the resources. To disable webhooks during local development and
testing, export the `ENABLE_WEBHOOKS` variable setting its value to `false` or prepend it while running the operator
using the following command:

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
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP/gRPC collector URL. Unset disables tracing. | *(unset)* |
| `OTEL_TRACES_SAMPLER` | `always_on`, `always_off`, `traceidratio`, `parentbased_always_off`, `parentbased_traceidratio`. | `parentbased_always_on` |
| `OTEL_TRACES_SAMPLER_ARG` | Ratio for ratio-based samplers (e.g. `0.1`). | *(unused unless a ratio sampler is selected)* |
| `TRACING_LABEL_ACTION` | PipelineRun label read to populate `cicd.pipeline.action.name`. Empty string disables the attribute. | `delivery.tekton.dev/action` |
| `TRACING_LABEL_APPLICATION` | PipelineRun label read to populate `delivery.tekton.dev.application`. Empty string disables the attribute. | `delivery.tekton.dev/application` |
| `TRACING_LABEL_COMPONENT` | PipelineRun label read to populate `delivery.tekton.dev.component`. Empty string disables the attribute. | `delivery.tekton.dev/component` |

### Emitted spans

Two spans per PipelineRun:

- `waitDuration` — `pr.CreationTimestamp` → `pr.Status.StartTime`
- `executeDuration` — `pr.Status.StartTime` → `pr.Status.CompletionTime`

Parenting follows the W3C Trace Context in the `tekton.dev/pipelinerunSpanContext` annotation. The annotation is copied forward from the Release CR onto each PipelineRun the controller creates, so all release-stage spans land under a single trace when a parent context is present upstream. The `tekton.dev/timingEmitted` annotation guards against re-emission on repeat reconciles.

### Span attributes

| Attribute | Span | Source |
|---|---|---|
| `namespace` | both | `pr.GetNamespace()` |
| `pipelinerun` | both | `pr.GetName()` |
| `delivery.tekton.dev.pipelinerun_uid` | both | `pr.GetUID()` |
| `cicd.pipeline.action.name` | both | PipelineRun label (name configurable via `TRACING_LABEL_ACTION`) |
| `delivery.tekton.dev.application` | both | PipelineRun label (name configurable via `TRACING_LABEL_APPLICATION`) |
| `delivery.tekton.dev.component` | both | PipelineRun label (name configurable via `TRACING_LABEL_COMPONENT`) |
| `cicd.pipeline.result` | execute | `Succeeded` condition mapped to the semconv `cicd.pipeline.result` enum (`success` / `failure` / `timeout` / `cancellation` / `error`) |
| `delivery.tekton.dev.result_message` | execute | Earliest failing TaskRun's `Succeeded` condition message, falling back to the PipelineRun's own condition message. Omitted on success; truncated to 1024 bytes (UTF-8 safe). |
