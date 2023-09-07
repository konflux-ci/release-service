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
	"github.com/redhat-appstudio/release-service/metadata"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// isReleasePipelineRun returns a boolean indicating whether the object passed is a release PipelineRun or not.
func isReleasePipelineRun(object client.Object) bool {
	_, ok := object.(*tektonv1.PipelineRun)
	if !ok {
		return false
	}

	labelValue, found := object.GetLabels()[metadata.PipelinesTypeLabel]

	return found && labelValue == PipelineTypeRelease
}

// hasPipelineSucceeded returns a boolean indicating whether the PipelineRun succeeded or not.
// If the object passed to this function is not a PipelineRun, the function will return false.
func hasPipelineSucceeded(object client.Object) bool {
	if pipelineRun, ok := object.(*tektonv1.PipelineRun); ok {
		return !pipelineRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown()
	}

	return false
}
