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
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func withEnv(key, value string) {
	prev, hadPrev := os.LookupEnv(key)
	Expect(os.Setenv(key, value)).To(Succeed())
	DeferCleanup(func() {
		if hadPrev {
			Expect(os.Setenv(key, prev)).To(Succeed())
		} else {
			Expect(os.Unsetenv(key)).To(Succeed())
		}
	})
}

func unsetEnv(key string) {
	prev, hadPrev := os.LookupEnv(key)
	Expect(os.Unsetenv(key)).To(Succeed())
	DeferCleanup(func() {
		if hadPrev {
			Expect(os.Setenv(key, prev)).To(Succeed())
		} else {
			Expect(os.Unsetenv(key)).To(Succeed())
		}
	})
}

func resetGlobalProviderOnCleanup() {
	prev := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tracenoop.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	DeferCleanup(func() {
		otel.SetTracerProvider(prev)
		otel.SetTextMapPropagator(prevProp)
	})
}

type testExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *testExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	return nil
}

func (e *testExporter) Shutdown(context.Context) error { return nil }

func (e *testExporter) GetSpans() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.spans
}

func newTracerProvider() (*testExporter, *sdktrace.TracerProvider) {
	exp := &testExporter{}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return exp, tp
}

func newFakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	Expect(tektonv1.AddToScheme(scheme)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func spanAttr(s sdktrace.ReadOnlySpan, key string) string {
	for _, attr := range s.Attributes() {
		if string(attr.Key) == key {
			return attr.Value.String()
		}
	}
	return ""
}

func hasAttr(s sdktrace.ReadOnlySpan, key string) bool {
	for _, attr := range s.Attributes() {
		if string(attr.Key) == key {
			return true
		}
	}
	return false
}

func defaultLabels() LabelNames {
	return LabelNames{
		Action:      DefaultTracingLabelAction,
		Application: DefaultTracingLabelApplication,
		Component:   DefaultTracingLabelComponent,
	}
}

var _ = Describe("ResultEnum", func() {
	DescribeTable("maps PipelineRun Succeeded condition to the semconv enum",
		func(status corev1.ConditionStatus, reason, want string) {
			cond := &apis.Condition{Status: status, Reason: reason}
			Expect(ResultEnum(cond).Value.AsString()).To(Equal(want))
		},
		Entry("success",
			corev1.ConditionTrue, tektonv1.PipelineRunReasonSuccessful.String(),
			semconv.CICDPipelineResultSuccess.Value.AsString()),
		Entry("completed is also success",
			corev1.ConditionTrue, tektonv1.PipelineRunReasonCompleted.String(),
			semconv.CICDPipelineResultSuccess.Value.AsString()),
		Entry("failure",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonFailed.String(),
			semconv.CICDPipelineResultFailure.Value.AsString()),
		Entry("timeout",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonTimedOut.String(),
			semconv.CICDPipelineResultTimeout.Value.AsString()),
		Entry("cancelled",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonCancelled.String(),
			semconv.CICDPipelineResultCancellation.Value.AsString()),
		Entry("cancelled-running-finally",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonCancelledRunningFinally.String(),
			semconv.CICDPipelineResultCancellation.Value.AsString()),
		Entry("stopped-running-finally",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonStoppedRunningFinally.String(),
			semconv.CICDPipelineResultCancellation.Value.AsString()),
		Entry("validation failure falls back to error",
			corev1.ConditionFalse, tektonv1.PipelineRunReasonFailedValidation.String(),
			semconv.CICDPipelineResultError.Value.AsString()),
		Entry("unknown reason falls back to error",
			corev1.ConditionFalse, "SomeFutureTektonReason",
			semconv.CICDPipelineResultError.Value.AsString()),
	)

	It("returns error when the condition is nil", func() {
		Expect(ResultEnum(nil).Value.AsString()).To(Equal(semconv.CICDPipelineResultError.Value.AsString()))
	})
})

var _ = Describe("TruncateResultMessage", func() {
	It("passes a short message through unchanged", func() {
		Expect(TruncateResultMessage("short")).To(Equal("short"))
	})

	It("passes a message exactly at the limit through unchanged", func() {
		msg := strings.Repeat("a", MaxResultMessageLen)
		Expect(TruncateResultMessage(msg)).To(Equal(msg))
	})

	It("truncates and suffixes a message over the limit", func() {
		msg := strings.Repeat("a", MaxResultMessageLen+50)
		got := TruncateResultMessage(msg)
		Expect(len(got)).To(BeNumerically("<=", MaxResultMessageLen))
		Expect(got).To(HaveSuffix(TruncatedSuffix))
	})

	It("preserves UTF-8 boundaries when truncating", func() {
		prefix := strings.Repeat("a", MaxResultMessageLen-len(TruncatedSuffix)-2)
		msg := prefix + "世" + strings.Repeat("b", 100)
		got := TruncateResultMessage(msg)
		head := strings.TrimSuffix(got, TruncatedSuffix)
		Expect(head).NotTo(Equal(got))
		for _, r := range head {
			Expect(r).NotTo(Equal('�'))
		}
	})
})

var _ = Describe("EarliestFailingTaskRunMessage", func() {
	var (
		t1 metav1.Time
		t2 metav1.Time
		t3 metav1.Time
	)

	BeforeEach(func() {
		now := time.Now()
		t1 = metav1.NewTime(now.Add(-3 * time.Minute))
		t2 = metav1.NewTime(now.Add(-2 * time.Minute))
		t3 = metav1.NewTime(now.Add(-time.Minute))
	})

	mkTR := func(name string, completion *metav1.Time, status corev1.ConditionStatus, message string) *tektonv1.TaskRun {
		tr := &tektonv1.TaskRun{ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ns",
			Labels:    map[string]string{"tekton.dev/pipelineRun": "pr"},
		}}
		tr.Status.CompletionTime = completion
		if completion != nil {
			tr.Status.Conditions = duckv1.Conditions{{
				Type:    apis.ConditionSucceeded,
				Status:  status,
				Message: message,
			}}
		}
		return tr
	}

	mkPR := func(names ...string) *tektonv1.PipelineRun {
		pr := &tektonv1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "pr", Namespace: "ns"}}
		for _, n := range names {
			pr.Status.ChildReferences = append(pr.Status.ChildReferences, tektonv1.ChildStatusReference{Name: n})
		}
		return pr
	}

	It("returns empty when the PipelineRun has no child references", func() {
		got := EarliestFailingTaskRunMessage(context.Background(), newFakeClient(), mkPR())
		Expect(got).To(Equal(""))
	})

	It("returns empty when the only child succeeded", func() {
		c := newFakeClient(mkTR("tr-ok", &t2, corev1.ConditionTrue, "done"))
		Expect(EarliestFailingTaskRunMessage(context.Background(), c, mkPR("tr-ok"))).To(Equal(""))
	})

	It("returns the failing child's message when one child fails", func() {
		c := newFakeClient(mkTR("tr-fail", &t2, corev1.ConditionFalse, "boom"))
		Expect(EarliestFailingTaskRunMessage(context.Background(), c, mkPR("tr-fail"))).To(Equal("boom"))
	})

	It("returns the earliest failing message when multiple children fail", func() {
		c := newFakeClient(
			mkTR("tr-late", &t3, corev1.ConditionFalse, "late"),
			mkTR("tr-early", &t1, corev1.ConditionFalse, "early"),
		)
		Expect(EarliestFailingTaskRunMessage(context.Background(), c, mkPR("tr-late", "tr-early"))).To(Equal("early"))
	})

	It("picks the failing child when success and failure are mixed", func() {
		c := newFakeClient(
			mkTR("tr-ok", &t1, corev1.ConditionTrue, "done"),
			mkTR("tr-fail", &t2, corev1.ConditionFalse, "bad"),
		)
		Expect(EarliestFailingTaskRunMessage(context.Background(), c, mkPR("tr-ok", "tr-fail"))).To(Equal("bad"))
	})

	It("skips incomplete children", func() {
		c := newFakeClient(
			mkTR("tr-running", nil, corev1.ConditionUnknown, ""),
			mkTR("tr-fail", &t2, corev1.ConditionFalse, "later"),
		)
		Expect(EarliestFailingTaskRunMessage(context.Background(), c, mkPR("tr-running", "tr-fail"))).To(Equal("later"))
	})

	It("returns only what the List finds when some referenced TaskRuns are absent from the cluster", func() {
		c := newFakeClient(mkTR("tr-fail", &t2, corev1.ConditionFalse, "present-failure"))
		Expect(EarliestFailingTaskRunMessage(context.Background(), c, mkPR("tr-fail", "tr-missing"))).To(Equal("present-failure"))
	})
})

