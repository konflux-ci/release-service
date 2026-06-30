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

package retry_test

import (
	"encoding/json"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/retry"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
)

var _ = Describe("Retry Matcher", func() {
	var logger logr.Logger

	BeforeEach(func() {
		logger = logr.Discard()
	})

	Context("ExtractTags", func() {
		It("should extract tags from defaults", func() {
			data := createTestData(map[string]interface{}{
				"mapping": map[string]interface{}{
					"defaults": map[string]interface{}{
						"tags": []string{"tag1", "tag2"},
					},
				},
			})

			tags := retry.ExtractTags(data, &logger)
			Expect(tags).To(ConsistOf("tag1", "tag2"))
		})

		It("should extract tags from components", func() {
			data := createTestData(map[string]interface{}{
				"mapping": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"tags": []string{"comp-tag1"},
						},
					},
				},
			})

			tags := retry.ExtractTags(data, &logger)
			Expect(tags).To(ConsistOf("comp-tag1"))
		})

		It("should extract componentTags from components", func() {
			data := createTestData(map[string]interface{}{
				"mapping": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"componentTags": []string{"comp-tag2"},
						},
					},
				},
			})

			tags := retry.ExtractTags(data, &logger)
			Expect(tags).To(ConsistOf("comp-tag2"))
		})

		It("should extract tags from repositories", func() {
			data := createTestData(map[string]interface{}{
				"mapping": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"repositories": []map[string]interface{}{
								{
									"tags": []string{"repo-tag1"},
								},
							},
						},
					},
				},
			})

			tags := retry.ExtractTags(data, &logger)
			Expect(tags).To(ConsistOf("repo-tag1"))
		})

		It("should extract tags from all four locations", func() {
			data := createTestData(map[string]interface{}{
				"mapping": map[string]interface{}{
					"defaults": map[string]interface{}{
						"tags": []string{"default-tag"},
					},
					"components": []map[string]interface{}{
						{
							"tags":          []string{"comp-tag"},
							"componentTags": []string{"comp-tag-alt"},
							"repositories": []map[string]interface{}{
								{
									"tags": []string{"repo-tag"},
								},
							},
						},
					},
				},
			})

			tags := retry.ExtractTags(data, &logger)
			Expect(tags).To(ConsistOf("default-tag", "comp-tag", "comp-tag-alt", "repo-tag"))
		})

		It("should return empty slice for nil data", func() {
			tags := retry.ExtractTags(nil, &logger)
			Expect(tags).To(BeEmpty())
		})

		It("should return empty slice for malformed JSON", func() {
			data := &runtime.RawExtension{Raw: []byte("not valid json")}
			tags := retry.ExtractTags(data, &logger)
			Expect(tags).To(BeEmpty())
		})
	})

	Context("IsDisabledByTags", func() {
		It("should return true if a tag matches", func() {
			matchedTag, disabled := retry.IsDisabledByTags(
				[]string{"prod", "test"},
				[]string{"prod"},
			)
			Expect(disabled).To(BeTrue())
			Expect(matchedTag).To(Equal("prod"))
		})

		It("should return false if no tag matches", func() {
			_, disabled := retry.IsDisabledByTags(
				[]string{"staging", "test"},
				[]string{"prod"},
			)
			Expect(disabled).To(BeFalse())
		})

		It("should return false for empty disable tags", func() {
			_, disabled := retry.IsDisabledByTags(
				[]string{"prod"},
				[]string{},
			)
			Expect(disabled).To(BeFalse())
		})
	})

	Context("GetMatchingRetryablePipeline", func() {
		It("should match pipeline with exact URL and revision", func() {
			pipeline := createTestPipeline(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
			)

			retryablePipelines := []v1alpha1.RetryablePipeline{
				{
					Url:        "https://github.com/org/repo",
					Revision:   "main",
					PathInRepo: "pipelines/release.yaml",
					RetryPolicy: v1alpha1.RetryPolicy{
						MaxRetries: 3,
					},
				},
			}

			matched := retry.GetMatchingRetryablePipeline(pipeline, retryablePipelines, &logger)
			Expect(matched).ToNot(BeNil())
			Expect(matched.RetryPolicy.MaxRetries).To(Equal(3))
		})

		It("should match pipeline with regex URL", func() {
			pipeline := createTestPipeline(
				"https://github.com/konflux/release-service-catalog",
				"main",
				"pipelines/release.yaml",
			)

			retryablePipelines := []v1alpha1.RetryablePipeline{
				{
					Url:        "https://github.com/konflux/.*",
					Revision:   "main",
					PathInRepo: "pipelines/release.yaml",
					RetryPolicy: v1alpha1.RetryPolicy{
						MaxRetries: 5,
					},
				},
			}

			matched := retry.GetMatchingRetryablePipeline(pipeline, retryablePipelines, &logger)
			Expect(matched).ToNot(BeNil())
			Expect(matched.RetryPolicy.MaxRetries).To(Equal(5))
		})

		It("should match pipeline with regex revision", func() {
			pipeline := createTestPipeline(
				"https://github.com/org/repo",
				"release-v1.2.3",
				"pipelines/release.yaml",
			)

			retryablePipelines := []v1alpha1.RetryablePipeline{
				{
					Url:        "https://github.com/org/repo",
					Revision:   "release-.*",
					PathInRepo: "pipelines/release.yaml",
					RetryPolicy: v1alpha1.RetryPolicy{
						MaxRetries: 2,
					},
				},
			}

			matched := retry.GetMatchingRetryablePipeline(pipeline, retryablePipelines, &logger)
			Expect(matched).ToNot(BeNil())
		})

		It("should not match if pathInRepo differs", func() {
			pipeline := createTestPipeline(
				"https://github.com/org/repo",
				"main",
				"pipelines/different.yaml",
			)

			retryablePipelines := []v1alpha1.RetryablePipeline{
				{
					Url:        "https://github.com/org/repo",
					Revision:   "main",
					PathInRepo: "pipelines/release.yaml",
					RetryPolicy: v1alpha1.RetryPolicy{
						MaxRetries: 3,
					},
				},
			}

			matched := retry.GetMatchingRetryablePipeline(pipeline, retryablePipelines, &logger)
			Expect(matched).To(BeNil())
		})

		It("should not match URL substring due to anchoring", func() {
			pipeline := createTestPipeline(
				"https://github.com/org/repo-extra",
				"main",
				"pipelines/release.yaml",
			)

			retryablePipelines := []v1alpha1.RetryablePipeline{
				{
					Url:        "https://github.com/org/repo",
					Revision:   "main",
					PathInRepo: "pipelines/release.yaml",
					RetryPolicy: v1alpha1.RetryPolicy{
						MaxRetries: 3,
					},
				},
			}

			matched := retry.GetMatchingRetryablePipeline(pipeline, retryablePipelines, &logger)
			Expect(matched).To(BeNil())
		})

		It("should not match revision substring due to anchoring", func() {
			pipeline := createTestPipeline(
				"https://github.com/org/repo",
				"main-extra",
				"pipelines/release.yaml",
			)

			retryablePipelines := []v1alpha1.RetryablePipeline{
				{
					Url:        "https://github.com/org/repo",
					Revision:   "main",
					PathInRepo: "pipelines/release.yaml",
					RetryPolicy: v1alpha1.RetryPolicy{
						MaxRetries: 3,
					},
				},
			}

			matched := retry.GetMatchingRetryablePipeline(pipeline, retryablePipelines, &logger)
			Expect(matched).To(BeNil())
		})

		It("should return nil for nil pipeline", func() {
			matched := retry.GetMatchingRetryablePipeline(nil, []v1alpha1.RetryablePipeline{}, &logger)
			Expect(matched).To(BeNil())
		})
	})

	Context("DetermineRetryInfo", func() {
		It("should return disabled when RSC is nil", func() {
			rpa := createTestRPA(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
				nil,
			)

			retryInfo := retry.DetermineRetryInfo(rpa, &v1alpha1.ReleasePlanList{}, nil, &logger)
			Expect(retryInfo.Enabled).To(BeFalse())
			Expect(retryInfo.Reason).To(Equal("no retry configuration available"))
		})

		It("should return disabled when RPA has no pipeline", func() {
			rpa := &v1alpha1.ReleasePlanAdmission{
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Pipeline: nil,
				},
			}

			rsc := &v1alpha1.ReleaseServiceConfig{}

			retryInfo := retry.DetermineRetryInfo(rpa, &v1alpha1.ReleasePlanList{}, rsc, &logger)
			Expect(retryInfo.Enabled).To(BeFalse())
			Expect(retryInfo.Reason).To(Equal("no pipeline defined"))
		})

		It("should return disabled when pipeline doesn't match any retryable pipeline", func() {
			rpa := createTestRPA(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
				nil,
			)

			rsc := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					RetryablePipelines: []v1alpha1.RetryablePipeline{
						{
							Url:        "https://github.com/different/repo",
							Revision:   "main",
							PathInRepo: "pipelines/release.yaml",
						},
					},
				},
			}

			retryInfo := retry.DetermineRetryInfo(rpa, &v1alpha1.ReleasePlanList{}, rsc, &logger)
			Expect(retryInfo.Enabled).To(BeFalse())
			Expect(retryInfo.Reason).To(Equal("pipeline not configured for retries"))
		})

		It("should return enabled when pipeline matches", func() {
			rpa := createTestRPA(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
				nil,
			)

			maxRetries := 3
			rsc := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					RetryablePipelines: []v1alpha1.RetryablePipeline{
						{
							Url:        "https://github.com/org/repo",
							Revision:   "main",
							PathInRepo: "pipelines/release.yaml",
							RetryPolicy: v1alpha1.RetryPolicy{
								MaxRetries: maxRetries,
							},
						},
					},
				},
			}

			retryInfo := retry.DetermineRetryInfo(rpa, &v1alpha1.ReleasePlanList{}, rsc, &logger)
			Expect(retryInfo.Enabled).To(BeTrue())
			Expect(retryInfo.MaxRetries).ToNot(BeNil())
			Expect(*retryInfo.MaxRetries).To(Equal(maxRetries))
			Expect(retryInfo.Reason).To(Equal("retries enabled by policy"))
		})

		It("should return disabled when RPA tags match disable tags", func() {
			rpaData := createTestData(map[string]interface{}{
				"mapping": map[string]interface{}{
					"defaults": map[string]interface{}{
						"tags": []string{"production"},
					},
				},
			})

			rpa := createTestRPA(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
				rpaData,
			)

			rsc := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					RetryablePipelines: []v1alpha1.RetryablePipeline{
						{
							Url:        "https://github.com/org/repo",
							Revision:   "main",
							PathInRepo: "pipelines/release.yaml",
							RetryPolicy: v1alpha1.RetryPolicy{
								MaxRetries: 3,
								DisableOn: &v1alpha1.DisableConditions{
									Tags: []string{"production"},
								},
							},
						},
					},
				},
			}

			retryInfo := retry.DetermineRetryInfo(rpa, &v1alpha1.ReleasePlanList{}, rsc, &logger)
			Expect(retryInfo.Enabled).To(BeFalse())
			Expect(retryInfo.Reason).To(Equal("disabled by tag: production"))
		})

		It("should return disabled when RP tags match disable tags", func() {
			rpa := createTestRPA(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
				nil,
			)

			rpData := createTestData(map[string]interface{}{
				"mapping": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"tags": []string{"staging"},
						},
					},
				},
			})

			matchedRPs := &v1alpha1.ReleasePlanList{
				Items: []v1alpha1.ReleasePlan{
					{
						Spec: v1alpha1.ReleasePlanSpec{
							Data: rpData,
						},
					},
				},
			}

			rsc := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					RetryablePipelines: []v1alpha1.RetryablePipeline{
						{
							Url:        "https://github.com/org/repo",
							Revision:   "main",
							PathInRepo: "pipelines/release.yaml",
							RetryPolicy: v1alpha1.RetryPolicy{
								MaxRetries: 3,
								DisableOn: &v1alpha1.DisableConditions{
									Tags: []string{"staging", "production"},
								},
							},
						},
					},
				},
			}

			retryInfo := retry.DetermineRetryInfo(rpa, matchedRPs, rsc, &logger)
			Expect(retryInfo.Enabled).To(BeFalse())
			Expect(retryInfo.Reason).To(Equal("disabled by tag: staging"))
		})

		It("should return disabled when RPA pipeline MaxRetries is 0", func() {
			maxRetries := 0
			rpa := createTestRPA(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
				nil,
			)
			rpa.Spec.Pipeline.MaxRetries = &maxRetries

			rsc := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					RetryablePipelines: []v1alpha1.RetryablePipeline{
						{
							Url:        "https://github.com/org/repo",
							Revision:   "main",
							PathInRepo: "pipelines/release.yaml",
							RetryPolicy: v1alpha1.RetryPolicy{
								MaxRetries: 3,
							},
						},
					},
				},
			}

			retryInfo := retry.DetermineRetryInfo(rpa, &v1alpha1.ReleasePlanList{}, rsc, &logger)
			Expect(retryInfo.Enabled).To(BeFalse())
			Expect(retryInfo.Reason).To(Equal("retries disabled by RPA pipeline override"))
		})

		It("should use RPA maxRetries but keep mitigations from RSC", func() {
			maxRetries := 5
			rpa := createTestRPA(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
				nil,
			)
			rpa.Spec.Pipeline.MaxRetries = &maxRetries

			rsc := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					RetryablePipelines: []v1alpha1.RetryablePipeline{
						{
							Url:        "https://github.com/org/repo",
							Revision:   "main",
							PathInRepo: "pipelines/release.yaml",
							RetryPolicy: v1alpha1.RetryPolicy{
								MaxRetries: 3,
								Mitigations: &v1alpha1.Mitigations{
									OOMKill: &v1alpha1.MemoryMitigation{Multiplier: "2"},
								},
							},
						},
					},
				},
			}

			retryInfo := retry.DetermineRetryInfo(rpa, &v1alpha1.ReleasePlanList{}, rsc, &logger)
			Expect(retryInfo.Enabled).To(BeTrue())
			Expect(*retryInfo.MaxRetries).To(Equal(5))
			Expect(retryInfo.Mitigations).NotTo(BeNil())
			Expect(retryInfo.Mitigations.OOMKill.Multiplier).To(Equal("2"))
			Expect(retryInfo.Reason).To(Equal("retries enabled by RPA pipeline override"))
		})

		It("should not enable retries with RPA override when pipeline doesn't match RSC", func() {
			maxRetries := 2
			rpa := createTestRPA(
				"https://github.com/org/repo",
				"main",
				"pipelines/release.yaml",
				nil,
			)
			rpa.Spec.Pipeline.MaxRetries = &maxRetries

			rsc := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					RetryablePipelines: []v1alpha1.RetryablePipeline{
						{
							Url:        "https://github.com/different/repo",
							Revision:   "main",
							PathInRepo: "pipelines/release.yaml",
							RetryPolicy: v1alpha1.RetryPolicy{
								MaxRetries: 3,
							},
						},
					},
				},
			}

			// pipeline must match the RSC to get mitigations, RPA only overrides the count
			retryInfo := retry.DetermineRetryInfo(rpa, &v1alpha1.ReleasePlanList{}, rsc, &logger)
			Expect(retryInfo.Enabled).To(BeFalse())
			Expect(retryInfo.Reason).To(Equal("pipeline not configured for retries"))
		})
	})
})

// Helper functions

func createTestData(data map[string]interface{}) *runtime.RawExtension {
	jsonData, _ := json.Marshal(data)
	return &runtime.RawExtension{Raw: jsonData}
}

func createTestPipeline(url, revision, pathInRepo string) *tektonutils.Pipeline {
	return &tektonutils.Pipeline{
		PipelineRef: tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: url},
				{Name: "revision", Value: revision},
				{Name: "pathInRepo", Value: pathInRepo},
			},
		},
	}
}

func createTestRPA(url, revision, pathInRepo string, data *runtime.RawExtension) *v1alpha1.ReleasePlanAdmission {
	return &v1alpha1.ReleasePlanAdmission{
		Spec: v1alpha1.ReleasePlanAdmissionSpec{
			Pipeline: &tektonutils.Pipeline{
				PipelineRef: tektonutils.PipelineRef{
					Resolver: "git",
					Params: []tektonutils.Param{
						{Name: "url", Value: url},
						{Name: "revision", Value: revision},
						{Name: "pathInRepo", Value: pathInRepo},
					},
				},
			},
			Data: data,
		},
	}
}
