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

package tekton

import (
	"reflect"

	"github.com/konflux-ci/release-service/metadata"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// isReleasePipelineRun returns a boolean indicating whether the object passed is a Final, Managed or a Tenant Release PipelineRun.
func isReleasePipelineRun(object client.Object) bool {
	_, ok := object.(*tektonv1.PipelineRun)
	if !ok {
		return false
	}

	labelValue, found := object.GetLabels()[metadata.PipelinesTypeLabel]

	return found && (labelValue == metadata.TenantCollectorsPipelineType.String() ||
		labelValue == metadata.ManagedCollectorsPipelineType.String() ||
		labelValue == metadata.FinalPipelineType.String() ||
		labelValue == metadata.ManagedPipelineType.String() ||
		labelValue == metadata.TenantPipelineType.String())
}

// hasPipelineSucceeded returns a boolean indicating whether the PipelineRun succeeded or not.
// If the object passed to this function is not a PipelineRun, the function will return false.
func hasPipelineSucceeded(object client.Object) bool {
	if pipelineRun, ok := object.(*tektonv1.PipelineRun); ok {
		return !pipelineRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
	}

	return false
}

// hasFinalizersChanged returns true if the finalizers have changed between old and new objects.
// This helps detect when other controllers (like Tekton) are modifying finalizers during deletion.
func hasFinalizersChanged(oldObj, newObj client.Object) bool {
	if oldObj == nil || newObj == nil {
		return false
	}

	oldFinalizers := oldObj.GetFinalizers()
	newFinalizers := newObj.GetFinalizers()

	return !reflect.DeepEqual(oldFinalizers, newFinalizers)
}

// hasDeletionTimestampChanged returns true if the deletion timestamp has changed between old and new objects.
// This detects when a PipelineRun is marked for deletion.
func hasDeletionTimestampChanged(oldObj, newObj client.Object) bool {
	if oldObj == nil || newObj == nil {
		return false
	}
	oldTs := oldObj.GetDeletionTimestamp()
	newTs := newObj.GetDeletionTimestamp()
	return (oldTs == nil) != (newTs == nil)
}

// IsPipelineRunDone returns true if the PipelineRun has completed (succeeded or failed)
// or if it has been marked for deletion. This handles the edge case where a PipelineRun
// is deleted while still running and Tekton hasn't updated the status to finished.
func IsPipelineRunDone(pipelineRun *tektonv1.PipelineRun) bool {
	if pipelineRun == nil {
		return false
	}
	return pipelineRun.IsDone() || pipelineRun.GetDeletionTimestamp() != nil
}

// PipelineRunFailureInfo defines the failure details extracted from a PipelineRun.
type PipelineRunFailureInfo struct {
	TaskName             string
	StepName             string
	SuccessfulTasks      int
	IsPipelineRunTimeout bool
	IsTaskRunTimeout     bool
	IsOOMKill            bool
}

// GetPipelineRunFailureInfo returns the failure details from a failed PipelineRun and its child TaskRuns.
func GetPipelineRunFailureInfo(pipelineRun *tektonv1.PipelineRun, taskRuns []tektonv1.TaskRun) *PipelineRunFailureInfo {
	if pipelineRun == nil {
		return &PipelineRunFailureInfo{}
	}

	info := &PipelineRunFailureInfo{
		IsPipelineRunTimeout: isPipelineRunTimedOut(pipelineRun),
	}

	taskName, taskRun, successfulTasks := getFailedTaskRun(pipelineRun, taskRuns)
	info.TaskName = taskName
	info.SuccessfulTasks = successfulTasks

	if taskRun == nil {
		return info
	}

	info.IsTaskRunTimeout = isTaskRunTimedOut(taskRun)

	for _, step := range taskRun.Status.Steps {
		if step.Terminated != nil && step.Terminated.ExitCode != 0 {
			info.StepName = step.Name
			info.IsOOMKill = step.Terminated.Reason == "OOMKilled"
			break
		}
	}

	return info
}

// isPipelineRunTimedOut checks whether the PipelineRun failed due to PipelineRun timeout.
func isPipelineRunTimedOut(pipelineRun *tektonv1.PipelineRun) bool {
	condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
	return condition != nil && condition.IsFalse() &&
		condition.Reason == tektonv1.PipelineRunReasonTimedOut.String()
}

// isTaskRunTimedOut checks whether the TaskRun failed due to TaskRun timeout.
func isTaskRunTimedOut(taskRun *tektonv1.TaskRun) bool {
	condition := taskRun.Status.GetCondition(apis.ConditionSucceeded)
	return condition != nil && condition.IsFalse() &&
		condition.Reason == tektonv1.TaskRunReasonTimedOut.String()
}

// getFailedTaskRun returns the first failed TaskRun from the PipelineRun's child references,
// its pipeline task name, and the count of successful tasks.
func getFailedTaskRun(pipelineRun *tektonv1.PipelineRun, taskRuns []tektonv1.TaskRun) (taskName string, failedTaskRun *tektonv1.TaskRun, successfulTasks int) {
	taskRunMap := make(map[string]*tektonv1.TaskRun, len(taskRuns))
	for i := range taskRuns {
		taskRunMap[taskRuns[i].Name] = &taskRuns[i]
	}

	for _, childRef := range pipelineRun.Status.ChildReferences {
		// Skip non TaskRun children. Kind may be empty when unset so treat that as a TaskRun.
		if childRef.Kind != "" && childRef.Kind != "TaskRun" {
			continue
		}

		taskRun, exists := taskRunMap[childRef.Name]
		if !exists {
			continue
		}

		condition := taskRun.Status.GetCondition(apis.ConditionSucceeded)
		if condition == nil {
			continue
		}

		if condition.IsTrue() {
			successfulTasks++
			continue
		}

		if condition.IsFalse() {
			return childRef.PipelineTaskName, taskRun, successfulTasks
		}
	}

	return "", nil, successfulTasks
}

// GetStepComputeResources returns the compute resources for a step by checking
// PipelineRun overrides StepSpecs or ComputeResources first, then falling back
// to the TaskRun task spec. Returns nil if none are configured.
func GetStepComputeResources(pipelineRun *tektonv1.PipelineRun, taskRun *tektonv1.TaskRun, taskName, stepName string) *corev1.ResourceRequirements {
	if pipelineRun != nil {
		for _, taskRunSpec := range pipelineRun.Spec.TaskRunSpecs {
			// Find the TaskRunSpec override matching the failed task.
			if taskRunSpec.PipelineTaskName != taskName {
				continue
			}

			// Check per step overrides. StepSpecs only override a single step resources.
			for _, stepSpec := range taskRunSpec.StepSpecs {
				if stepSpec.Name == stepName {
					resources := stepSpec.ComputeResources
					return &resources
				}
			}

			// Check task level override. ComputeResources applies to all steps in the task.
			if taskRunSpec.ComputeResources != nil {
				return taskRunSpec.ComputeResources
			}
		}
	}

	// No PipelineRun override found. Fall back to the step definition in the TaskRun task spec.
	if taskRun != nil && taskRun.Spec.TaskSpec != nil {
		for _, step := range taskRun.Spec.TaskSpec.Steps {
			if step.Name == stepName {
				resources := step.ComputeResources
				return &resources
			}
		}
	}

	return nil
}

// GetPipelineRunTimeouts returns the timeout fields for a PipelineRun.
// Returns nil if the PipelineRun or its timeouts are not defined.
func GetPipelineRunTimeouts(pipelineRun *tektonv1.PipelineRun) *tektonv1.TimeoutFields {
	if pipelineRun == nil || pipelineRun.Spec.Timeouts == nil {
		return nil
	}

	return pipelineRun.Spec.Timeouts
}

// GetTaskRunTimeout returns the timeout for a task by checking PipelineRun
// per task overrides first, then the matching TaskRun timeout, then the global
// tasks timeout from the PipelineRun. Returns nil if no timeout is
// configured at any level.
func GetTaskRunTimeout(pipelineRun *tektonv1.PipelineRun, taskRun *tektonv1.TaskRun, taskName string) *metav1.Duration {
	// Check per task timeout override from the PipelineRun TaskRunSpecs.
	if pipelineRun != nil {
		for _, taskRunSpec := range pipelineRun.Spec.TaskRunSpecs {
			if taskRunSpec.PipelineTaskName == taskName && taskRunSpec.Timeout != nil {
				return taskRunSpec.Timeout
			}
		}
	}

	// No per task override. Check the TaskRun timeout which Tekton sets from the pipeline spec.
	if taskRun != nil && taskRun.Spec.Timeout != nil {
		return taskRun.Spec.Timeout
	}

	// Fall back to the PipelineRun global tasks timeout.
	if pipelineRun != nil && pipelineRun.Spec.Timeouts != nil {
		return pipelineRun.Spec.Timeouts.Tasks
	}

	return nil
}
