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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func makeCompletedPR(status corev1.ConditionStatus, reason, message string) *tektonv1.PipelineRun {
	start := time.Now().Add(-time.Minute)
	completion := time.Now()
	return &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rel-pr",
			Namespace:         "trace-demo-managed",
			CreationTimestamp: metav1.NewTime(start.Add(-30 * time.Second)),
		},
		Status: tektonv1.PipelineRunStatus{
			PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
				StartTime:      &metav1.Time{Time: start},
				CompletionTime: &metav1.Time{Time: completion},
			},
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:    apis.ConditionSucceeded,
					Status:  status,
					Reason:  reason,
					Message: message,
				}},
			},
		},
	}
}

var _ = Describe("EmitWaitDuration", func() {
	It("emits identity and label-driven attributes", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		creation := time.Now().Add(-time.Minute)
		start := time.Now()
		pr := &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "rel-pr",
				Namespace:         "trace-demo-managed",
				UID:               types.UID("uid-abc"),
				CreationTimestamp: metav1.NewTime(creation),
				Labels: map[string]string{
					DefaultTracingLabelAction:      "release",
					DefaultTracingLabelApplication: "my-app",
					DefaultTracingLabelComponent:   "my-comp",
				},
			},
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: start},
				},
			},
		}
		EmitWaitDuration(context.Background(), pr, defaultLabels())

		spans := exp.GetSpans()
		Expect(spans).To(HaveLen(1))
		s := spans[0]
		Expect(spanAttr(s, string(NamespaceKey))).To(Equal("trace-demo-managed"))
		Expect(spanAttr(s, string(PipelineRunKey))).To(Equal("rel-pr"))
		Expect(spanAttr(s, string(DeliveryPipelineRunUIDKey))).To(Equal("uid-abc"))
		Expect(spanAttr(s, string(semconv.CICDPipelineActionNameKey))).To(Equal("release"))
		Expect(spanAttr(s, string(DeliveryApplicationKey))).To(Equal("my-app"))
		Expect(spanAttr(s, string(DeliveryComponentKey))).To(Equal("my-comp"))
	})

	It("omits label-driven attributes when the labels are absent on the PipelineRun", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		pr := &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute))},
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: time.Now()},
				},
			},
		}
		EmitWaitDuration(context.Background(), pr, defaultLabels())

		s := exp.GetSpans()[0]
		Expect(hasAttr(s, string(semconv.CICDPipelineActionNameKey))).To(BeFalse())
		Expect(hasAttr(s, string(DeliveryApplicationKey))).To(BeFalse())
		Expect(hasAttr(s, string(DeliveryComponentKey))).To(BeFalse())
	})

	It("treats an empty configured label name as disabling that attribute", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		partial := LabelNames{
			Action:      "",
			Application: DefaultTracingLabelApplication,
			Component:   DefaultTracingLabelComponent,
		}
		pr := &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute)),
				Labels: map[string]string{
					DefaultTracingLabelAction:      "should-not-be-emitted",
					DefaultTracingLabelApplication: "present-app",
				},
			},
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: time.Now()},
				},
			},
		}
		EmitWaitDuration(context.Background(), pr, partial)

		s := exp.GetSpans()[0]
		Expect(hasAttr(s, string(semconv.CICDPipelineActionNameKey))).To(BeFalse())
		Expect(spanAttr(s, string(DeliveryApplicationKey))).To(Equal("present-app"))
	})
})

