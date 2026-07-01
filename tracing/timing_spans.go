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
	"encoding/json"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// CtxFromSpanContext extracts the parent span context from a PipelineRun annotation.
func CtxFromSpanContext(jsonCarrier string) (context.Context, bool) {
	if jsonCarrier == "" {
		return context.Background(), false
	}
	var carrier map[string]string
	if err := json.Unmarshal([]byte(jsonCarrier), &carrier); err != nil {
		setupLog.Info("ignoring malformed span context annotation", "error", err)
		return context.Background(), false
	}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.MapCarrier(carrier))
	sc := trace.SpanContextFromContext(ctx)
	return ctx, sc.IsValid()
}

// setCommonAttributes sets the span attributes that identify a PipelineRun.
func setCommonAttributes(span trace.Span, pr *tektonv1.PipelineRun, labels LabelNames) {
	span.SetAttributes(
		NamespaceKey.String(pr.Namespace),
		PipelineRunKey.String(pr.Name),
		DeliveryPipelineRunUIDKey.String(string(pr.UID)),
	)
	prLabels := pr.GetLabels()
	for _, m := range []struct {
		labelName string
		key       attribute.Key
	}{
		{labels.Action, semconv.CICDPipelineActionNameKey},
		{labels.Application, DeliveryApplicationKey},
		{labels.Component, DeliveryComponentKey},
	} {
		if m.labelName == "" {
			continue
		}
		if v := prLabels[m.labelName]; v != "" {
			span.SetAttributes(m.key.String(v))
		}
	}
}

// setOutcomeAttributes records the PipelineRun's terminal status on a timing span.
func setOutcomeAttributes(span trace.Span, pr *tektonv1.PipelineRun, failureMsg string) {
	cond := pr.Status.GetCondition(apis.ConditionSucceeded)
	if cond == nil {
		return
	}
	span.SetAttributes(ResultEnum(cond))
	if cond.Status == corev1.ConditionFalse {
		msg := failureMsg
		if msg == "" {
			msg = cond.Message
		}
		if msg != "" {
			span.SetAttributes(DeliveryResultMessageKey.String(TruncateResultMessage(msg)))
		}
	}
}

// EmitWaitDuration records the time between a PipelineRun's creation and start.
func EmitWaitDuration(ctx context.Context, pr *tektonv1.PipelineRun, labels LabelNames) {
	if pr.Status.StartTime == nil {
		return
	}
	start := pr.CreationTimestamp.Time
	end := pr.Status.StartTime.Time
	if end.Before(start) {
		return
	}

	tr := otel.Tracer(TracerName)
	_, span := tr.Start(ctx, SpanWaitDuration, trace.WithTimestamp(start))

	setCommonAttributes(span, pr, labels)
	span.End(trace.WithTimestamp(end))
}

// EmitExecuteDuration records the time between a PipelineRun's start and completion, plus its result and failure message.
func EmitExecuteDuration(ctx context.Context, pr *tektonv1.PipelineRun, labels LabelNames, failureMsg string) {
	if pr.Status.StartTime == nil || pr.Status.CompletionTime == nil {
		return
	}
	start := pr.Status.StartTime.Time
	end := pr.Status.CompletionTime.Time
	if end.Before(start) {
		return
	}

	tr := otel.Tracer(TracerName)
	_, span := tr.Start(ctx, SpanExecuteDuration, trace.WithTimestamp(start))

	setCommonAttributes(span, pr, labels)
	setOutcomeAttributes(span, pr, failureMsg)
	span.End(trace.WithTimestamp(end))
}

// EmitReleaseValidationFailureWait records the time between a Release's creation and validation rejection, plus its result and failure message.
func EmitReleaseValidationFailureWait(
	parentCtx context.Context,
	releaseName, releaseNamespace string,
	releaseLabels map[string]string,
	createdAt, validatedAt time.Time,
	validationMessage string,
	labels LabelNames,
) bool {
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		return false
	}
	if validatedAt.Before(createdAt) {
		return false
	}

	tr := otel.Tracer(TracerName)
	_, span := tr.Start(parentCtx, SpanWaitDuration, trace.WithTimestamp(createdAt))

	span.SetAttributes(
		NamespaceKey.String(releaseNamespace),
		ReleaseKey.String(releaseName),
	)
	for _, m := range []struct {
		labelName string
		key       attribute.Key
	}{
		{labels.Action, semconv.CICDPipelineActionNameKey},
		{labels.Application, DeliveryApplicationKey},
		{labels.Component, DeliveryComponentKey},
	} {
		if m.labelName == "" {
			continue
		}
		if v := releaseLabels[m.labelName]; v != "" {
			span.SetAttributes(m.key.String(v))
		}
	}

	span.SetAttributes(semconv.CICDPipelineResultError)
	if validationMessage != "" {
		span.SetAttributes(DeliveryResultMessageKey.String(TruncateResultMessage(validationMessage)))
	}

	span.End(trace.WithTimestamp(validatedAt))
	return true
}

// EmitTimingSpans emits the timing spans for a PipelineRun.
func EmitTimingSpans(ctx context.Context, c client.Client, pr *tektonv1.PipelineRun, labels LabelNames, spanContext string) bool {
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		return false
	}

	parentCtx, ok := CtxFromSpanContext(spanContext)
	if !ok {
		parentCtx = context.Background()
	}

	if pr.Status.StartTime == nil || pr.Status.CompletionTime == nil {
		return false
	}

	EmitWaitDuration(parentCtx, pr, labels)
	EmitExecuteDuration(parentCtx, pr, labels, resolveFailureMessage(ctx, c, pr))

	return true
}

// resolveFailureMessage returns the failure detail for a failed PipelineRun.
func resolveFailureMessage(ctx context.Context, c client.Client, pr *tektonv1.PipelineRun) string {
	cond := pr.Status.GetCondition(apis.ConditionSucceeded)
	if cond == nil || cond.Status != corev1.ConditionFalse {
		return ""
	}
	if msg := EarliestFailingTaskRunMessage(ctx, c, pr); msg != "" {
		return msg
	}
	return cond.Message
}
