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

package retry

import (
	"math"
	"strconv"
	"time"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konflux-ci/release-service/api/v1alpha1"
)

// ApplyMemoryMitigation multiplies memory limits and requests by the configured multiplier,
// capping at MaxComputeResources. Returns nil when there is nothing to mitigate.
func ApplyMemoryMitigation(current *corev1.ResourceRequirements, mitigation *v1alpha1.MemoryMitigation) *corev1.ResourceRequirements {
	if current == nil || mitigation == nil {
		return nil
	}

	multiplier, err := strconv.ParseFloat(mitigation.Multiplier, 64)
	if err != nil || multiplier <= 0 {
		return nil
	}

	result := current.DeepCopy()
	hasMemory := false

	// multiply the memory limit and cap at the configured max.
	if memoryLimit, ok := result.Limits[corev1.ResourceMemory]; ok {
		newLimit := multiplyQuantity(memoryLimit, multiplier)
		if mitigation.MaxComputeResources != nil {
			if maxLimit, ok := mitigation.MaxComputeResources.Limits[corev1.ResourceMemory]; ok && newLimit.Cmp(maxLimit) > 0 {
				newLimit = maxLimit
			}
		}
		result.Limits[corev1.ResourceMemory] = newLimit
		hasMemory = true
	}

	// multiply the memory request and cap at the configured max.
	if memoryRequest, ok := result.Requests[corev1.ResourceMemory]; ok {
		newRequest := multiplyQuantity(memoryRequest, multiplier)
		if mitigation.MaxComputeResources != nil {
			if maxRequest, ok := mitigation.MaxComputeResources.Requests[corev1.ResourceMemory]; ok && newRequest.Cmp(maxRequest) > 0 {
				newRequest = maxRequest
			}
		}
		result.Requests[corev1.ResourceMemory] = newRequest
		hasMemory = true
	}

	// if there was no memory limit or request, return nil.
	if !hasMemory {
		return nil
	}

	return result
}

// ApplyTaskTimeoutMitigation adds the increment to the task timeout and ensures the pipeline-level
// timeouts can still accommodate it.
func ApplyTaskTimeoutMitigation(current *metav1.Duration, pipelineTimeouts *tektonv1.TimeoutFields, mitigation *v1alpha1.TimeoutIncrement) (*metav1.Duration, *tektonv1.TimeoutFields) {
	if mitigation == nil {
		return current, pipelineTimeouts
	}

	newTaskTimeout := addCappedDuration(current, mitigation)

	adjustedTimeouts := copyTimeoutFields(pipelineTimeouts)

	// bump tasks timeout if the new task timeout exceeds it
	if adjustedTimeouts.Tasks != nil && newTaskTimeout > adjustedTimeouts.Tasks.Duration {
		adjustedTimeouts.Tasks = &metav1.Duration{Duration: newTaskTimeout}
	}

	// bump pipeline timeout if it no longer covers tasks and finally
	enforcePipelineCeiling(adjustedTimeouts)

	return &metav1.Duration{Duration: newTaskTimeout}, adjustedTimeouts
}

// ApplyPipelineTimeoutMitigation adds the increment to the pipeline timeout, capping at MaxTimeout.
func ApplyPipelineTimeoutMitigation(current *tektonv1.TimeoutFields, mitigation *v1alpha1.TimeoutIncrement) *tektonv1.TimeoutFields {
	if mitigation == nil {
		return current
	}
	adjustedTimeouts := copyTimeoutFields(current)
	newPipelineTimeout := addCappedDuration(adjustedTimeouts.Pipeline, mitigation)
	adjustedTimeouts.Pipeline = &metav1.Duration{Duration: newPipelineTimeout}

	return adjustedTimeouts
}

// MergeTaskRunSpecs merges overrides into the original by PipelineTaskName.
// Existing entries are updated in place, new ones are appended.
func MergeTaskRunSpecs(original []tektonv1.PipelineTaskRunSpec, overrides ...tektonv1.PipelineTaskRunSpec) []tektonv1.PipelineTaskRunSpec {
	result := make([]tektonv1.PipelineTaskRunSpec, len(original))
	for i := range original {
		result[i] = *original[i].DeepCopy()
	}

	for _, override := range overrides {
		// skip overrides with no task name to avoid invalid tekton specs.
		if override.PipelineTaskName == "" {
			continue
		}

		found := false
		// find the matching task by name and apply the override.
		for i := range result {
			if result[i].PipelineTaskName != override.PipelineTaskName {
				continue
			}
			found = true

			// tekton rejects a TaskRunSpec with both StepSpecs and ComputeResources
			// so setting one has to clear the other.
			if len(override.StepSpecs) > 0 {
				result[i].StepSpecs = override.StepSpecs
				result[i].ComputeResources = nil
			}
			if override.ComputeResources != nil {
				result[i].ComputeResources = override.ComputeResources
				result[i].StepSpecs = nil
			}
			if override.Timeout != nil {
				result[i].Timeout = override.Timeout
			}
			break
		}
		// task not in the original list so append it as a new entry.
		if !found {
			result = append(result, override)
		}
	}

	return result
}

// addCappedDuration adds the mitigation increment to the base duration, capping at MaxTimeout.
func addCappedDuration(base *metav1.Duration, mitigation *v1alpha1.TimeoutIncrement) time.Duration {
	var duration time.Duration
	if base != nil {
		duration = base.Duration
	}

	// Add the increment to the base duration, capping at MaxTimeout.
	duration += mitigation.Increment.Duration
	if mitigation.MaxTimeout != nil && duration > mitigation.MaxTimeout.Duration {
		duration = mitigation.MaxTimeout.Duration
	}

	return duration
}

// copyTimeoutFields returns a copy of the timeout fields.
func copyTimeoutFields(timeouts *tektonv1.TimeoutFields) *tektonv1.TimeoutFields {
	copied := &tektonv1.TimeoutFields{}
	if timeouts != nil {
		copied.Pipeline = timeouts.Pipeline
		copied.Tasks = timeouts.Tasks
		copied.Finally = timeouts.Finally
	}
	return copied
}

// enforcePipelineCeiling ensures pipeline greater than or equal to tasks plus finally.
func enforcePipelineCeiling(timeouts *tektonv1.TimeoutFields) {
	if timeouts.Pipeline == nil {
		return
	}

	// get the current tasks and finally durations if set.
	var tasksDuration, finallyDuration time.Duration
	if timeouts.Tasks != nil {
		tasksDuration = timeouts.Tasks.Duration
	}
	if timeouts.Finally != nil {
		finallyDuration = timeouts.Finally.Duration
	}

	// tekton rejects a PipelineRun where pipeline is less than tasks plus finally.
	needed := tasksDuration + finallyDuration
	if timeouts.Pipeline.Duration < needed {
		timeouts.Pipeline = &metav1.Duration{Duration: needed}
	}
}

// multiplyQuantity scales a resource quantity by a float and rounds up.
func multiplyQuantity(quantity resource.Quantity, multiplier float64) resource.Quantity {
	return *resource.NewQuantity(int64(math.Ceil(float64(quantity.Value())*multiplier)), quantity.Format)
}
