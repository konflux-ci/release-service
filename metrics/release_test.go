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
	"time"

	"github.com/konflux-ci/operator-toolkit/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	When("RegisterCompletedReleasePipelineProcessing is called", func() {
		var completionTime, startTime *metav1.Time

		BeforeEach(func() {
			initializeMetrics()

			completionTime = &metav1.Time{}
			startTime = &metav1.Time{Time: completionTime.Add(-60 * time.Second)}
		})

		It("does nothing if the start time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleasePipelineProcessing(nil, completionTime, "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("does nothing if the completion time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleasePipelineProcessing(startTime, nil, "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("decrements ReleaseConcurrentProcessingsTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterCompletedReleasePipelineProcessing(startTime, completionTime, "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(-1)))
		})

		It("adds an observation to ReleaseProcessingDurationSeconds", func() {
			RegisterCompletedReleasePipelineProcessing(startTime, completionTime,
				releaseProcessingDurationSecondsLabels[0],
				releaseProcessingDurationSecondsLabels[1],
				releaseProcessingDurationSecondsLabels[2],
			)
			Expect(testutil.CollectAndCompare(ReleaseProcessingDurationSeconds,
				test.NewHistogramReader(
					releaseProcessingDurationSecondsOpts,
					releaseProcessingDurationSecondsLabels,
					startTime, completionTime,
				))).To(Succeed())
		})
	})

	When("RegisterValidatedRelease is called", func() {
		var validationTime, startTime *metav1.Time

		BeforeEach(func() {
			initializeMetrics()

			validationTime = &metav1.Time{}
			startTime = &metav1.Time{Time: validationTime.Add(-60 * time.Second)}
		})

		It("does nothing if the validation start time is nil", func() {
			RegisterValidatedRelease(nil, validationTime, "", "")
		})

		It("does nothing if the start time is nil", func() {
			RegisterValidatedRelease(startTime, nil, "", "")
		})

		It("adds an observation to ReleaseValidationDurationSeconds", func() {
			RegisterValidatedRelease(startTime, validationTime,
				releaseValidationDurationSecondsLabels[0],
				releaseValidationDurationSecondsLabels[1],
			)
			Expect(testutil.CollectAndCompare(ReleaseValidationDurationSeconds,
				test.NewHistogramReader(
					releaseValidationDurationSecondsOpts,
					releaseValidationDurationSecondsLabels,
					startTime, validationTime,
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

	When("RegisterNewReleasePipelineProcessing is called", func() {
		var processingStartTime, startTime *metav1.Time

		BeforeEach(func() {
			initializeMetrics()

			processingStartTime = &metav1.Time{}
			startTime = &metav1.Time{Time: processingStartTime.Add(-60 * time.Second)}
		})

		It("does nothing if the processing start time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterNewReleasePipelineProcessing(nil, processingStartTime, "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("does nothing if the start time is nil", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterNewReleasePipelineProcessing(startTime, nil, "", "", "")
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
		})

		It("adds an observation to ReleasePreProcessingDurationSeconds", func() {
			RegisterNewReleasePipelineProcessing(startTime, processingStartTime,
				releasePreProcessingDurationSecondsLabels[0],
				releasePreProcessingDurationSecondsLabels[1],
				releasePreProcessingDurationSecondsLabels[2],
			)
			Expect(testutil.CollectAndCompare(ReleasePreProcessingDurationSeconds,
				test.NewHistogramReader(
					releasePreProcessingDurationSecondsOpts,
					releasePreProcessingDurationSecondsLabels,
					startTime, processingStartTime,
				))).To(Succeed())
		})

		It("increments ReleaseConcurrentProcessingsTotal", func() {
			Expect(testutil.ToFloat64(ReleaseConcurrentProcessingsTotal.WithLabelValues())).To(Equal(float64(0)))
			RegisterNewReleasePipelineProcessing(startTime, processingStartTime,
				releasePreProcessingDurationSecondsLabels[0],
				releasePreProcessingDurationSecondsLabels[1],
				releasePreProcessingDurationSecondsLabels[2],
			)
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
		ReleaseConcurrentProcessingsTotal.Reset()
		ReleaseConcurrentPostActionsExecutionsTotal.Reset()
		ReleaseValidationDurationSeconds.Reset()
		ReleasePreProcessingDurationSeconds.Reset()
		ReleaseDurationSeconds.Reset()
		ReleaseProcessingDurationSeconds.Reset()
		ReleasePostActionsExecutionDurationSeconds.Reset()
		ReleaseTotal.Reset()
	}

})