var _ = Describe("EmitExecuteDuration outcome attributes", func() {
	DescribeTable("maps the Succeeded condition to cicd.pipeline.result and result_message",
		func(status corev1.ConditionStatus, reason, message, failureMsg, wantResult, wantMessage string, expectMsg bool) {
			exp, tp := newTracerProvider()
			DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

			pr := makeCompletedPR(status, reason, message)
			EmitExecuteDuration(context.Background(), pr, defaultLabels(), failureMsg)

			s := exp.GetSpans()[0]
			Expect(spanAttr(s, string(semconv.CICDPipelineResultKey))).To(Equal(wantResult))
			if expectMsg {
				Expect(spanAttr(s, string(DeliveryResultMessageKey))).To(Equal(wantMessage))
			} else {
				Expect(hasAttr(s, string(DeliveryResultMessageKey))).To(BeFalse())
			}
		},
		Entry("success omits result_message",
			corev1.ConditionTrue, tektonv1.PipelineRunReasonSuccessful.String(),
			"All steps completed", "",
			semconv.CICDPipelineResultSuccess.Value.AsString(), "", false),
		Entry("failure with walker-supplied message",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonFailed.String(),
			"pr-level fallback", "failing-taskrun exit 1",
			semconv.CICDPipelineResultFailure.Value.AsString(), "failing-taskrun exit 1", true),
		Entry("failure without a walker message falls back to the PipelineRun condition message",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonFailedValidation.String(),
			"validation failed", "",
			semconv.CICDPipelineResultError.Value.AsString(), "validation failed", true),
		Entry("timeout maps correctly",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonTimedOut.String(), "", "",
			semconv.CICDPipelineResultTimeout.Value.AsString(), "", false),
		Entry("cancelled maps correctly",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonCancelled.String(), "", "",
			semconv.CICDPipelineResultCancellation.Value.AsString(), "", false),
	)

	It("truncates an overlong failure message", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		pr := makeCompletedPR(corev1.ConditionFalse, tektonv1.PipelineRunReasonFailed.String(), "")
		long := strings.Repeat("x", MaxResultMessageLen*2)
		EmitExecuteDuration(context.Background(), pr, defaultLabels(), long)

		got := spanAttr(exp.GetSpans()[0], string(DeliveryResultMessageKey))
		Expect(len(got)).To(BeNumerically("<=", MaxResultMessageLen))
		Expect(got).To(HaveSuffix(TruncatedSuffix))
	})

	It("emits no result attribute when the Succeeded condition is missing", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		pr := makeCompletedPR(corev1.ConditionTrue, "", "")
		pr.Status.Conditions = duckv1.Conditions{}
		EmitExecuteDuration(context.Background(), pr, defaultLabels(), "")

		s := exp.GetSpans()[0]
		Expect(hasAttr(s, string(semconv.CICDPipelineResultKey))).To(BeFalse())
		Expect(hasAttr(s, string(DeliveryResultMessageKey))).To(BeFalse())
	})
})

var _ = Describe("EmitWaitDuration negative paths", func() {
	It("emits no span when StartTime is unset", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		pr := &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Minute))},
		}
		EmitWaitDuration(context.Background(), pr, defaultLabels())
		Expect(exp.GetSpans()).To(BeEmpty())
	})

	It("emits no span when StartTime is before CreationTimestamp (clock skew)", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		now := time.Now()
		pr := &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now)},
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-time.Second)},
				},
			},
		}
		EmitWaitDuration(context.Background(), pr, defaultLabels())
		Expect(exp.GetSpans()).To(BeEmpty())
	})
})

var _ = Describe("EmitExecuteDuration negative paths", func() {
	It("emits no span when StartTime is unset", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		now := time.Now()
		pr := &tektonv1.PipelineRun{
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					CompletionTime: &metav1.Time{Time: now},
				},
			},
		}
		EmitExecuteDuration(context.Background(), pr, defaultLabels(), "")
		Expect(exp.GetSpans()).To(BeEmpty())
	})

	It("emits no span when CompletionTime is unset", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		now := time.Now()
		pr := &tektonv1.PipelineRun{
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-time.Minute)},
				},
			},
		}
		EmitExecuteDuration(context.Background(), pr, defaultLabels(), "")
		Expect(exp.GetSpans()).To(BeEmpty())
	})

	It("emits no span when CompletionTime is before StartTime (clock skew)", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		now := time.Now()
		pr := &tektonv1.PipelineRun{
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					StartTime:      &metav1.Time{Time: now},
					CompletionTime: &metav1.Time{Time: now.Add(-time.Second)},
				},
			},
		}
		EmitExecuteDuration(context.Background(), pr, defaultLabels(), "")
		Expect(exp.GetSpans()).To(BeEmpty())
	})
})

