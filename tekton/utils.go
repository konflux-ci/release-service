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