var _ = Describe("LoadLabelNames", func() {
	envKeys := []string{
		EnvTracingLabelAction,
		EnvTracingLabelApplication,
		EnvTracingLabelComponent,
	}

	saveEnv := func() map[string]string {
		saved := map[string]string{}
		for _, k := range envKeys {
			if v, ok := os.LookupEnv(k); ok {
				saved[k] = v
			}
		}
		return saved
	}

	restoreEnv := func(saved map[string]string) {
		for _, k := range envKeys {
			Expect(os.Unsetenv(k)).To(Succeed())
		}
		for k, v := range saved {
			Expect(os.Setenv(k, v)).To(Succeed())
		}
	}

	BeforeEach(func() {
		SetupLabelNames()
		DeferCleanup(SetupLabelNames)
	})

	It("returns the delivery.tekton.dev/* defaults when env vars are unset", func() {
		saved := saveEnv()
		DeferCleanup(restoreEnv, saved)
		for _, k := range envKeys {
			Expect(os.Unsetenv(k)).To(Succeed())
		}
		SetupLabelNames()
		got := LoadLabelNames()
		Expect(got.Action).To(Equal(DefaultTracingLabelAction))
		Expect(got.Application).To(Equal(DefaultTracingLabelApplication))
		Expect(got.Component).To(Equal(DefaultTracingLabelComponent))
	})

	It("treats an explicit empty-string as disabling the attribute", func() {
		saved := saveEnv()
		DeferCleanup(restoreEnv, saved)
		for _, k := range envKeys {
			Expect(os.Setenv(k, "")).To(Succeed())
		}
		SetupLabelNames()
		got := LoadLabelNames()
		Expect(got.Action).To(Equal(""))
		Expect(got.Application).To(Equal(""))
		Expect(got.Component).To(Equal(""))
	})
})

