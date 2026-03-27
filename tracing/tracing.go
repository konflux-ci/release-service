/*
Copyright 2026 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tracing

import (
	"context"
	"os"
	"strconv"
	"unicode/utf8"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TracerName      = "release-service"
	EnvOTLPEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"

	EnvTracesSampler    = "OTEL_TRACES_SAMPLER"
	EnvTracesSamplerArg = "OTEL_TRACES_SAMPLER_ARG"

	EnvTracingLabelAction      = "TRACING_LABEL_ACTION"
	EnvTracingLabelApplication = "TRACING_LABEL_APPLICATION"
	EnvTracingLabelComponent   = "TRACING_LABEL_COMPONENT"
)

const AttrNamespace = "delivery.tekton.dev"

const (
	SpanWaitDuration    = "waitDuration"
	SpanExecuteDuration = "executeDuration"
)

// MaxResultMessageLen caps delivery.tekton.dev.result_message at 1024 bytes so the
// attribute stays within typical tracing-backend tag-value limits.
const MaxResultMessageLen = 1024

const TruncatedSuffix = "...[truncated]"

var (
	DefaultTracingLabelAction      = AttrNamespace + "/action"
	DefaultTracingLabelApplication = AttrNamespace + "/application"
	DefaultTracingLabelComponent   = AttrNamespace + "/component"
)

var (
	NamespaceKey   = attribute.Key("namespace")
	PipelineRunKey = attribute.Key("pipelinerun")

	DeliveryPipelineRunUIDKey = attribute.Key(AttrNamespace + ".pipelinerun_uid")
	DeliveryApplicationKey    = attribute.Key(AttrNamespace + ".application")
	DeliveryComponentKey      = attribute.Key(AttrNamespace + ".component")
	DeliveryResultMessageKey  = attribute.Key(AttrNamespace + ".result_message")
)

// ResultEnum returns the cicd.pipeline.result span attribute matching the PipelineRun's terminal condition.
func ResultEnum(cond *apis.Condition) attribute.KeyValue {
	if cond.Status == corev1.ConditionTrue {
		return semconv.CICDPipelineResultSuccess
	}
	switch cond.Reason {
	case tektonv1.PipelineRunReasonCancelled.String(),
		tektonv1.PipelineRunReasonCancelledRunningFinally.String():
		return semconv.CICDPipelineResultCancellation
	case tektonv1.PipelineRunReasonTimedOut.String():
		return semconv.CICDPipelineResultTimeout
	case tektonv1.PipelineRunReasonFailed.String():
		return semconv.CICDPipelineResultFailure
	}
	return semconv.CICDPipelineResultError
}

// TruncateResultMessage caps msg at MaxResultMessageLen to fit within span attribute size limits.
func TruncateResultMessage(msg string) string {
	if len(msg) <= MaxResultMessageLen {
		return msg
	}
	keep := MaxResultMessageLen - len(TruncatedSuffix)
	if keep < 0 {
		keep = 0
	}
	head := msg[:keep]
	for len(head) > 0 {
		r, size := utf8.DecodeLastRuneInString(head)
		if r != utf8.RuneError || size > 1 {
			break
		}
		head = head[:len(head)-size]
	}
	return head + TruncatedSuffix
}

type LabelNames struct {
	Action      string
	Application string
	Component   string
}

var labelNames LabelNames

// SetupLabelNames populates the cached label config from env.
func SetupLabelNames() {
	labelNames = LabelNames{
		Action:      envOr(EnvTracingLabelAction, DefaultTracingLabelAction),
		Application: envOr(EnvTracingLabelApplication, DefaultTracingLabelApplication),
		Component:   envOr(EnvTracingLabelComponent, DefaultTracingLabelComponent),
	}
}

// LoadLabelNames returns the cached label config populated by SetupLabelNames.
func LoadLabelNames() LabelNames { return labelNames }

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func samplerFromEnv() sdktrace.Sampler {
	name := os.Getenv(EnvTracesSampler)
	argStr := os.Getenv(EnvTracesSamplerArg)
	arg, err := strconv.ParseFloat(argStr, 64)
	if err != nil && argStr != "" {
		setupLog.Error(err, "ignoring malformed sampler argument", "env", EnvTracesSamplerArg, "value", argStr)
	}
	if argStr == "" && (name == "traceidratio" || name == "parentbased_traceidratio") {
		setupLog.Info("ratio sampler selected without "+EnvTracesSamplerArg+"; defaulting to 0% sampling", "env", EnvTracesSampler, "value", name)
	}
	switch name {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(arg)
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(arg))
	}
	return sdktrace.ParentBased(sdktrace.AlwaysSample())
}

// EarliestFailingTaskRunMessage returns the failure message from the earliest-completed failing TaskRun of a PipelineRun.
func EarliestFailingTaskRunMessage(ctx context.Context, c client.Client, pr *tektonv1.PipelineRun) string {
	trList := &tektonv1.TaskRunList{}
	if err := c.List(ctx, trList,
		client.InNamespace(pr.Namespace),
		client.MatchingLabels{"tekton.dev/pipelineRun": pr.Name}); err != nil {
		return ""
	}
	var (
		earliestTime *metav1.Time
		earliestMsg  string
	)
	for i := range trList.Items {
		tr := &trList.Items[i]
		if tr.Status.CompletionTime == nil {
			continue
		}
		cond := tr.Status.GetCondition(apis.ConditionSucceeded)
		if cond == nil || cond.Status != corev1.ConditionFalse {
			continue
		}
		if earliestTime == nil || tr.Status.CompletionTime.Before(earliestTime) {
			earliestTime = tr.Status.CompletionTime
			earliestMsg = cond.Message
		}
	}
	return earliestMsg
}

var setupLog = ctrl.Log.WithName("tracing")

type TracerProvider struct {
	shutdown func(context.Context) error
}

// New configures the OTel SDK from environment and installs it as the global tracer provider.
func New() *TracerProvider {
	if os.Getenv(EnvOTLPEndpoint) == "" {
		setupLog.Info("OTLP endpoint not configured, using noop tracer provider")
		return &TracerProvider{shutdown: func(context.Context) error { return nil }}
	}

	exporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithEndpointURL(os.Getenv(EnvOTLPEndpoint)),
	)
	if err != nil {
		setupLog.Error(err, "failed to create OTLP exporter, using noop tracer provider")
		return &TracerProvider{shutdown: func(context.Context) error { return nil }}
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(TracerName),
		),
	)
	if err != nil {
		setupLog.Error(err, "failed to create resource")
		res = resource.Default()
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(samplerFromEnv()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	setupLog.Info("tracing initialized", "endpoint", os.Getenv(EnvOTLPEndpoint))

	return &TracerProvider{shutdown: tp.Shutdown}
}

// Shutdown flushes pending spans before process exit.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.shutdown != nil {
		return tp.shutdown(ctx)
	}
	return nil
}
