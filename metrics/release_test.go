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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redhat-appstudio/operator-toolkit/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

var _ = Describe("Release metrics", Ordered, func() {
	var (
		initializeMetrics func()
	)

	When("RegisterCompletedRelease is called", func() {
		var completionTime, startTime *metav1.Time

		BeforeEach(func() {
			initializeMetrics()

			completionTime = &metav1.Time{}
			startTime = &metav1.Time{Time: completionTime.Add(-60 * time.Second)}
		})

		It("does nothing if the start time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedRelease(nil, completionTime, "", "", "", "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("does nothing if the completion time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedRelease(startTime, nil, "", "", "", "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("decrements ReleaseConcurrentTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedRelease(startTime, completionTime, "", "", "", "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentTotal.WithLabelValues())).To(Equal(float64(-1)))
		})

		It("adds an observation to ReleaseDurationSeconds", func() {
			RegisterCompletedRelease(startTime, completionTime,
				releaseDurationSecondsLabels[0],
				releaseDurationSecondsLabels[1],
				releaseDurationSecondsLabels[2],
				releaseDurationSecondsLabels[3],
				releaseDurationSecondsLabels[4],
				releaseDurationSecondsLabels[5],
			)
			Expect(testutil.CollectAndCompare(ReleaseDurationSeconds,
				test.NewHistogramReader(
					releaseDurationSecondsOpts,
					releaseDurationSecondsLabels,
					startTime, completionTime,
				))).To(Succeed())
		})

		It("increments ReleaseTotal", func() {
			RegisterCompletedRelease(startTime, completionTime,
				releaseTotalLabels[0],
				releaseTotalLabels[1],
				releaseTotalLabels[2],
				releaseTotalLabels[3],
				releaseTotalLabels[4],
				releaseTotalLabels[5],
			)
			Expect(testutil.CollectAndCompare(ReleaseTotal,
				test.NewCounterReader(
					releaseTotalOpts,
					releaseTotalLabels,
				))).To(Succeed())
		})
	})

	When("RegisterCompletedReleaseDeployment is called", func() {
		var completionTime, startTime *metav1.Time

		BeforeEach(func() {
			initializeMetrics()

			completionTime = &metav1.Time{}
			startTime = &metav1.Time{Time: completionTime.Add(-60 * time.Second)}
		})

		It("does nothing if the start time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentDeploymentsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleaseDeployment(nil, completionTime, "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentDeploymentsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("does nothing if the completion time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentDeploymentsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleaseDeployment(startTime, nil, "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentDeploymentsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("decrements ReleaseConcurrentDeploymentsTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentDeploymentsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleaseDeployment(startTime, completionTime, "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentDeploymentsTotal.WithLabelValues())).To(Equal(float64(-1)))
		})

		It("adds an observation to ReleaseDeploymentDurationSeconds", func() {
			RegisterCompletedReleaseDeployment(startTime, completionTime,
				releaseDeploymentDurationSecondsLabels[0],
				releaseDeploymentDurationSecondsLabels[1],
				releaseDeploymentDurationSecondsLabels[2],
			)
			Expect(testutil.CollectAndCompare(ReleaseDeploymentDurationSeconds,
				test.NewHistogramReader(
					releaseDeploymentDurationSecondsOpts,
					releaseDeploymentDurationSecondsLabels,
					startTime, completionTime,
				))).To(Succeed())
		})
	})

	When("RegisterCompletedReleasePostActionsExecuted is called", func() {
		var completionTime, startTime *metav1.Time

		BeforeEach(func() {
			initializeMetrics()

			completionTime = &metav1.Time{}
			startTime = &metav1.Time{Time: completionTime.Add(-60 * time.Second)}
		})

		It("does nothing if the start time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentPostActionsExecutionsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleasePostActionsExecuted(nil, completionTime, "")
			Expect(testutil.ToFloat64(ReleaseConcurrentPostActionsExecutionsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("does nothing if the completion time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentPostActionsExecutionsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleasePostActionsExecuted(startTime, nil, "")
			Expect(testutil.ToFloat64(ReleaseConcurrentPostActionsExecutionsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("decrements ReleaseConcurrentPostActionsExecutionsTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentPostActionsExecutionsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleasePostActionsExecuted(startTime, completionTime, "")
			Expect(testutil.ToFloat64(ReleaseConcurrentPostActionsExecutionsTotal.WithLabelValues())).To(Equal(float64(-1)))
		})

		It("adds an observation to ReleasePostActionsExecutionDurationSeconds", func() {
			RegisterCompletedReleasePostActionsExecuted(startTime, completionTime,
				releasePostActionsExecutionDurationSecondsLabels[0],
			)
			Expect(testutil.CollectAndCompare(ReleasePostActionsExecutionDurationSeconds,
				test.NewHistogramReader(
					releasePostActionsExecutionDurationSecondsOpts,
					releasePostActionsExecutionDurationSecondsLabels,
					startTime, completionTime,
				))).To(Succeed())
		})
	})

	When("RegisterCompletedReleaseProcessing is called", func() {
		var completionTime, startTime *metav1.Time

		BeforeEach(func() {
			initializeMetrics()

			completionTime = &metav1.Time{}
			startTime = &metav1.Time{Time: completionTime.Add(-60 * time.Second)}
		})

		It("does nothing if the start time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleaseProcessing(nil, completionTime, "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("does nothing if the completion time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleaseProcessing(startTime, nil, "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("decrements ReleaseConcurrentProcessingsTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleaseProcessing(startTime, completionTime, "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(-1)))
		})

		It("adds an observation to ReleaseProcessingDurationSeconds", func() {
			RegisterCompletedReleaseProcessing(startTime, completionTime,
				releaseProcessingDurationSecondsLabels[0],
				releaseProcessingDurationSecondsLabels[1],
			)
			Expect(testutil.CollectAndCompare(ReleaseProcessingDurationSeconds,
				test.NewHistogramReader(
					releaseProcessingDurationSecondsOpts,
					releaseProcessingDurationSecondsLabels,
					startTime, completionTime,
				))).To(Succeed())
		})
	})

	When("RegisterNewRelease is called", func() {
		BeforeEach(func() {
			initializeMetrics()
		})

		It("increments ReleaseConcurrentTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterNewRelease()
			Expect(testutil.ToFloat64(ReleaseConcurrentTotal.WithLabelValues())).To(Equal(float64(1)))
		})
	})

	When("RegisterNewReleaseDeployment is called", func() {
		BeforeEach(func() {
			initializeMetrics()
		})

		It("increments ReleaseConcurrentDeploymentsTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentDeploymentsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterNewReleaseDeployment()
			Expect(testutil.ToFloat64(ReleaseConcurrentDeploymentsTotal.WithLabelValues())).To(Equal(float64(1)))
		})
	})

	When("RegisterNewReleaseProcessing is called", func() {
		BeforeEach(func() {
			initializeMetrics()
		})

		It("increments ReleaseConcurrentProcessingsTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterNewReleaseProcessing()
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(1)))
		})
	})

	When("RegisterNewReleasePostActionsExecution is called", func() {
		BeforeEach(func() {
			initializeMetrics()
		})

		It("increments ReleaseConcurrentPostActionsExecutionsTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentPostActionsExecutionsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterNewReleasePostActionsExecution()
			Expect(testutil.ToFloat64(ReleaseConcurrentPostActionsExecutionsTotal.WithLabelValues())).To(Equal(float64(1)))
		})
	})

	initializeMetrics = func() {
		ReleaseConcurrentTotal.Reset()
		ReleaseConcurrentDeploymentsTotal.Reset()
		ReleaseConcurrentProcessingsTotal.Reset()
		ReleaseConcurrentPostActionsExecutionsTotal.Reset()
		ReleaseDeploymentDurationSeconds.Reset()
		ReleaseDurationSeconds.Reset()
		ReleaseProcessingDurationSeconds.Reset()
		ReleasePostActionsExecutionDurationSeconds.Reset()
		ReleaseTotal.Reset()
	}

})
