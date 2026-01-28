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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
})

var _ = Describe("AdjustTimeouts", func() {
	var logger logr.Logger

	BeforeEach(func() {
		logger = logr.Discard() // Use a no-op logger for tests
	})

	dur := func(d time.Duration) *metav1.Duration {
		return &metav1.Duration{Duration: d}
	}

	DescribeTable("should adjust timeouts correctly",
		func(input *tektonv1.TimeoutFields, expectedPipeline, expectedTasks, expectedFinally *time.Duration) {
			result := AdjustTimeouts(input, logger)

			if expectedPipeline == nil && expectedTasks == nil && expectedFinally == nil {
				// Expect empty struct or nil
				if result != nil {
					Expect(*result).To(Equal(tektonv1.TimeoutFields{}))
				}
				return
			}

			Expect(result).NotTo(BeNil())
			if expectedPipeline != nil {
				Expect(result.Pipeline).NotTo(BeNil())
				Expect(result.Pipeline.Duration).To(Equal(*expectedPipeline))
			}
			if expectedTasks != nil {
				Expect(result.Tasks).NotTo(BeNil())
				Expect(result.Tasks.Duration).To(Equal(*expectedTasks))
			}
			if expectedFinally != nil {
				Expect(result.Finally).NotTo(BeNil())
				Expect(result.Finally.Duration).To(Equal(*expectedFinally))
			}
		},
		Entry("nil timeouts returns nil",
			nil, nil, nil, nil),

		Entry("all unset returns original",
			&tektonv1.TimeoutFields{}, nil, nil, nil),

		Entry("only pipeline (30m) sets tasks to pipeline - 10m",
			&tektonv1.TimeoutFields{Pipeline: dur(30 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), nil),

		Entry("only pipeline (10m < 20m) sets tasks to pipeline/2",
			&tektonv1.TimeoutFields{Pipeline: dur(10 * time.Minute)},
			ptr(10*time.Minute), ptr(5*time.Minute), nil),

		Entry("only tasks sets pipeline to tasks + 10m",
			&tektonv1.TimeoutFields{Tasks: dur(20 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), nil),

		Entry("only finally returns empty struct",
			&tektonv1.TimeoutFields{Finally: dur(5 * time.Minute)},
			nil, nil, nil),

		Entry("pipeline + tasks valid (tasks < pipeline) returns original",
			&tektonv1.TimeoutFields{Pipeline: dur(30 * time.Minute), Tasks: dur(20 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), nil),

		Entry("pipeline + tasks invalid (tasks >= pipeline) adjusts tasks",
			&tektonv1.TimeoutFields{Pipeline: dur(30 * time.Minute), Tasks: dur(30 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), nil),

		Entry("pipeline + finally (finally <= pipeline/2) sets tasks = pipeline - finally",
			&tektonv1.TimeoutFields{Pipeline: dur(30 * time.Minute), Finally: dur(10 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), ptr(10*time.Minute)),

		Entry("pipeline + finally (finally > pipeline/2) adjusts tasks and finally",
			&tektonv1.TimeoutFields{Pipeline: dur(30 * time.Minute), Finally: dur(20 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), ptr(10*time.Minute)),

		Entry("tasks + finally sets pipeline = tasks + finally",
			&tektonv1.TimeoutFields{Tasks: dur(20 * time.Minute), Finally: dur(10 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), ptr(10*time.Minute)),

		Entry("all three valid returns original",
			&tektonv1.TimeoutFields{Pipeline: dur(30 * time.Minute), Tasks: dur(20 * time.Minute), Finally: dur(10 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), ptr(10*time.Minute)),

		Entry("all three invalid adjusts tasks and finally",
			&tektonv1.TimeoutFields{Pipeline: dur(30 * time.Minute), Tasks: dur(25 * time.Minute), Finally: dur(10 * time.Minute)},
			ptr(30*time.Minute), ptr(20*time.Minute), ptr(10*time.Minute)),
	)
})

func ptr(d time.Duration) *time.Duration {
	return &d
}
