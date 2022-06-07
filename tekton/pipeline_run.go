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
	"encoding/json"
	"fmt"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"unicode"
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

// ReleasePipelineRun is a PipelineRun alias, so we can add new methods to it in this file.
type ReleasePipelineRun struct {
	tektonv1beta1.PipelineRun
}

// NewReleasePipelineRun creates an empty PipelineRun in the given namespace. The name will be autogenerated,
// using the prefix passed as an argument to the function.
func NewReleasePipelineRun(prefix, namespace string) *ReleasePipelineRun {
	pipelineRun := tektonv1beta1.PipelineRun{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: prefix + "-",
			Namespace:    namespace,
		},
		Spec: tektonv1beta1.PipelineRunSpec{},
	}

	return &ReleasePipelineRun{pipelineRun}
}

// AsPipelineRun casts the ReleasePipelineRun to PipelineRun, so it can be used in the Kubernetes client.
func (r *ReleasePipelineRun) AsPipelineRun() *tektonv1beta1.PipelineRun {
	return &r.PipelineRun
}

// WithExtraParam adds an extra param to the release PipelineRun. If the parameter is not part of the Pipeline
// definition, it will be silently ignored.
func (r *ReleasePipelineRun) WithExtraParam(name string, value tektonv1beta1.ArrayOrString) *ReleasePipelineRun {
	r.Spec.Params = append(r.Spec.Params, tektonv1beta1.Param{
		Name:  name,
		Value: value,
	})

	return r
}

// WithOwner set's owner annotations to the release PipelineRun.
func (r *ReleasePipelineRun) WithOwner(release *v1alpha1.Release) *ReleasePipelineRun {
	_ = libhandler.SetOwnerAnnotations(release, r)

	return r
}

// WithReleaseLabels adds Release name and namespace as labels to the release PipelineRun.
func (r *ReleasePipelineRun) WithReleaseLabels(releaseName, releaseNamespace string) *ReleasePipelineRun {
	r.ObjectMeta.Labels = map[string]string{
		PipelinesTypeLabel:    PipelineTypeRelease,
		ReleaseNameLabel:      releaseName,
		ReleaseWorkspaceLabel: releaseNamespace,
	}

	return r
}

// WithApplicationSnapshot adds a param containing the ApplicationSnapshot as a json string
// to the release PipelineRun.
func (r *ReleasePipelineRun) WithApplicationSnapshot(snapshot *v1alpha1.ApplicationSnapshot) *ReleasePipelineRun {
	// We ignore the error here because none should be raised when marshalling the spec of a CRD.
	// If we end up deciding it is useful, we will need to pass the errors trough the chain and
	// add something like a `Complete` function that returns the final object and error.
	snapshotString, _ := json.Marshal(snapshot.Spec)

	// Get snapshot.Kind runes to make the first letter lowercase
	snapshotRunes := []rune(snapshot.Kind)
	snapshotRunes[0] = unicode.ToLower(snapshotRunes[0])

	r.WithExtraParam(string(snapshotRunes), tektonv1beta1.ArrayOrString{
		Type:      tektonv1beta1.ParamTypeString,
		StringVal: string(snapshotString),
	})

	return r
}

// WithReleaseStrategy adds Pipeline reference, params, and the policy to the release PipelineRun.
func (r *ReleasePipelineRun) WithReleaseStrategy(strategy *v1alpha1.ReleaseStrategy) *ReleasePipelineRun {
	r.Spec.PipelineRef = &tektonv1beta1.PipelineRef{
		Name:   strategy.Spec.Pipeline,
		Bundle: strategy.Spec.Bundle,
	}

	valueType := tektonv1beta1.ParamTypeString

	if strategy.Spec.Policy != "" {
		r.WithExtraParam("policy", tektonv1beta1.ArrayOrString{
			Type:      valueType,
			StringVal: strategy.Spec.Policy,
		})
	}

	for _, param := range strategy.Spec.Params {
		if len(param.Values) > 0 {
			valueType = tektonv1beta1.ParamTypeArray
		}

		r.WithExtraParam(param.Name, tektonv1beta1.ArrayOrString{
			Type:      valueType,
			StringVal: param.Value,
			ArrayVal:  param.Values,
		})
	}

	if strategy.Spec.PersistentVolumeClaim == "" {
		r.WithWorkspace(os.Getenv("DEFAULT_RELEASE_WORKSPACE_NAME"), os.Getenv("DEFAULT_RELEASE_PVC"))
	} else {
		r.WithWorkspace(os.Getenv("DEFAULT_RELEASE_WORKSPACE_NAME"), strategy.Spec.PersistentVolumeClaim)
	}

	r.WithServiceAccount(strategy.Spec.ServiceAccount)

	return r
}

// WithServiceAccount adds a reference to the service account to be used to gain elevated privileges during the
// execution of the different Pipeline tasks.
func (r *ReleasePipelineRun) WithServiceAccount(serviceAccount string) *ReleasePipelineRun {
	r.Spec.ServiceAccountName = serviceAccount

	return r
}

// WithWorkspace adds a workspace to the PipelineRun using the given name and PersistentVolumeClaim.
// If any of those values is empty, no workspace will be added.
func (r *ReleasePipelineRun) WithWorkspace(name, persistentVolumeClaim string) *ReleasePipelineRun {
	if name == "" || persistentVolumeClaim == "" {
		return r
	}

	r.Spec.Workspaces = append(r.Spec.Workspaces, tektonv1beta1.WorkspaceBinding{
		Name: name,
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: persistentVolumeClaim,
		},
	})

	return r
}
