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
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ReleaseAttemptConcurrentTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "release_attempt_concurrent_requests",
			Help: "Total number of concurrent release attempts",
		},
	)

	ReleaseAttemptDeploymentSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "release_attempt_deployment_seconds",
			Help:    "Release durations from the moment the SnapshotEnvironmentBinding was created til the release is marked as deployed",
			Buckets: []float64{10, 20, 40, 60, 120, 240, 360, 480, 600},
		},
	)

	ReleaseAttemptDeploymentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_attempt_deployment_total",
			Help: "Total number of deployments released to managed environments by the operator",
		},
		[]string{"reason", "succeeded", "target"},
	)

	ReleaseAttemptDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_attempt_duration_seconds",
			Help:    "Release durations from the moment the release PipelineRun was created til the release is marked as finished",
			Buckets: []float64{60, 150, 300, 450, 600, 750, 900, 1050, 1200, 1800, 3600},
		},
		[]string{"reason", "strategy", "succeeded", "target"},
	)

	ReleaseAttemptInvalidTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_attempt_invalid_total",
			Help: "Number of invalid releases",
		},
		[]string{"reason"},
	)

	ReleaseAttemptRunningSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "release_attempt_running_seconds",
			Help:    "Release durations from the moment the release resource was created til the release is marked as running",
			Buckets: []float64{0.5, 1, 2, 3, 4, 5, 6, 7, 10, 15, 30},
		},
	)

	ReleaseAttemptTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_attempt_total",
			Help: "Total number of releases processed by the operator",
		},
		[]string{"reason", "strategy", "succeeded", "target"},
	)
)

// RegisterCompletedRelease decrements the 'release_attempt_concurrent_total' metric, increments `release_attempt_total`
// and registers a new observation for 'release_attempt_duration_seconds' with the elapsed time from the moment the
// Release attempt started (Release marked as 'Running').
func RegisterCompletedRelease(reason, strategy, target string, startTime, completionTime *metav1.Time, succeeded bool) {
	labels := prometheus.Labels{
		"reason":    reason,
		"strategy":  strategy,
		"succeeded": strconv.FormatBool(succeeded),
		"target":    target,
	}

	ReleaseAttemptConcurrentTotal.Dec()
	ReleaseAttemptDurationSeconds.With(labels).Observe(completionTime.Sub(startTime.Time).Seconds())
	ReleaseAttemptTotal.With(labels).Inc()
}

// RegisterDeployedRelease increments the 'release_attempt_deployment_total' and registers a new observation for
// 'release_attempt_deployment_seconds' with the elapsed time from the moment the SnapshotEnvironmentBinding was
// created to when it was marked as deployed.
func RegisterDeployedRelease(reason, target, succeeded string, startTime, completionTime *metav1.Time) {
	labels := prometheus.Labels{
		"reason":    reason,
		"succeeded": succeeded,
		"target":    target,
	}

	ReleaseAttemptDeploymentSeconds.Observe(completionTime.Sub(startTime.Time).Seconds())
	ReleaseAttemptDeploymentTotal.With(labels).Inc()
}

// RegisterInvalidRelease increments the 'release_attempt_invalid_total' and `release_attempt_total` metrics.
func RegisterInvalidRelease(reason string) {
	ReleaseAttemptInvalidTotal.With(prometheus.Labels{"reason": reason}).Inc()
	ReleaseAttemptTotal.With(prometheus.Labels{
		"reason":    reason,
		"strategy":  "",
		"succeeded": "false",
		"target":    "",
	}).Inc()
}

// RegisterNewRelease increments the 'release_attempt_concurrent_total' and registers a new observation for
// 'release_attempt_duration_seconds' with the elapsed time from the moment the Release was created to when
// it started (Release marked as 'Running').
func RegisterNewRelease(creationTime metav1.Time, startTime *metav1.Time) {
	ReleaseAttemptConcurrentTotal.Inc()
	ReleaseAttemptRunningSeconds.Observe(startTime.Sub(creationTime.Time).Seconds())
}

func init() {
	metrics.Registry.MustRegister(
		ReleaseAttemptConcurrentTotal,
		ReleaseAttemptDeploymentSeconds,
		ReleaseAttemptDeploymentTotal,
		ReleaseAttemptDurationSeconds,
		ReleaseAttemptInvalidTotal,
		ReleaseAttemptRunningSeconds,
		ReleaseAttemptTotal,
	)
}