var _ = Describe("samplerFromEnv", func() {
	DescribeTable("returns the sampler that matches the requested decision tree",
		func(samplerEnv, argEnv, wantPrefix string) {
			if samplerEnv == "" {
				unsetEnv(EnvTracesSampler)
			} else {
				withEnv(EnvTracesSampler, samplerEnv)
			}
			if argEnv == "" {
				unsetEnv(EnvTracesSamplerArg)
			} else {
				withEnv(EnvTracesSamplerArg, argEnv)
			}
			Expect(samplerFromEnv().Description()).To(HavePrefix(wantPrefix))
		},
		Entry("always_on selects the unconditional sampler",
			"always_on", "", "AlwaysOnSampler"),
		Entry("always_off selects the never-sample sampler",
			"always_off", "", "AlwaysOffSampler"),
		Entry("traceidratio with arg=0.5 selects the matching ratio sampler",
			"traceidratio", "0.5", "TraceIDRatioBased{0.5"),
		Entry("traceidratio with no arg falls back to ratio 0",
			"traceidratio", "", "TraceIDRatioBased{0"),
		Entry("parentbased_always_off honors a parent's sampling decision but defaults off",
			"parentbased_always_off", "", "ParentBased{root:AlwaysOffSampler"),
		Entry("parentbased_traceidratio applies the ratio at the root",
			"parentbased_traceidratio", "0.25", "ParentBased{root:TraceIDRatioBased{0.25"),
		Entry("unset env defaults to parent-based always-on",
			"", "", "ParentBased{root:AlwaysOnSampler"),
		Entry("unrecognized env value falls back to the default",
			"some-future-otel-keyword", "", "ParentBased{root:AlwaysOnSampler"),
	)
})

var _ = Describe("New", func() {
	It("disables sampling when the OTLP endpoint env var is unset", func() {
		unsetEnv(EnvOTLPEndpoint)
		resetGlobalProviderOnCleanup()

		tp := New()
		Expect(tp).NotTo(BeNil())

		_, isSDK := otel.GetTracerProvider().(*sdktrace.TracerProvider)
		Expect(isSDK).To(BeFalse())

		_, span := otel.Tracer("no-endpoint-probe").Start(context.Background(), "probe")
		defer span.End()
		Expect(span.SpanContext().IsSampled()).To(BeFalse())

		Expect(tp.Shutdown(context.Background())).To(Succeed())
	})

	It("installs an SDK TracerProvider and a W3C TraceContext propagator when the endpoint is well-formed", func() {
		withEnv(EnvOTLPEndpoint, "http://localhost:4317")
		resetGlobalProviderOnCleanup()

		tp := New()
		Expect(tp).NotTo(BeNil())

		_, isSDK := otel.GetTracerProvider().(*sdktrace.TracerProvider)
		Expect(isSDK).To(BeTrue())
		_, isW3C := otel.GetTextMapPropagator().(propagation.TraceContext)
		Expect(isW3C).To(BeTrue())

		Expect(tp.Shutdown(context.Background())).To(Succeed())
	})
})

var _ = Describe("TracerProvider.Shutdown", func() {
	It("returns nil for a noop-backed provider", func() {
		unsetEnv(EnvOTLPEndpoint)
		resetGlobalProviderOnCleanup()

		tp := New()
		Expect(tp.Shutdown(context.Background())).To(Succeed())
	})

	It("is idempotent for an SDK-backed provider", func() {
		withEnv(EnvOTLPEndpoint, "http://localhost:4317")
		resetGlobalProviderOnCleanup()

		tp := New()
		Expect(tp.Shutdown(context.Background())).To(Succeed())
		// A second call must not panic or surface an error.
		Expect(tp.Shutdown(context.Background())).To(Succeed())
	})

	It("is a noop on a TracerProvider with no shutdown hook", func() {
		tp := &TracerProvider{}
		Expect(tp.Shutdown(context.Background())).To(Succeed())
	})
})
