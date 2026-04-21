/*
Copyright 2022 Red Hat Inc.

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

package tekton

import (
	"time"

	"github.com/konflux-ci/release-service/metadata"
	"github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var _ = Describe("Utils", Ordered, func() {
	When("isReleasePipelineRun is called", func() {
		It("should return false when the PipelineRun is not one of the supported ones", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(isReleasePipelineRun(pipelineRun)).To(BeFalse())
		})

		It("should return true when the PipelineRun is of type 'tenant-collectors'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.TenantCollectorsPipelineType.String()}).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(isReleasePipelineRun(pipelineRun)).To(BeTrue())
		})

		It("should return true when the PipelineRun is of type 'managed-collectors'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedCollectorsPipelineType.String()}).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(isReleasePipelineRun(pipelineRun)).To(BeTrue())
		})

		It("should return true when the PipelineRun is of type 'final'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.FinalPipelineType.String()}).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(isReleasePipelineRun(pipelineRun)).To(BeTrue())
		})

		It("should return true when the PipelineRun is of type 'managed'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(isReleasePipelineRun(pipelineRun)).To(BeTrue())
		})

		It("should return true when the PipelineRun is of type 'tenant'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.TenantPipelineType.String()}).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(isReleasePipelineRun(pipelineRun)).To(BeTrue())
		})
	})

	When("hasPipelineSucceeded is called", func() {
		It("should return false when the PipelineRun has not succeeded", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(hasPipelineSucceeded(pipelineRun)).To(BeFalse())
		})

		It("should return true when the PipelineRun is of type 'managed'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			pipelineRun.Status.MarkSucceeded("", "")
			Expect(hasPipelineSucceeded(pipelineRun)).To(BeTrue())
		})
	})

	When("hasDeletionTimestampChanged is called", func() {
		It("should return false when both objects are nil", func() {
			Expect(hasDeletionTimestampChanged(nil, nil)).To(BeFalse())
		})

		It("should return false when the old object is nil", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(hasDeletionTimestampChanged(nil, pipelineRun)).To(BeFalse())
		})

		It("should return false when the new object is nil", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(hasDeletionTimestampChanged(pipelineRun, nil)).To(BeFalse())
		})

		It("should return false when both objects have no deletionTimestamp", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(hasDeletionTimestampChanged(oldPipelineRun, newPipelineRun)).To(BeFalse())
		})

		It("should return true when deletionTimestamp is added to new object", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			now := metav1.Now()
			newPipelineRun.DeletionTimestamp = &now
			Expect(hasDeletionTimestampChanged(oldPipelineRun, newPipelineRun)).To(BeTrue())
		})
	})

	When("IsPipelineRunDone is called", func() {
		It("should return false when PipelineRun is nil", func() {
			Expect(IsPipelineRunDone(nil)).To(BeFalse())
		})

		It("should return false when PipelineRun is still running", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			pipelineRun.Status.MarkRunning("Test", "Running")
			Expect(IsPipelineRunDone(pipelineRun)).To(BeFalse())
		})

		It("should return true when PipelineRun has succeeded", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			pipelineRun.Status.MarkSucceeded("Test", "Succeeded")
			Expect(IsPipelineRunDone(pipelineRun)).To(BeTrue())
		})

		It("should return true when PipelineRun has failed", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			pipelineRun.Status.MarkFailed("Test", "Failed")
			Expect(IsPipelineRunDone(pipelineRun)).To(BeTrue())
		})

		It("should return true when PipelineRun has deletionTimestamp even if status is Unknown", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			pipelineRun.Status.MarkRunning("Test", "Running")
			now := metav1.Now()
			pipelineRun.DeletionTimestamp = &now
			Expect(IsPipelineRunDone(pipelineRun)).To(BeTrue())
		})
	})

	When("hasFinalizersChanged is called", func() {
		It("should return false when both objects are nil", func() {
			Expect(hasFinalizersChanged(nil, nil)).To(BeFalse())
		})

		It("should return false when the old object is nil", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(hasFinalizersChanged(nil, pipelineRun)).To(BeFalse())
		})

		It("should return false when the new object is nil", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(hasFinalizersChanged(pipelineRun, nil)).To(BeFalse())
		})

		It("should return false when the finalizers are identical", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("finalizer1").
				WithFinalizer("finalizer2").
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("finalizer1").
				WithFinalizer("finalizer2").
				Build()
			Expect(err).NotTo(HaveOccurred())

			Expect(hasFinalizersChanged(oldPipelineRun, newPipelineRun)).To(BeFalse())
		})

		It("should return true when finalizers are added to the new object", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("finalizer1").
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("finalizer1").
				WithFinalizer("finalizer2").
				Build()
			Expect(err).NotTo(HaveOccurred())

			Expect(hasFinalizersChanged(oldPipelineRun, newPipelineRun)).To(BeTrue())
		})

		It("should return true when finalizers are removed from the new object", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("finalizer1").
				WithFinalizer("finalizer2").
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("finalizer1").
				Build()
			Expect(err).NotTo(HaveOccurred())

			Expect(hasFinalizersChanged(oldPipelineRun, newPipelineRun)).To(BeTrue())
		})

		It("should return true when finalizers are reordered in the new object", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("finalizer1").
				WithFinalizer("finalizer2").
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("finalizer2").
				WithFinalizer("finalizer1").
				Build()
			Expect(err).NotTo(HaveOccurred())

			Expect(hasFinalizersChanged(oldPipelineRun, newPipelineRun)).To(BeTrue())
		})

		It("should return false when both objects have no finalizers", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())

			Expect(hasFinalizersChanged(oldPipelineRun, newPipelineRun)).To(BeFalse())
		})
	})

	When("GetPipelineRunFailureInfo is called", func() {
		It("should return empty info when PipelineRun is nil", func() {
			info := GetPipelineRunFailureInfo(nil, nil)
			Expect(info).NotTo(BeNil())
			Expect(info.TaskName).To(BeEmpty())
			Expect(info.IsOOMKill).To(BeFalse())
			Expect(info.IsPipelineRunTimeout).To(BeFalse())
			Expect(info.IsTaskRunTimeout).To(BeFalse())
		})

		It("should return empty info when taskRuns is nil", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkFailed("Failed", "something broke")
			info := GetPipelineRunFailureInfo(pipelineRun, nil)
			Expect(info).NotTo(BeNil())
			Expect(info.TaskName).To(BeEmpty())
			Expect(info.IsOOMKill).To(BeFalse())
		})

		It("should detect OOMKill from step termination reason", func() {
			pipelineRun := &tektonv1.PipelineRun{
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{Name: "task-run-1", PipelineTaskName: "update-status"},
						},
					},
				},
			}
			pipelineRun.Status.MarkFailed("Failed", "update-status failed")

			taskRuns := []tektonv1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "task-run-1"},
					Status: tektonv1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{
								{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse, Reason: "Failed"},
							},
						},
						TaskRunStatusFields: tektonv1.TaskRunStatusFields{
							Steps: []tektonv1.StepState{
								{
									Name: "update-cr-status",
									ContainerState: corev1.ContainerState{
										Terminated: &corev1.ContainerStateTerminated{
											ExitCode: 137,
											Reason:   "OOMKilled",
										},
									},
								},
							},
						},
					},
				},
			}

			info := GetPipelineRunFailureInfo(pipelineRun, taskRuns)
			Expect(info.IsOOMKill).To(BeTrue())
			Expect(info.IsPipelineRunTimeout).To(BeFalse())
			Expect(info.IsTaskRunTimeout).To(BeFalse())
			Expect(info.TaskName).To(Equal("update-status"))
			Expect(info.StepName).To(Equal("update-cr-status"))
		})

		It("should detect PipelineRun timeout", func() {
			pipelineRun := &tektonv1.PipelineRun{
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{Name: "task-run-1", PipelineTaskName: "collect-data"},
							{Name: "task-run-2", PipelineTaskName: "publish-data"},
						},
					},
				},
			}
			pipelineRun.Status.MarkFailed(tektonv1.PipelineRunReasonTimedOut.String(), "timed out")

			taskRuns := []tektonv1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "task-run-1"},
					Status: tektonv1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{
								{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "task-run-2"},
					Status: tektonv1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{
								{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse, Reason: "Failed"},
							},
						},
					},
				},
			}

			info := GetPipelineRunFailureInfo(pipelineRun, taskRuns)
			Expect(info.IsPipelineRunTimeout).To(BeTrue())
			Expect(info.IsTaskRunTimeout).To(BeFalse())
			Expect(info.IsOOMKill).To(BeFalse())
			Expect(info.TaskName).To(Equal("publish-data"))
			Expect(info.SuccessfulTasks).To(Equal(1))
		})

		It("should detect TaskRun timeout", func() {
			pipelineRun := &tektonv1.PipelineRun{
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{Name: "task-run-1", PipelineTaskName: "publish-data"},
						},
					},
				},
			}
			pipelineRun.Status.MarkFailed("Failed", "task timed out")

			taskRuns := []tektonv1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "task-run-1"},
					Status: tektonv1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{
								{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse, Reason: tektonv1.TaskRunReasonTimedOut.String()},
							},
						},
					},
				},
			}

			info := GetPipelineRunFailureInfo(pipelineRun, taskRuns)
			Expect(info.IsTaskRunTimeout).To(BeTrue())
			Expect(info.IsPipelineRunTimeout).To(BeFalse())
			Expect(info.TaskName).To(Equal("publish-data"))
		})

		It("should detect generic error and count successful tasks", func() {
			pipelineRun := &tektonv1.PipelineRun{
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{Name: "task-run-1", PipelineTaskName: "collect-data"},
							{Name: "task-run-2", PipelineTaskName: "publish-data"},
							{Name: "task-run-3", PipelineTaskName: "update-status"},
						},
					},
				},
			}
			pipelineRun.Status.MarkFailed("Failed", "deploy failed")

			taskRuns := []tektonv1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "task-run-1"},
					Status: tektonv1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{
								{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "task-run-2"},
					Status: tektonv1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{
								{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "task-run-3"},
					Status: tektonv1.TaskRunStatus{
						Status: duckv1.Status{
							Conditions: duckv1.Conditions{
								{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse, Reason: "Failed"},
							},
						},
						TaskRunStatusFields: tektonv1.TaskRunStatusFields{
							Steps: []tektonv1.StepState{
								{
									Name: "update-cr-status",
									ContainerState: corev1.ContainerState{
										Terminated: &corev1.ContainerStateTerminated{
											ExitCode: 1,
											Reason:   "Error",
										},
									},
								},
							},
						},
					},
				},
			}

			info := GetPipelineRunFailureInfo(pipelineRun, taskRuns)
			Expect(info.IsOOMKill).To(BeFalse())
			Expect(info.IsPipelineRunTimeout).To(BeFalse())
			Expect(info.IsTaskRunTimeout).To(BeFalse())
			Expect(info.TaskName).To(Equal("update-status"))
			Expect(info.StepName).To(Equal("update-cr-status"))
			Expect(info.SuccessfulTasks).To(Equal(2))
		})
	})

	When("GetStepComputeResources is called", func() {
		It("should return nil when both PipelineRun and TaskRun are nil", func() {
			Expect(GetStepComputeResources(nil, nil, "push-snapshot", "push-snapshot")).To(BeNil())
		})

		It("should return step resources from TaskRun task spec", func() {
			expectedResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					TaskSpec: &tektonv1.TaskSpec{
						Steps: []tektonv1.Step{
							{
								Name:             "update-cr-status",
								ComputeResources: *expectedResources,
							},
						},
					},
				},
			}

			Expect(GetStepComputeResources(nil, taskRun, "update-status", "update-cr-status")).To(Equal(expectedResources))
		})

		It("should return nil when step is not found in TaskRun task spec", func() {
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					TaskSpec: &tektonv1.TaskSpec{
						Steps: []tektonv1.Step{
							{Name: "collect-data"},
						},
					},
				},
			}

			Expect(GetStepComputeResources(nil, taskRun, "push-snapshot", "push-snapshot")).To(BeNil())
		})

		It("should return PipelineRun step override over TaskRun task spec", func() {
			stepResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			}
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					TaskRunSpecs: []tektonv1.PipelineTaskRunSpec{
						{
							PipelineTaskName: "push-snapshot",
							StepSpecs: []tektonv1.TaskRunStepSpec{
								{
									Name:             "push-snapshot",
									ComputeResources: stepResources,
								},
							},
						},
					},
				},
			}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					TaskSpec: &tektonv1.TaskSpec{
						Steps: []tektonv1.Step{
							{
								Name: "push-snapshot",
								ComputeResources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
								},
							},
						},
					},
				},
			}

			Expect(GetStepComputeResources(pipelineRun, taskRun, "push-snapshot", "push-snapshot")).To(Equal(&stepResources))
		})

		It("should return PipelineRun task level resources over TaskRun task spec", func() {
			taskResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			}
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					TaskRunSpecs: []tektonv1.PipelineTaskRunSpec{
						{
							PipelineTaskName: "collect-data",
							ComputeResources: taskResources,
						},
					},
				},
			}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					TaskSpec: &tektonv1.TaskSpec{
						Steps: []tektonv1.Step{
							{
								Name: "collect-data",
								ComputeResources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
								},
							},
						},
					},
				},
			}

			Expect(GetStepComputeResources(pipelineRun, taskRun, "collect-data", "collect-data")).To(Equal(taskResources))
		})

		It("should fall back to TaskRun task spec when PipelineRun has no overrides", func() {
			expectedResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}
			pipelineRun := &tektonv1.PipelineRun{}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					TaskSpec: &tektonv1.TaskSpec{
						Steps: []tektonv1.Step{
							{
								Name:             "push-snapshot",
								ComputeResources: *expectedResources,
							},
						},
					},
				},
			}

			Expect(GetStepComputeResources(pipelineRun, taskRun, "push-snapshot", "push-snapshot")).To(Equal(expectedResources))
		})

		It("should fall back to TaskRun task spec when PipelineRun task has no resources", func() {
			expectedResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					TaskRunSpecs: []tektonv1.PipelineTaskRunSpec{
						{PipelineTaskName: "publish-data"},
					},
				},
			}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					TaskSpec: &tektonv1.TaskSpec{
						Steps: []tektonv1.Step{
							{
								Name:             "publish-data",
								ComputeResources: *expectedResources,
							},
						},
					},
				},
			}

			Expect(GetStepComputeResources(pipelineRun, taskRun, "publish-data", "publish-data")).To(Equal(expectedResources))
		})

		It("should fall back to TaskRun task spec when step not in StepSpecs override", func() {
			expectedResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					TaskRunSpecs: []tektonv1.PipelineTaskRunSpec{
						{
							PipelineTaskName: "update-status",
							StepSpecs: []tektonv1.TaskRunStepSpec{
								{
									Name: "run-script",
									ComputeResources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceMemory: resource.MustParse("512Mi"),
										},
									},
								},
							},
						},
					},
				},
			}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					TaskSpec: &tektonv1.TaskSpec{
						Steps: []tektonv1.Step{
							{
								Name:             "update-cr-status",
								ComputeResources: *expectedResources,
							},
						},
					},
				},
			}

			Expect(GetStepComputeResources(pipelineRun, taskRun, "update-status", "update-cr-status")).To(Equal(expectedResources))
		})
	})

	When("GetPipelineRunTimeouts is called", func() {
		It("should return nil when PipelineRun is nil", func() {
			Expect(GetPipelineRunTimeouts(nil)).To(BeNil())
		})

		It("should return nil when no timeouts are set", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			Expect(GetPipelineRunTimeouts(pipelineRun)).To(BeNil())
		})

		It("should return all timeout fields", func() {
			timeouts := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 2 * time.Hour},
				Tasks:    &metav1.Duration{Duration: 90 * time.Minute},
				Finally:  &metav1.Duration{Duration: 30 * time.Minute},
			}
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					Timeouts: timeouts,
				},
			}

			Expect(GetPipelineRunTimeouts(pipelineRun)).To(Equal(timeouts))
		})

		It("should return timeouts even when pipeline is only set", func() {
			timeouts := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
			}
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					Timeouts: timeouts,
				},
			}

			result := GetPipelineRunTimeouts(pipelineRun)
			Expect(result).To(Equal(timeouts))
			Expect(result.Tasks).To(BeNil())
			Expect(result.Finally).To(BeNil())
		})
	})

	When("GetTaskRunTimeout is called", func() {
		It("should return nil when both PipelineRun and TaskRun are nil", func() {
			Expect(GetTaskRunTimeout(nil, nil, "update-cr-status")).To(BeNil())
		})

		It("should return TaskRun spec timeout", func() {
			taskRunTimeout := &metav1.Duration{Duration: 2 * time.Minute}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					Timeout: taskRunTimeout,
				},
			}

			Expect(GetTaskRunTimeout(nil, taskRun, "update-cr-status")).To(Equal(taskRunTimeout))
		})

		It("should return PipelineRun per-task override over TaskRun spec timeout", func() {
			overrideTimeout := &metav1.Duration{Duration: 5 * time.Minute}
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					TaskRunSpecs: []tektonv1.PipelineTaskRunSpec{
						{
							PipelineTaskName: "update-cr-status",
							Timeout:          overrideTimeout,
						},
					},
				},
			}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					Timeout: &metav1.Duration{Duration: 2 * time.Minute},
				},
			}

			Expect(GetTaskRunTimeout(pipelineRun, taskRun, "update-cr-status")).To(Equal(overrideTimeout))
		})

		It("should fall back to TaskRun spec timeout when PipelineRun has no override", func() {
			taskRunTimeout := &metav1.Duration{Duration: 2 * time.Minute}
			pipelineRun := &tektonv1.PipelineRun{}
			taskRun := &tektonv1.TaskRun{
				Spec: tektonv1.TaskRunSpec{
					Timeout: taskRunTimeout,
				},
			}

			Expect(GetTaskRunTimeout(pipelineRun, taskRun, "publish-data")).To(Equal(taskRunTimeout))
		})

		It("should fall back to global tasks timeout when no per-task or TaskRun timeout exists", func() {
			tasksTimeout := &metav1.Duration{Duration: 90 * time.Minute}
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					Timeouts: &tektonv1.TimeoutFields{
						Tasks: tasksTimeout,
					},
				},
			}

			Expect(GetTaskRunTimeout(pipelineRun, nil, "update-cr-status")).To(Equal(tasksTimeout))
		})

		It("should return nil when no task level timeouts exist", func() {
			pipelineRun := &tektonv1.PipelineRun{
				Spec: tektonv1.PipelineRunSpec{
					Timeouts: &tektonv1.TimeoutFields{
						Pipeline: &metav1.Duration{Duration: 2 * time.Hour},
					},
				},
			}

			Expect(GetTaskRunTimeout(pipelineRun, nil, "update-cr-status")).To(BeNil())
		})
	})
})
