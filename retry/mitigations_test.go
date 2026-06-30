/*
Copyright 2026.

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

package retry_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/retry"
)

var _ = Describe("Mitigations", func() {

	Context("ApplyMemoryMitigation", func() {
		It("should multiply memory limits and requests", func() {
			current := &corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")},
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("32Mi")},
			}
			result := retry.ApplyMemoryMitigation(current, &v1alpha1.MemoryMitigation{Multiplier: "2"})
			Expect(result).NotTo(BeNil())
			Expect(result.Limits.Memory().Value()).To(Equal(int64(128 * 1024 * 1024)))
			Expect(result.Requests.Memory().Value()).To(Equal(int64(64 * 1024 * 1024)))
		})

		It("should round up for fractional multipliers", func() {
			current := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("100Mi")},
			}
			result := retry.ApplyMemoryMitigation(current, &v1alpha1.MemoryMitigation{Multiplier: "1.5"})
			Expect(result).NotTo(BeNil())
			Expect(result.Limits.Memory().Value()).To(Equal(int64(150 * 1024 * 1024)))
		})

		It("should cap limits and requests at MaxComputeResources", func() {
			current := &corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
			}
			result := retry.ApplyMemoryMitigation(current, &v1alpha1.MemoryMitigation{
				Multiplier: "4",
				MaxComputeResources: &corev1.ResourceRequirements{
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
					Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
				},
			})
			Expect(result).NotTo(BeNil())
			Expect(result.Limits.Memory().Value()).To(Equal(int64(1024 * 1024 * 1024)))
			Expect(result.Requests.Memory().Value()).To(Equal(int64(1024 * 1024 * 1024)))
		})

		It("should return nil for nil current", func() {
			Expect(retry.ApplyMemoryMitigation(nil, &v1alpha1.MemoryMitigation{Multiplier: "2"})).To(BeNil())
		})

		It("should return nil for nil mitigation", func() {
			current := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")},
			}
			Expect(retry.ApplyMemoryMitigation(current, nil)).To(BeNil())
		})

		It("should return nil when no memory is configured", func() {
			current := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
			}
			Expect(retry.ApplyMemoryMitigation(current, &v1alpha1.MemoryMitigation{Multiplier: "2"})).To(BeNil())
		})

		It("should return nil for invalid multiplier", func() {
			current := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")},
			}
			Expect(retry.ApplyMemoryMitigation(current, &v1alpha1.MemoryMitigation{Multiplier: "abc"})).To(BeNil())
		})

		It("should not mutate the original", func() {
			current := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")},
			}
			before := current.Limits.Memory().Value()
			retry.ApplyMemoryMitigation(current, &v1alpha1.MemoryMitigation{Multiplier: "2"})
			Expect(current.Limits.Memory().Value()).To(Equal(before))
		})
	})

	Context("ApplyTaskTimeoutMitigation", func() {
		It("should add the increment without touching pipeline when task still fits", func() {
			pipelineTimeouts := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 30 * time.Minute},
				Tasks:    &metav1.Duration{Duration: 20 * time.Minute},
			}
			taskResult, pipelineResult := retry.ApplyTaskTimeoutMitigation(
				&metav1.Duration{Duration: 5 * time.Minute},
				pipelineTimeouts,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 10 * time.Minute}},
			)
			Expect(taskResult.Duration).To(Equal(15 * time.Minute))
			Expect(pipelineResult.Tasks.Duration).To(Equal(20 * time.Minute))
			Expect(pipelineResult.Pipeline.Duration).To(Equal(30 * time.Minute))
		})

		It("should bump tasks when the new task timeout exceeds it", func() {
			pipelineTimeouts := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
				Tasks:    &metav1.Duration{Duration: 10 * time.Minute},
			}
			taskResult, pipelineResult := retry.ApplyTaskTimeoutMitigation(
				&metav1.Duration{Duration: 5 * time.Minute},
				pipelineTimeouts,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 15 * time.Minute}},
			)
			Expect(taskResult.Duration).To(Equal(20 * time.Minute))
			Expect(pipelineResult.Tasks.Duration).To(Equal(20 * time.Minute))
			// pipeline is still large enough, no change needed
			Expect(pipelineResult.Pipeline.Duration).To(Equal(1 * time.Hour))
		})

		It("should bump both tasks and pipeline when both are exceeded", func() {
			pipelineTimeouts := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 3 * time.Minute},
				Tasks:    &metav1.Duration{Duration: 2 * time.Minute},
				Finally:  &metav1.Duration{Duration: 1 * time.Minute},
			}
			taskResult, pipelineResult := retry.ApplyTaskTimeoutMitigation(
				&metav1.Duration{Duration: 10 * time.Second},
				pipelineTimeouts,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 15 * time.Minute}},
			)
			Expect(taskResult.Duration).To(Equal(15*time.Minute + 10*time.Second))
			Expect(pipelineResult.Tasks.Duration).To(Equal(15*time.Minute + 10*time.Second))
			Expect(pipelineResult.Pipeline.Duration).To(Equal(16*time.Minute + 10*time.Second))
		})

		It("should cap the task timeout at MaxTimeout", func() {
			max := metav1.Duration{Duration: 1 * time.Hour}
			taskResult, _ := retry.ApplyTaskTimeoutMitigation(
				&metav1.Duration{Duration: 50 * time.Minute},
				nil,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 20 * time.Minute}, MaxTimeout: &max},
			)
			Expect(taskResult.Duration).To(Equal(1 * time.Hour))
		})

		It("should use the increment as the timeout when current is nil", func() {
			taskResult, _ := retry.ApplyTaskTimeoutMitigation(nil, nil,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 10 * time.Minute}},
			)
			Expect(taskResult.Duration).To(Equal(10 * time.Minute))
		})

		It("should leave pipeline timeouts nil when none were set", func() {
			taskResult, pipelineResult := retry.ApplyTaskTimeoutMitigation(
				&metav1.Duration{Duration: 5 * time.Minute},
				nil,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 10 * time.Minute}},
			)
			Expect(taskResult.Duration).To(Equal(15 * time.Minute))
			Expect(pipelineResult.Pipeline).To(BeNil())
			Expect(pipelineResult.Tasks).To(BeNil())
		})

		It("should not bump pipeline when only tasks is set", func() {
			pipelineTimeouts := &tektonv1.TimeoutFields{
				Tasks: &metav1.Duration{Duration: 5 * time.Minute},
			}
			taskResult, pipelineResult := retry.ApplyTaskTimeoutMitigation(
				&metav1.Duration{Duration: 5 * time.Minute},
				pipelineTimeouts,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 15 * time.Minute}},
			)
			Expect(taskResult.Duration).To(Equal(20 * time.Minute))
			Expect(pipelineResult.Tasks.Duration).To(Equal(20 * time.Minute))
			// no pipeline set, so nothing to enforce
			Expect(pipelineResult.Pipeline).To(BeNil())
		})

		It("should return current values when mitigation is nil", func() {
			current := &metav1.Duration{Duration: 5 * time.Minute}
			taskResult, pipelineResult := retry.ApplyTaskTimeoutMitigation(current, nil, nil)
			Expect(taskResult).To(Equal(current))
			Expect(pipelineResult).To(BeNil())
		})
	})

	Context("ApplyPipelineTimeoutMitigation", func() {
		It("should add the increment and preserve tasks timeout", func() {
			current := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
				Tasks:    &metav1.Duration{Duration: 50 * time.Minute},
			}
			result := retry.ApplyPipelineTimeoutMitigation(current,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 30 * time.Minute}},
			)
			Expect(result.Pipeline.Duration).To(Equal(90 * time.Minute))
			Expect(result.Tasks.Duration).To(Equal(50 * time.Minute))
		})

		It("should cap at MaxTimeout", func() {
			max := metav1.Duration{Duration: 1 * time.Hour}
			result := retry.ApplyPipelineTimeoutMitigation(
				&tektonv1.TimeoutFields{Pipeline: &metav1.Duration{Duration: 50 * time.Minute}},
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 30 * time.Minute}, MaxTimeout: &max},
			)
			Expect(result.Pipeline.Duration).To(Equal(1 * time.Hour))
		})

		It("should use the increment when current is nil", func() {
			result := retry.ApplyPipelineTimeoutMitigation(nil,
				&v1alpha1.TimeoutIncrement{Increment: metav1.Duration{Duration: 30 * time.Minute}},
			)
			Expect(result.Pipeline.Duration).To(Equal(30 * time.Minute))
		})

		It("should return current when mitigation is nil", func() {
			current := &tektonv1.TimeoutFields{Pipeline: &metav1.Duration{Duration: 1 * time.Hour}}
			Expect(retry.ApplyPipelineTimeoutMitigation(current, nil)).To(Equal(current))
		})
	})

	Context("MergeTaskRunSpecs", func() {
		It("should update matching task and preserve other fields", func() {
			original := []tektonv1.PipelineTaskRunSpec{{
				PipelineTaskName: "push-snapshot",
				Timeout:          &metav1.Duration{Duration: 5 * time.Minute},
			}}
			result := retry.MergeTaskRunSpecs(original, tektonv1.PipelineTaskRunSpec{
				PipelineTaskName: "push-snapshot",
				ComputeResources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
				},
			})
			Expect(result).To(HaveLen(1))
			Expect(result[0].ComputeResources).NotTo(BeNil())
			Expect(result[0].Timeout).NotTo(BeNil())
		})

		It("should append when no matching task exists", func() {
			result := retry.MergeTaskRunSpecs(
				[]tektonv1.PipelineTaskRunSpec{{PipelineTaskName: "collect-data"}},
				tektonv1.PipelineTaskRunSpec{PipelineTaskName: "update-cr-status", Timeout: &metav1.Duration{Duration: 10 * time.Minute}},
			)
			Expect(result).To(HaveLen(2))
			Expect(result[1].PipelineTaskName).To(Equal("update-cr-status"))
		})

		It("should clear ComputeResources when overriding with StepSpecs", func() {
			result := retry.MergeTaskRunSpecs(
				[]tektonv1.PipelineTaskRunSpec{{
					PipelineTaskName: "collect-data",
					ComputeResources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")},
					},
				}},
				tektonv1.PipelineTaskRunSpec{PipelineTaskName: "collect-data", StepSpecs: []tektonv1.TaskRunStepSpec{{Name: "collect-data"}}},
			)
			Expect(result[0].StepSpecs).To(HaveLen(1))
			Expect(result[0].ComputeResources).To(BeNil())
		})

		It("should clear StepSpecs when overriding with ComputeResources", func() {
			result := retry.MergeTaskRunSpecs(
				[]tektonv1.PipelineTaskRunSpec{{
					PipelineTaskName: "push-snapshot",
					StepSpecs:        []tektonv1.TaskRunStepSpec{{Name: "push-snapshot"}},
				}},
				tektonv1.PipelineTaskRunSpec{
					PipelineTaskName: "push-snapshot",
					ComputeResources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
					},
				},
			)
			Expect(result[0].ComputeResources).NotTo(BeNil())
			Expect(result[0].StepSpecs).To(BeNil())
		})

		It("should not mutate the original slice", func() {
			original := []tektonv1.PipelineTaskRunSpec{{
				PipelineTaskName: "update-cr-status",
				Timeout:          &metav1.Duration{Duration: 5 * time.Minute},
			}}
			retry.MergeTaskRunSpecs(original, tektonv1.PipelineTaskRunSpec{
				PipelineTaskName: "update-cr-status",
				Timeout:          &metav1.Duration{Duration: 10 * time.Minute},
			})
			Expect(original[0].Timeout.Duration).To(Equal(5 * time.Minute))
		})

		It("should handle nil original", func() {
			result := retry.MergeTaskRunSpecs(nil, tektonv1.PipelineTaskRunSpec{PipelineTaskName: "push-snapshot"})
			Expect(result).To(HaveLen(1))
		})
	})
})
