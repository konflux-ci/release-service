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

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
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
// the Pipeline.
// +kubebuilder:object:generate=true
type ParameterizedPipeline struct {
	Pipeline `json:",inline"`

	// Params is a slice of parameters for a given resolver
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

// GetTektonParams returns the ParameterizedPipeline []Param as []tektonv1.Param.
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

// resolveRevisionToSHA is a helper function that resolves a revision to an SHA if needed.
// If the revision is already an SHA, returns it directly. Otherwise, resolves via GitHub API.
func resolveRevisionToSHA(ctx context.Context, url, revision string) (string, error) {
	if isSHA(revision) {
		return revision, nil
	}

	resolvedSHA, err := ResolveBranchToSHA(ctx, url, revision)
	if err != nil {
		return "", err // Requeue on any resolution failure
	}

	return resolvedSHA, nil
}

// ResolveGitReferenceToCommitSHA resolves a git reference (branch name) to a commit SHA
// using the GitHub API. Returns the commit SHA or an error if resolution fails.
// If the revision is already an SHA, returns it directly without making an API call.
func (pr *PipelineRef) ResolveGitReferenceToCommitSHA(ctx context.Context) (string, error) {
	if pr.Resolver != "git" {
		return "", fmt.Errorf("can only resolve git references, got resolver: %s", pr.Resolver)
	}

	url, revision, _, err := pr.GetGitResolverParams()
	if err != nil {
		return "", fmt.Errorf("failed to get git resolver params: %w", err)
	}

	return resolveRevisionToSHA(ctx, url, revision)
}

// CreatePipelineRefWithResolvedSHA creates a new PipelineRef with the revision resolved to a commit SHA.
func (pr *PipelineRef) CreatePipelineRefWithResolvedSHA(ctx context.Context) (*PipelineRef, error) {
	url, revision, _, err := pr.GetGitResolverParams()
	if err != nil {
		return nil, err
	}

	commitSHA, err := resolveRevisionToSHA(ctx, url, revision)
	if err != nil {
		return nil, err
	}

	newRef := &PipelineRef{
		Resolver: pr.Resolver,
		Params:   make([]Param, len(pr.Params)),
	}

	for i, param := range pr.Params {
		if param.Name == "revision" {
			newRef.Params[i] = Param{
				Name:  "revision",
				Value: commitSHA,
			}
		} else {
			newRef.Params[i] = param
		}
	}

	return newRef, nil
}
