/*
Copyright 2023.

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

package utils

import (
	"context"
	"fmt"
	"io"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Param defines the parameters for a given resolver in PipelineRef
type Param struct {
	// Name is the name of the parameter
	Name string `json:"name"`

	// Value is the value of the parameter
	Value string `json:"value"`
}

// PipelineRef represents a reference to a Pipeline using a resolver.
// +kubebuilder:object:generate=true
type PipelineRef struct {
	// Resolver is the name of a Tekton resolver to be used (e.g. git)
	Resolver string `json:"resolver"`

	// Params is a slice of parameters for a given resolver
	Params []Param `json:"params"`

	// OciStorage specifies the OCI repository where the Trusted Artifacts are stored.
	// This value is passed to the Pipeline as the "ociStorage" parameter.
	// If not set, the default value from the Pipeline definition will be used.
	// This field is intended for use in ReleasePlanAdmissions.
	// +optional
	OciStorage string `json:"ociStorage,omitempty"`
}

// Pipeline contains a reference to a Pipeline and the name of the service account to use while executing it.
// +kubebuilder:object:generate=true
type Pipeline struct {
	// PipelineRef is the reference to the Pipeline
	PipelineRef PipelineRef `json:"pipelineRef"`

	// ServiceAccountName is the ServiceAccount to use during the execution of the Pipeline
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// TaskRunSpecs is the PipelineTaskRunSpec to be used in the PipelineRun execution
	// +optional
	TaskRunSpecs []tektonv1.PipelineTaskRunSpec `json:"taskRunSpecs,omitempty"`

	// Timeouts defines the different Timeouts to use in the PipelineRun execution
	// +optional
	Timeouts tektonv1.TimeoutFields `json:"timeouts,omitempty"`
}

// ParameterizedPipeline is an extension of the Pipeline struct, adding an array of parameters that will be passed to
// the Pipeline. Used by ReleasePlan's TenantPipeline and FinalPipeline.
// +kubebuilder:object:generate=true
type ParameterizedPipeline struct {
	Pipeline `json:",inline"`

	// Params is a slice of parameters to be passed to the Pipeline
	// +optional
	Params []Param `json:"params,omitempty"`
}

// GetGitResolverParams returns the common parameters found in a Git resolver. That is url, revision and pathInRepo.
// If the PipelineRef doesn't use a git resolver this function will return an error.
func (pr *PipelineRef) GetGitResolverParams() (string, string, string, error) {
	if pr.Resolver != "git" {
		return "", "", "", fmt.Errorf("not a git ref")
	}

	var url, revision, pathInRepo string
	for _, param := range pr.Params {
		switch param.Name {
		case "url":
			url = param.Value
		case "revision":
			revision = param.Value
		case "pathInRepo":
			pathInRepo = param.Value
		}
	}

	return url, revision, pathInRepo, nil
}

// GetRevision returns the value of the revision param. If not found an error will be raised.
func (pr *PipelineRef) GetRevision() (string, error) {
	for _, param := range pr.Params {
		if param.Name == "revision" {
			return param.Value, nil
		}
	}

	return "", fmt.Errorf("no revision found")
}

// GetUrl returns the value of the url param. If not found an error will be raised.
func (pr *PipelineRef) GetUrl() (string, error) {
	for _, param := range pr.Params {
		if param.Name == "url" {
			return param.Value, nil
		}
	}

	return "", fmt.Errorf("no url found")
}

// ToTektonPipelineRef converts a PipelineRef object to Tekton's own PipelineRef type and returns it.
func (pr *PipelineRef) ToTektonPipelineRef() *tektonv1.PipelineRef {
	params := tektonv1.Params{}

	for _, p := range pr.Params {
		params = append(params, tektonv1.Param{
			Name: p.Name,
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: p.Value,
			},
		})
	}

	tektonPipelineRef := &tektonv1.PipelineRef{
		ResolverRef: tektonv1.ResolverRef{
			Resolver: tektonv1.ResolverName(pr.Resolver),
			Params:   params,
		},
	}

	return tektonPipelineRef
}

// GetOciStorageParam returns the ociStorage parameter as a Tekton param slice.
// Returns an empty slice if OciStorage is not set.
func (pr *PipelineRef) GetOciStorageParam() []tektonv1.Param {
	if pr.OciStorage != "" {
		return []tektonv1.Param{
			{
				Name: "ociStorage",
				Value: tektonv1.ParamValue{
					Type:      tektonv1.ParamTypeString,
					StringVal: pr.OciStorage,
				},
			},
		}
	}
	return []tektonv1.Param{}
}

// GetOciStorageParam returns the ociStorage parameter from the Pipeline's PipelineRef.
// Used by the release adapter to pass the ociStorage parameter to the managed PipelineRun.
func (p *Pipeline) GetOciStorageParam() []tektonv1.Param {
	return p.PipelineRef.GetOciStorageParam()
}

// GetTektonParams returns the ParameterizedPipeline's Params as []tektonv1.Param.
// This is used by ReleasePlan's TenantPipeline and FinalPipeline.
func (prp *ParameterizedPipeline) GetTektonParams() []tektonv1.Param {
	params := []tektonv1.Param{}

	for _, param := range prp.Params {
		params = append(params, tektonv1.Param{
			Name: param.Name,
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: param.Value,
			},
		})
	}

	return params
}

// IsClusterScoped returns whether the PipelineRef uses a cluster resolver or not.
func (pr *PipelineRef) IsClusterScoped() bool {
	return pr.Resolver == "cluster"
}

// maxConditionMessageLength is the maximum length for a Kubernetes condition message.
// This limit is enforced by the API server validation (MaxLength=32768).
const maxConditionMessageLength = 31000

// GetFailedTaskRunLogs returns the logs from the first failed TaskRun in the PipelineRun.
// If no failed TaskRun is found or logs cannot be retrieved, it returns an empty string.
func GetFailedTaskRunLogs(ctx context.Context, cli client.Client, pipelineRun *tektonv1.PipelineRun) string {
	if pipelineRun == nil {
		return ""
	}

	// Fetch all TaskRuns for this PipelineRun in a single API call
	taskRunList := &tektonv1.TaskRunList{}
	err := cli.List(ctx, taskRunList,
		client.InNamespace(pipelineRun.Namespace),
		client.MatchingLabels{
			"tekton.dev/pipelineRun": pipelineRun.Name,
		},
	)
	if err != nil {
		return ""
	}

	// Build a map for quick lookup by name
	taskRunMap := make(map[string]*tektonv1.TaskRun, len(taskRunList.Items))
	for i := range taskRunList.Items {
		taskRunMap[taskRunList.Items[i].Name] = &taskRunList.Items[i]
	}

	// Iterate in the same order as ChildReferences to maintain consistent behavior
	for _, childRef := range pipelineRun.Status.ChildReferences {
		if childRef.Kind != "TaskRun" {
			continue
		}

		taskRun, exists := taskRunMap[childRef.Name]
		if !exists {
			continue
		}

		condition := taskRun.Status.GetCondition(apis.ConditionSucceeded)
		if condition != nil && condition.IsFalse() {
			prefix := fmt.Sprintf("task %s failed: ", childRef.PipelineTaskName)
			maxLen := maxConditionMessageLength - len(prefix)
			truncationMarker := "...(truncated)\n"

			message := getTaskRunLogs(ctx, taskRun)
			if message == "" {
				message = condition.Message
			}

			if len(message) > maxLen {
				message = truncationMarker + message[len(message)-maxLen+len(truncationMarker):]
			}
			return prefix + message
		}
	}

	return ""
}

// getTaskRunLogs fetches the logs from the pod associated with the TaskRun.
func getTaskRunLogs(ctx context.Context, taskRun *tektonv1.TaskRun) string {
	if taskRun.Status.PodName == "" {
		return ""
	}

	config := ctrl.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return ""
	}

	req := clientset.CoreV1().Pods(taskRun.Namespace).GetLogs(taskRun.Status.PodName, &corev1.PodLogOptions{})
	stream, err := req.Stream(ctx)
	if err != nil {
		return ""
	}
	defer stream.Close()

	logs, err := io.ReadAll(stream)
	if err != nil {
		return ""
	}

	return string(logs)
}