var _ = Describe("CtxFromSpanContext", func() {
	It("returns a fresh Background context with ok=false for an empty carrier", func() {
		ctx, ok := CtxFromSpanContext("")
		Expect(ok).To(BeFalse())
		Expect(trace.SpanContextFromContext(ctx).IsValid()).To(BeFalse())
	})

	It("returns Background with ok=false when the carrier JSON is malformed", func() {
		ctx, ok := CtxFromSpanContext("{not-json")
		Expect(ok).To(BeFalse())
		Expect(trace.SpanContextFromContext(ctx).IsValid()).To(BeFalse())
	})

	It("returns ok=false when the JSON is valid but carries no traceparent", func() {
		ctx, ok := CtxFromSpanContext(`{"unrelated":"value"}`)
		Expect(ok).To(BeFalse())
		Expect(trace.SpanContextFromContext(ctx).IsValid()).To(BeFalse())
	})

	It("round-trips a real W3C trace context with the same TraceID and SpanID", func() {
		_, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "parent")
		defer parentSpan.End()
		want := parentSpan.SpanContext()

		carrier := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(parentCtx, carrier)
		encoded, err := json.Marshal(map[string]string(carrier))
		Expect(err).NotTo(HaveOccurred())

		got, ok := CtxFromSpanContext(string(encoded))
		Expect(ok).To(BeTrue())
		gotSC := trace.SpanContextFromContext(got)
		Expect(gotSC.TraceID()).To(Equal(want.TraceID()))
		Expect(gotSC.SpanID()).To(Equal(want.SpanID()))
		Expect(gotSC.IsRemote()).To(BeTrue())
	})
})

var _ = Describe("resolveFailureMessage", func() {
	now := time.Now()
	mkPR := func(status corev1.ConditionStatus, message string, children ...string) *tektonv1.PipelineRun {
		pr := &tektonv1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "rel-pr", Namespace: "ns"}}
		pr.Status.Conditions = duckv1.Conditions{{
			Type:    apis.ConditionSucceeded,
			Status:  status,
			Message: message,
		}}
		for _, n := range children {
			pr.Status.ChildReferences = append(pr.Status.ChildReferences, tektonv1.ChildStatusReference{Name: n})
		}
		return pr
	}
	mkFailingTR := func(name, message string) *tektonv1.TaskRun {
		tr := &tektonv1.TaskRun{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}}
		tr.Status.CompletionTime = &metav1.Time{Time: now}
		tr.Status.Conditions = duckv1.Conditions{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Message: message,
		}}
		return tr
	}

	It("returns empty when the Succeeded condition is missing", func() {
		pr := &tektonv1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Namespace: "ns"}}
		Expect(resolveFailureMessage(context.Background(), newFakeClient(), pr)).To(Equal(""))
	})

	It("returns empty when the PipelineRun succeeded", func() {
		pr := mkPR(corev1.ConditionTrue, "all good")
		Expect(resolveFailureMessage(context.Background(), newFakeClient(), pr)).To(Equal(""))
	})

	It("prefers the failing TaskRun message over the PipelineRun condition message", func() {
		pr := mkPR(corev1.ConditionFalse, "pr-level fallback", "tr-fail")
		c := newFakeClient(mkFailingTR("tr-fail", "taskrun-level reason"))
		Expect(resolveFailureMessage(context.Background(), c, pr)).To(Equal("taskrun-level reason"))
	})

	It("falls back to the PipelineRun condition message when no TaskRun message is available", func() {
		pr := mkPR(corev1.ConditionFalse, "pr-level only", "tr-missing")
		Expect(resolveFailureMessage(context.Background(), newFakeClient(), pr)).To(Equal("pr-level only"))
	})
})

