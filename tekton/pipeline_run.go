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
	"fmt"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineType represents a PipelineRun type within AppStudio
type PipelineType string

const (
	// pipelinesLabelPrefix is the prefix of the pipelines label
	pipelinesLabelPrefix = "pipelines.appstudio.openshift.io"

	// releaseLabelPrefix is the prefix of the release labels
	releaseLabelPrefix = "release.appstudio.openshift.io"

	//PipelineTypeRelease is the type for PipelineRuns created to run a release Pipeline
	PipelineTypeRelease = "release"
)

var (
	// PipelinesTypeLabel is the label used to describe the type of pipeline
	PipelinesTypeLabel = fmt.Sprintf("%s/%s", pipelinesLabelPrefix, "type")

	// ReleaseNameLabel is the label used to specify the name of the Release associated with the PipelineRun
	ReleaseNameLabel = fmt.Sprintf("%s/%s", releaseLabelPrefix, "name")

	// ReleaseWorkspaceLabel is the label used to specify the workspace of the Release associated with the PipelineRun
	ReleaseWorkspaceLabel = fmt.Sprintf("%s/%s", releaseLabelPrefix, "workspace")
)

// CreateReleasePipelineRun creates a PipelineRun from a given Release and ReleaseStrategy.
// Release details are added to the labels of the new PipelineRun to be able to reference it later on.
func CreateReleasePipelineRun(release *v1alpha1.Release, releaseStrategy *v1alpha1.ReleaseStrategy) *tektonv1beta1.PipelineRun {
	return &tektonv1beta1.PipelineRun{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: releaseStrategy.Name + "-",
			Labels: map[string]string{
				PipelinesTypeLabel:    PipelineTypeRelease,
				ReleaseNameLabel:      release.Name,
				ReleaseWorkspaceLabel: release.Namespace,
			},
			Namespace: releaseStrategy.Namespace,
		},
		Spec: tektonv1beta1.PipelineRunSpec{
			PipelineRef: &tektonv1beta1.PipelineRef{
				Name:   releaseStrategy.Spec.Pipeline,
				Bundle: releaseStrategy.Spec.Bundle,
			},
		},
	}
}
