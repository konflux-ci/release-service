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
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Pipeline", func() {
	var (
		clusterRef PipelineRef
		gitRef     PipelineRef
		bundleRef  PipelineRef
	)

	BeforeEach(func() {
		clusterRef = PipelineRef{
			Resolver: "cluster",
			Params: []Param{
				{Name: "kind", Value: "pipeline"},
				{Name: "name", Value: "my-cluster-pipeline"},
				{Name: "namespace", Value: "my-namespace"},
			},
		}
		gitRef = PipelineRef{
			Resolver: "git",
			Params: []Param{
				{Name: "url", Value: "my-git-url"},
				{Name: "revision", Value: "my-revision"},
				{Name: "pathInRepo", Value: "my-path-in-repo"},
			},
		}
		bundleRef = PipelineRef{
			Resolver: "bundles",
			Params: []Param{
				{Name: "bundle", Value: "my-bundle"},
				{Name: "name", Value: "my-pipeline"},
				{Name: "kind", Value: "pipeline"},
			},
		}
	})

	When("GetGitResolverParams method is called", func() {
		It("should return all the common parameters", func() {
			url, revision, pathInRepo, err := gitRef.GetGitResolverParams()
			Expect(url).To(Equal("my-git-url"))
			Expect(revision).To(Equal("my-revision"))
			Expect(pathInRepo).To(Equal("my-path-in-repo"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if a git resolver is not used", func() {
			_, _, _, err := bundleRef.GetGitResolverParams()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not a git ref"))
		})
	})

	When("GetUrl method is called", func() {
		It("should return the url if it exists", func() {
			url, err := gitRef.GetUrl()
			Expect(url).To(Equal("my-git-url"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return the url if it does not exist", func() {
			url, err := bundleRef.GetUrl()
			Expect(url).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})
	})

	When("GetRevision method is called", func() {
		It("should return the revision if it exists", func() {
			revision, err := gitRef.GetRevision()
			Expect(revision).To(Equal("my-revision"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return the revision if it does not exist", func() {
			revision, err := bundleRef.GetRevision()
			Expect(revision).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})
	})

	When("ToTektonPipelineRef method is called", func() {
		It("should return Tekton PipelineRef representation of the PipelineRef", func() {
			ref := clusterRef.ToTektonPipelineRef()
			Expect(string(ref.ResolverRef.Resolver)).To(Equal("cluster"))
			params := ref.ResolverRef.Params
			Expect(params[0].Name).To(Equal("kind"))
			Expect(params[0].Value.StringVal).To(Equal("pipeline"))
			Expect(params[1].Name).To(Equal("name"))
			Expect(params[1].Value.StringVal).To(Equal("my-cluster-pipeline"))
			Expect(params[2].Name).To(Equal("namespace"))
			Expect(params[2].Value.StringVal).To(Equal("my-namespace"))

			ref = gitRef.ToTektonPipelineRef()
			Expect(string(ref.ResolverRef.Resolver)).To(Equal("git"))
			params = ref.ResolverRef.Params
			Expect(params[0].Name).To(Equal("url"))
			Expect(params[0].Value.StringVal).To(Equal("my-git-url"))
			Expect(params[1].Name).To(Equal("revision"))
			Expect(params[1].Value.StringVal).To(Equal("my-revision"))
			Expect(params[2].Name).To(Equal("pathInRepo"))
			Expect(params[2].Value.StringVal).To(Equal("my-path-in-repo"))

			ref = bundleRef.ToTektonPipelineRef()
			Expect(string(ref.ResolverRef.Resolver)).To(Equal("bundles"))
			params = ref.ResolverRef.Params
			Expect(params[0].Name).To(Equal("bundle"))
			Expect(params[0].Value.StringVal).To(Equal("my-bundle"))
			Expect(params[1].Name).To(Equal("name"))
			Expect(params[1].Value.StringVal).To(Equal("my-pipeline"))
			Expect(params[2].Name).To(Equal("kind"))
			Expect(params[2].Value.StringVal).To(Equal("pipeline"))
		})
	})

	When("GetOciStorageParam method is called", func() {
		It("should return ociStorage param when set on PipelineRef", func() {
			pipelineRef := PipelineRef{
				Resolver: "git",
				Params: []Param{
					{Name: "url", Value: "my-git-url"},
				},
				OciStorage: "quay.io/my-org/storage",
			}

			params := pipelineRef.GetOciStorageParam()
			Expect(params).To(HaveLen(1))
			Expect(reflect.TypeOf(params[0])).To(Equal(reflect.TypeOf(tektonv1.Param{})))
			Expect(params[0].Name).To(Equal("ociStorage"))
			Expect(params[0].Value.StringVal).To(Equal("quay.io/my-org/storage"))
		})

		It("should return ociStorage param for Pipeline via PipelineRef", func() {
			pipeline := Pipeline{
				PipelineRef: PipelineRef{
					Resolver: "git",
					Params: []Param{
						{Name: "url", Value: "my-git-url"},
					},
					OciStorage: "quay.io/my-org/storage",
				},
			}

			params := pipeline.GetOciStorageParam()
			Expect(params).To(HaveLen(1))
			Expect(reflect.TypeOf(params[0])).To(Equal(reflect.TypeOf(tektonv1.Param{})))
			Expect(params[0].Name).To(Equal("ociStorage"))
			Expect(params[0].Value.StringVal).To(Equal("quay.io/my-org/storage"))
		})

		It("should return an empty list when PipelineRef has no ociStorage", func() {
			pipeline := Pipeline{
				PipelineRef: PipelineRef{
					Resolver: "git",
					Params: []Param{
						{Name: "url", Value: "my-git-url"},
					},
				},
			}
			params := pipeline.GetOciStorageParam()
			Expect(params).To(BeEmpty())
		})
	})

	When("GetTektonParams method is called on ParameterizedPipeline", func() {
		It("should return a tekton Param list for ParameterizedPipeline using its own Params field", func() {
			// ParameterizedPipeline uses its own Params field (for TenantPipeline/FinalPipeline)
			parameterizedPipeline := ParameterizedPipeline{
				Pipeline: Pipeline{
					PipelineRef: PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "my-git-url"},
						},
					},
				},
				Params: []Param{
					{Name: "parameter1", Value: "value1"},
					{Name: "parameter2", Value: "value2"},
				},
			}

			params := parameterizedPipeline.GetTektonParams()
			Expect(reflect.TypeOf(params[0])).To(Equal(reflect.TypeOf(tektonv1.Param{})))
			Expect(params[0].Name).To(Equal("parameter1"))
			Expect(params[0].Value.StringVal).To(Equal("value1"))

			Expect(reflect.TypeOf(params[1])).To(Equal(reflect.TypeOf(tektonv1.Param{})))
			Expect(params[1].Name).To(Equal("parameter2"))
			Expect(params[1].Value.StringVal).To(Equal("value2"))
		})

		It("should return an empty list when ParameterizedPipeline has no params", func() {
			parameterizedPipeline := ParameterizedPipeline{
				Pipeline: Pipeline{
					PipelineRef: PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "my-git-url"},
						},
					},
				},
			}
			params := parameterizedPipeline.GetTektonParams()
			Expect(params).To(BeEmpty())
		})
	})

	When("IsClusterScoped method is called", func() {
		It("should return true for a cluster pipeline", func() {
			Expect(clusterRef.IsClusterScoped()).To(BeTrue())
		})

		It("should return false for non-cluster pipelines", func() {
			Expect(gitRef.IsClusterScoped()).To(BeFalse())
			Expect(bundleRef.IsClusterScoped()).To(BeFalse())
		})
	})

	When("GetFailedTaskRunLogs is called", func() {
		var (
			ctx    context.Context
			scheme *runtime.Scheme
		)

		BeforeEach(func() {
			ctx = context.Background()
			scheme = runtime.NewScheme()
			Expect(tektonv1.AddToScheme(scheme)).To(Succeed())
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
		})

		It("should return empty string when pipelineRun is nil", func() {
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			result := GetFailedTaskRunLogs(ctx, cli, nil)
			Expect(result).To(BeEmpty())
		})

		It("should return empty string when no TaskRuns exist", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-run",
					Namespace: "default",
				},
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{
								TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
								Name:             "test-task-run",
								PipelineTaskName: "test-task",
							},
						},
					},
				},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			result := GetFailedTaskRunLogs(ctx, cli, pipelineRun)
			Expect(result).To(BeEmpty())
		})

		It("should return empty string when TaskRun succeeded", func() {
			taskRun := &tektonv1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-task-run",
					Namespace: "default",
					Labels: map[string]string{
						"tekton.dev/pipelineRun": "test-pipeline-run",
					},
				},
				Status: tektonv1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{
								Type:   apis.ConditionSucceeded,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-run",
					Namespace: "default",
				},
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{
								TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
								Name:             "test-task-run",
								PipelineTaskName: "test-task",
							},
						},
					},
				},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(taskRun).Build()
			result := GetFailedTaskRunLogs(ctx, cli, pipelineRun)
			Expect(result).To(BeEmpty())
		})

		It("should return failure message when TaskRun failed", func() {
			taskRun := &tektonv1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-task-run",
					Namespace: "default",
					Labels: map[string]string{
						"tekton.dev/pipelineRun": "test-pipeline-run",
					},
				},
				Status: tektonv1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{
								Type:    apis.ConditionSucceeded,
								Status:  corev1.ConditionFalse,
								Message: "task execution failed",
							},
						},
					},
				},
			}
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-run",
					Namespace: "default",
				},
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{
								TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
								Name:             "test-task-run",
								PipelineTaskName: "test-task",
							},
						},
					},
				},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(taskRun).Build()
			result := GetFailedTaskRunLogs(ctx, cli, pipelineRun)
			Expect(result).To(Equal("task test-task failed: task execution failed"))
		})

		It("should truncate long messages", func() {
			longMessage := strings.Repeat("x", maxConditionMessageLength+1000)
			taskRun := &tektonv1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-task-run",
					Namespace: "default",
					Labels: map[string]string{
						"tekton.dev/pipelineRun": "test-pipeline-run",
					},
				},
				Status: tektonv1.TaskRunStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{
							{
								Type:    apis.ConditionSucceeded,
								Status:  corev1.ConditionFalse,
								Message: longMessage,
							},
						},
					},
				},
			}
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-run",
					Namespace: "default",
				},
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{
								TypeMeta:         runtime.TypeMeta{Kind: "TaskRun"},
								Name:             "test-task-run",
								PipelineTaskName: "test-task",
							},
						},
					},
				},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(taskRun).Build()
			result := GetFailedTaskRunLogs(ctx, cli, pipelineRun)
			Expect(len(result)).To(BeNumerically("<=", maxConditionMessageLength))
			Expect(result).To(ContainSubstring("...(truncated)"))
		})

		It("should skip non-TaskRun child references", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-run",
					Namespace: "default",
				},
				Status: tektonv1.PipelineRunStatus{
					PipelineRunStatusFields: tektonv1.PipelineRunStatusFields{
						ChildReferences: []tektonv1.ChildStatusReference{
							{
								TypeMeta:         runtime.TypeMeta{Kind: "CustomRun"},
								Name:             "some-custom-run",
								PipelineTaskName: "custom-task",
							},
						},
					},
				},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			result := GetFailedTaskRunLogs(ctx, cli, pipelineRun)
			Expect(result).To(BeEmpty())
		})
	})

})
