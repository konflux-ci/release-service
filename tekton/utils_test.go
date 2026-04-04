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
	"github.com/konflux-ci/release-service/metadata"
	"github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
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
			Expect(info.IsTimeout).To(BeFalse())
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
			Expect(info.IsTimeout).To(BeFalse())
			Expect(info.TaskName).To(Equal("update-status"))
			Expect(info.StepName).To(Equal("update-cr-status"))
		})

		It("should detect pipeline-level timeout", func() {
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
			Expect(info.IsTimeout).To(BeTrue())
			Expect(info.IsOOMKill).To(BeFalse())
			Expect(info.TaskName).To(Equal("publish-data"))
			Expect(info.SuccessfulTasks).To(Equal(1))
		})

		It("should detect task-level timeout", func() {
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
								{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse, Reason: string(tektonv1.TaskRunReasonTimedOut)},
							},
						},
					},
				},
			}

			info := GetPipelineRunFailureInfo(pipelineRun, taskRuns)
			Expect(info.IsTimeout).To(BeTrue())
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
			Expect(info.IsTimeout).To(BeFalse())
			Expect(info.TaskName).To(Equal("update-status"))
			Expect(info.StepName).To(Equal("update-cr-status"))
			Expect(info.SuccessfulTasks).To(Equal(2))
		})
	})
})
