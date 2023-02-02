/*
Copyright 2022.

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

package metrics

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Metrics Release", Ordered, func() {
	BeforeAll(func() {
		// We need to unregister in advance otherwise it breaks with 'AlreadyRegisteredError'
		metrics.Registry.Unregister(ReleaseAttemptRunningSeconds)
		metrics.Registry.Unregister(ReleaseAttemptConcurrentTotal)
		metrics.Registry.Unregister(ReleaseAttemptDeploymentSeconds)
		metrics.Registry.Unregister(ReleaseAttemptDurationSeconds)
	})

	var (
		AttemptRunningSecondsHeader = inputHeader{
			Name: "release_attempt_running_seconds",
			Help: "Release durations from the moment the release resource was created til the release is marked as running",
		}
		AttemptDeploymentSecondsHeader = inputHeader{
			Name: "release_attempt_deployment_seconds",
			Help: "Release durations from the moment the SnapshotEnvironmentBinding was created til the release is marked as deployed",
		}
		AttemptDeploymentTotalHeader = inputHeader{
			Name: "release_attempt_deployment_total",
			Help: "Total number of deployments released to managed environments by the operator",
		}
		AttemptDurationSecondsHeader = inputHeader{
			Name: "release_attempt_duration_seconds",
			Help: "Release durations from the moment the release PipelineRun was created til the release is marked as finished",
		}
		AttemptTotalHeader = inputHeader{
			Name: "release_attempt_total",
			Help: "Total number of releases processed by the operator",
		}
	)

	const (
		validReleaseReason   = "valid_release_reason"
		invalidReleaseReason = "invalid_release_reason"
		strategy             = "nostrategy"
		deployReason         = "CommitsSynced"
		deploySuccess        = "True"
	)

	var defaultNamespace = "default"

	Context("When RegisterNewRelease is called", func() {
		// As we need to share metrics within the Context, we need to use "per Context" '(Before|After)All'
		BeforeAll(func() {
			// Mocking metrics to be able to reset data with each tests. Otherwise, we would have to take previous tests into account.
			//
			// 'Help' can't be overridden due to 'https://github.com/prometheus/client_golang/blob/83d56b1144a0c2eb10d399e7abbae3333bebc463/prometheus/registry.go#L314'
			ReleaseAttemptRunningSeconds = prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name:    "release_attempt_running_seconds",
					Help:    "Release durations from the moment the release resource was created til the release is marked as running",
					Buckets: []float64{1, 5, 10, 30},
				},
			)
			ReleaseAttemptConcurrentTotal = prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "release_attempt_concurrent_requests",
					Help: "Total number of concurrent release attempts",
				},
			)
			metrics.Registry.MustRegister(ReleaseAttemptRunningSeconds, ReleaseAttemptConcurrentTotal)
		})

		AfterAll(func() {
			metrics.Registry.Unregister(ReleaseAttemptRunningSeconds)
			metrics.Registry.Unregister(ReleaseAttemptConcurrentTotal)
		})

		// Input seconds for duration of operations less or equal to the following buckets of 1, 5, 10 and 30 seconds
		inputSeconds := []float64{1, 3, 8, 15}
		elapsedSeconds := 0.0

		It("increments the 'release_attempt_concurrent_total'.", func() {
			creationTime := metav1.Time{}
			for _, seconds := range inputSeconds {
				startTime := metav1.NewTime(creationTime.Add(time.Second * time.Duration(seconds)))
				elapsedSeconds += seconds
				RegisterNewRelease(creationTime, &startTime)
			}
			Expect(testutil.ToFloat64(ReleaseAttemptConcurrentTotal)).To(Equal(float64(len(inputSeconds))))
		})

		It("registers a new observation for 'release_attempt_running_seconds' with the elapsed time from the moment"+
			"the Release was created to when it started (Release marked as 'Running').", func() {
			// Defined buckets for ReleaseAttemptRunningSeconds
			timeBuckets := []string{"1", "5", "10", "30"}
			data := []int{1, 2, 3, 4}
			readerData := createHistogramReader(AttemptRunningSecondsHeader, timeBuckets, data, "", elapsedSeconds, len(inputSeconds))
			Expect(testutil.CollectAndCompare(ReleaseAttemptRunningSeconds, strings.NewReader(readerData))).To(Succeed())
		})
	})

	Context("When RegisterCompletedRelease is called", func() {
		BeforeAll(func() {
			ReleaseAttemptRunningSeconds = prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name:    "release_attempt_running_seconds",
					Help:    "Release durations from the moment the release resource was created til the release is marked as running",
					Buckets: []float64{1, 5, 10, 30},
				},
			)
			ReleaseAttemptDurationSeconds = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "release_attempt_duration_seconds",
					Help:    "Release durations from the moment the release PipelineRun was created til the release is marked as finished",
					Buckets: []float64{60, 600, 1800, 3600},
				},
				[]string{"reason", "strategy", "succeeded", "target"},
			)

			ReleaseAttemptConcurrentTotal = prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "release_attempt_concurrent_requests",
					Help: "Total number of concurrent release attempts",
				},
			)
			metrics.Registry.MustRegister(ReleaseAttemptRunningSeconds, ReleaseAttemptDurationSeconds, ReleaseAttemptConcurrentTotal)
		})

		AfterAll(func() {
			metrics.Registry.Unregister(ReleaseAttemptRunningSeconds)
			metrics.Registry.Unregister(ReleaseAttemptDurationSeconds)
			metrics.Registry.Unregister(ReleaseAttemptConcurrentTotal)
		})

		// Input seconds for duration of operations less or equal to the following buckets of 60, 600, 1800 and 3600 seconds
		inputSeconds := []float64{30, 500, 1500, 3000}
		elapsedSeconds := 0.0
		labels := fmt.Sprintf(`reason="%s", strategy="%s", succeeded="true", target="%s",`,
			validReleaseReason, strategy, defaultNamespace)

		It("increments 'ReleaseAttemptConcurrentTotal' so we can decrement it to a non-negative number in the next test", func() {
			creationTime := metav1.Time{}
			for _, seconds := range inputSeconds {
				startTime := metav1.NewTime(creationTime.Add(time.Second * time.Duration(seconds)))
				RegisterNewRelease(creationTime, &startTime)
			}
			Expect(testutil.ToFloat64(ReleaseAttemptConcurrentTotal)).To(Equal(float64(len(inputSeconds))))
		})

		It("increments 'ReleaseAttemptTotal' and decrements 'ReleaseAttemptConcurrentTotal'", func() {
			completionTime := metav1.Time{}
			for _, seconds := range inputSeconds {
				completionTime := metav1.NewTime(completionTime.Add(time.Second * time.Duration(seconds)))
				elapsedSeconds += seconds
				RegisterCompletedRelease(validReleaseReason, strategy, defaultNamespace, &metav1.Time{}, &completionTime, true)
			}
			readerData := createCounterReader(AttemptTotalHeader, labels, true, len(inputSeconds))
			Expect(testutil.ToFloat64(ReleaseAttemptConcurrentTotal)).To(Equal(0.0))
			Expect(testutil.CollectAndCompare(ReleaseAttemptTotal, strings.NewReader(readerData))).To(Succeed())
		})

		It("registers a new observation for 'ReleaseAttemptDurationSeconds' with the elapsed time from the moment the Release attempt started (Release marked as 'Running').", func() {
			timeBuckets := []string{"60", "600", "1800", "3600"}
			// For each time bucket how many Releases completed below 4 seconds
			data := []int{1, 2, 3, 4}
			readerData := createHistogramReader(AttemptDurationSecondsHeader, timeBuckets, data, labels, elapsedSeconds, len(inputSeconds))
			Expect(testutil.CollectAndCompare(ReleaseAttemptDurationSeconds, strings.NewReader(readerData))).To(Succeed())
		})
	})

	Context("When RegisterDeployedRelease is called", func() {
		BeforeAll(func() {
			ReleaseAttemptDeploymentSeconds = prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name:    "release_attempt_deployment_seconds",
					Help:    "Release durations from the moment the SnapshotEnvironmentBinding was created til the release is marked as deployed",
					Buckets: []float64{1, 5, 10, 30},
				},
			)
			metrics.Registry.MustRegister(ReleaseAttemptDeploymentSeconds)
		})

		AfterAll(func() {
			metrics.Registry.Unregister(ReleaseAttemptDeploymentSeconds)
		})

		// Input seconds for duration of operations less or equal to the following buckets of 1, 5, 10 and 30 seconds
		inputSeconds := []float64{1, 3, 8, 15}
		elapsedSeconds := 0.0

		It("increments the 'ReleaseAttemptDeploymentTotal' metric.", func() {
			startTime := metav1.Time{}
			for _, seconds := range inputSeconds {
				completionTime := metav1.NewTime(startTime.Add(time.Second * time.Duration(seconds)))
				elapsedSeconds += seconds
				RegisterDeployedRelease(deployReason, "", deploySuccess, &startTime, &completionTime)
			}

			labels := fmt.Sprintf(`reason="%s", succeeded="%s", target="",`, deployReason, deploySuccess)
			readerData := createCounterReader(AttemptDeploymentTotalHeader, labels, true, len(inputSeconds))
			Expect(testutil.CollectAndCompare(ReleaseAttemptDeploymentTotal.WithLabelValues("CommitsSynced", "True", ""),
				strings.NewReader(readerData))).To(Succeed())
		})

		It("registers a new observation for 'ReleaseAttemptDeploymentSeconds' with the time difference between the passed "+
			"start time and finish time.", func() {
			// Defined buckets for ReleaseAttemptDeploymentSeconds
			timeBuckets := []string{"1", "5", "10", "30"}
			data := []int{1, 2, 3, 4}
			readerData := createHistogramReader(AttemptDeploymentSecondsHeader, timeBuckets, data, "", elapsedSeconds, len(inputSeconds))
			Expect(testutil.CollectAndCompare(ReleaseAttemptDeploymentSeconds, strings.NewReader(readerData))).To(Succeed())
		})
	})

	Context("When RegisterInvalidRelease", func() {
		It("increments the 'ReleaseAttemptInvalidTotal' metric", func() {
			for i := 0; i < 10; i++ {
				RegisterInvalidRelease(invalidReleaseReason)
			}
			Expect(testutil.ToFloat64(ReleaseAttemptInvalidTotal)).To(Equal(float64(10)))
		})

		It("increments the 'ReleaseAttemptTotal' metric.", func() {
			labels := fmt.Sprintf(`reason="%s", strategy="", succeeded="false", target="",`, invalidReleaseReason)
			readerData := createCounterReader(AttemptTotalHeader, labels, true, 10.0)
			Expect(testutil.CollectAndCompare(ReleaseAttemptTotal.WithLabelValues("invalid_release_reason", "", "false", ""),
				strings.NewReader(readerData))).To(Succeed())
		})
	})
})