var _ = Describe("EmitTimingSpans", func() {
	makeRunningPR := func() *tektonv1.PipelineRun {
		creation := time.Now().Add(-2 * time.Minute)
		start := time.Now().Add(-time.Minute)
		return &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "rel-pr",
				Namespace:         "ns",
				UID:               types.UID("uid-running"),
				CreationTimestamp: metav1.NewTime(creation),
			},
			Status: tektonv1.PipelineRunStatus{
				PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: start},
				},
			},
		}
	}

	It("returns false and emits nothing when the global TracerProvider is the noop", func() {
		prev := otel.GetTracerProvider()
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		DeferCleanup(func() { otel.SetTracerProvider(prev) })

		ok := EmitTimingSpans(context.Background(), newFakeClient(), makeCompletedPR(corev1.ConditionTrue, "", ""), defaultLabels(), "")
		Expect(ok).To(BeFalse())
	})

	It("returns false when the PipelineRun has not started yet", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		pr := &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "rel-pr",
				Namespace:         "ns",
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
		}
		Expect(EmitTimingSpans(context.Background(), newFakeClient(), pr, defaultLabels(), "")).To(BeFalse())
		Expect(exp.GetSpans()).To(BeEmpty())
	})

	It("emits only waitDuration while the PipelineRun is still running", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		Expect(EmitTimingSpans(context.Background(), newFakeClient(), makeRunningPR(), defaultLabels(), "")).To(BeTrue())

		spans := exp.GetSpans()
		Expect(spans).To(HaveLen(1))
		Expect(spans[0].Name()).To(Equal(SpanWaitDuration))
	})

	It("emits both waitDuration and executeDuration when the PipelineRun has completed", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		pr := makeCompletedPR(corev1.ConditionTrue, tektonv1.PipelineRunReasonSuccessful.String(), "ok")
		Expect(EmitTimingSpans(context.Background(), newFakeClient(), pr, defaultLabels(), "")).To(BeTrue())

		names := []string{}
		for _, s := range exp.GetSpans() {
			names = append(names, s.Name())
		}
		Expect(names).To(ConsistOf(SpanWaitDuration, SpanExecuteDuration))
	})

	It("parents the emitted spans under a valid injected trace context", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		parentCtx, parentSpan := otel.Tracer("upstream").Start(context.Background(), "parent")
		want := parentSpan.SpanContext().TraceID()
		parentSpan.End()

		carrier := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(parentCtx, carrier)
		encoded, err := json.Marshal(map[string]string(carrier))
		Expect(err).NotTo(HaveOccurred())

		pr := makeCompletedPR(corev1.ConditionTrue, tektonv1.PipelineRunReasonSuccessful.String(), "ok")
		Expect(EmitTimingSpans(context.Background(), newFakeClient(), pr, defaultLabels(), string(encoded))).To(BeTrue())

		emitted := []string{}
		for _, s := range exp.GetSpans() {
			if s.Name() == SpanWaitDuration || s.Name() == SpanExecuteDuration {
				emitted = append(emitted, s.Name())
				Expect(s.Parent().TraceID()).To(Equal(want),
					"timing span %q should be parented under the injected trace", s.Name())
			}
		}
		Expect(emitted).To(ConsistOf(SpanWaitDuration, SpanExecuteDuration))
	})

	It("falls back to a root trace when the carrier annotation is malformed", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		pr := makeCompletedPR(corev1.ConditionTrue, tektonv1.PipelineRunReasonSuccessful.String(), "ok")
		Expect(EmitTimingSpans(context.Background(), newFakeClient(), pr, defaultLabels(), "{not-json")).To(BeTrue())

		for _, s := range exp.GetSpans() {
			Expect(s.Parent().IsValid()).To(BeFalse(),
				"span %q should be a root span when the injected carrier is unusable", s.Name())
		}
	})

	It("threads the failing-TaskRun message into executeDuration's result_message", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		pr := makeCompletedPR(corev1.ConditionFalse, tektonv1.PipelineRunReasonFailed.String(), "pr-level fallback")
		pr.Status.ChildReferences = []tektonv1.ChildStatusReference{{Name: "tr-fail"}}

		failingTR := &tektonv1.TaskRun{ObjectMeta: metav1.ObjectMeta{Name: "tr-fail", Namespace: pr.Namespace}}
		failingTR.Status.CompletionTime = pr.Status.CompletionTime
		failingTR.Status.Conditions = duckv1.Conditions{{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Message: "taskrun-level reason",
		}}

		Expect(EmitTimingSpans(context.Background(), newFakeClient(failingTR), pr, defaultLabels(), "")).To(BeTrue())

		var execute trace.SpanContext
		var resultMessage string
		for _, s := range exp.GetSpans() {
			if s.Name() == SpanExecuteDuration {
				execute = s.SpanContext()
				resultMessage = spanAttr(s, string(DeliveryResultMessageKey))
			}
		}
		Expect(execute.IsValid()).To(BeTrue(), "executeDuration span should have been emitted")
		Expect(resultMessage).To(Equal("taskrun-level reason"))
	})
})

var _ = Describe("EmitReleaseValidationFailureWait", func() {
	var (
		createdAt   = time.Now().Add(-time.Minute)
		validatedAt = time.Now()
		labels      = defaultLabels()
	)

	It("returns false and emits nothing when the global TracerProvider is the noop", func() {
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		Expect(EmitReleaseValidationFailureWait(
			context.Background(),
			"rel", "ns", nil,
			createdAt, validatedAt,
			"validation failed",
			labels,
		)).To(BeFalse())
	})

	It("returns false when validatedAt precedes createdAt", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		Expect(EmitReleaseValidationFailureWait(
			context.Background(),
			"rel", "ns", nil,
			validatedAt, createdAt,
			"validation failed",
			labels,
		)).To(BeFalse())
		Expect(exp.GetSpans()).To(BeEmpty())
	})

	It("emits a waitDuration span carrying cicd.pipeline.result=error and result_message", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		Expect(EmitReleaseValidationFailureWait(
			context.Background(),
			"rel-pr", "trace-demo-managed",
			map[string]string{
				DefaultTracingLabelApplication: "my-app",
				DefaultTracingLabelComponent:   "my-comp",
			},
			createdAt, validatedAt,
			"missing required author label",
			labels,
		)).To(BeTrue())

		spans := exp.GetSpans()
		Expect(spans).To(HaveLen(1))
		s := spans[0]
		Expect(s.Name()).To(Equal(SpanWaitDuration))
		Expect(s.StartTime()).To(BeTemporally("~", createdAt, time.Second))
		Expect(s.EndTime()).To(BeTemporally("~", validatedAt, time.Second))
		Expect(spanAttr(s, string(NamespaceKey))).To(Equal("trace-demo-managed"))
		Expect(spanAttr(s, string(PipelineRunKey))).To(Equal("rel-pr"))
		Expect(spanAttr(s, string(semconv.CICDPipelineResultKey))).To(Equal(semconv.CICDPipelineResultError.Value.AsString()))
		Expect(spanAttr(s, string(DeliveryResultMessageKey))).To(Equal("missing required author label"))
		Expect(spanAttr(s, string(DeliveryApplicationKey))).To(Equal("my-app"))
		Expect(spanAttr(s, string(DeliveryComponentKey))).To(Equal("my-comp"))
	})

	It("omits result_message when the supplied message is empty", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		Expect(EmitReleaseValidationFailureWait(
			context.Background(),
			"rel", "ns", nil,
			createdAt, validatedAt,
			"",
			labels,
		)).To(BeTrue())
		s := exp.GetSpans()[0]
		Expect(hasAttr(s, string(DeliveryResultMessageKey))).To(BeFalse())
	})

	It("truncates an overlong result message", func() {
		exp, tp := newTracerProvider()
		DeferCleanup(func() error { return tp.Shutdown(context.Background()) })

		long := strings.Repeat("x", MaxResultMessageLen*2)
		Expect(EmitReleaseValidationFailureWait(
			context.Background(),
			"rel", "ns", nil,
			createdAt, validatedAt,
			long,
			labels,
		)).To(BeTrue())
		emitted := spanAttr(exp.GetSpans()[0], string(DeliveryResultMessageKey))
		Expect(len(emitted)).To(BeNumerically("<=", MaxResultMessageLen))
		Expect(emitted).To(HaveSuffix(TruncatedSuffix))
	})
})
