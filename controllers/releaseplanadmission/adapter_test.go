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

package releaseplanadmission

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	toolkit "github.com/konflux-ci/operator-toolkit/loader"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/loader"
	"github.com/konflux-ci/release-service/metadata"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("ReleasePlanAdmission adapter", Ordered, func() {
	var (
		createReleasePlanAdmissionAndAdapter func() *adapter
		createResources                      func()
		deleteResources                      func()

		releasePlan *v1alpha1.ReleasePlan
	)

	AfterAll(func() {
		deleteResources()
	})

	BeforeAll(func() {
		createResources()
	})

	Context("When newAdapter is called", func() {
		It("creates and return a new adapter", func() {
			Expect(reflect.TypeOf(newAdapter(ctx, k8sClient, nil, loader.NewLoader(), &ctrl.Log))).To(Equal(reflect.TypeOf(&adapter{})))
		})
	})

	Context("When EnsureMatchingInformationIsSet is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.releasePlanAdmission)
		})

		BeforeEach(func() {
			adapter = createReleasePlanAdmissionAndAdapter()
		})

		It("should RequeueWithError if error occurs when looking for ReleasePlans", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Err:        fmt.Errorf("some error"),
				},
			})

			result, err := adapter.EnsureMatchingInformationIsSet()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.ReleasePlans).To(BeNil())
		})

		It("should mark the ReleasePlanAdmission as unmatched if no ReleasePlans are found", func() {
			adapter.releasePlanAdmission.Status.ReleasePlans = []v1alpha1.MatchedReleasePlan{{Name: "foo"}}
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource:   &v1alpha1.ReleasePlanList{},
				},
			})

			result, err := adapter.EnsureMatchingInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.ReleasePlans).To(Equal([]v1alpha1.MatchedReleasePlan{}))
			for i := range adapter.releasePlanAdmission.Status.Conditions {
				if adapter.releasePlanAdmission.Status.Conditions[i].Type == "Matched" {
					Expect(adapter.releasePlanAdmission.Status.Conditions[i].Status).To(Equal(metav1.ConditionFalse))
				}
			}
		})

		It("should mark the ReleasePlanAdmission as matched if ReleasePlans are found", func() {
			secondReleasePlan := releasePlan.DeepCopy()
			secondReleasePlan.Name = "rp-two"
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource: &v1alpha1.ReleasePlanList{
						Items: []v1alpha1.ReleasePlan{
							*releasePlan,
							*secondReleasePlan,
						},
					},
				},
			})

			result, err := adapter.EnsureMatchingInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.ReleasePlans).To(HaveLen(2))
			for i := range adapter.releasePlanAdmission.Status.Conditions {
				if adapter.releasePlanAdmission.Status.Conditions[i].Type == "Matched" {
					Expect(adapter.releasePlanAdmission.Status.Conditions[i].Status).To(Equal(metav1.ConditionTrue))
				}
			}
		})

		It("should overwrite previously matched ReleasePlans", func() {
			adapter.releasePlanAdmission.Status.ReleasePlans = []v1alpha1.MatchedReleasePlan{
				{Name: "rp-two", Active: false},
				{Name: "foo", Active: false},
			}
			secondReleasePlan := releasePlan.DeepCopy()
			secondReleasePlan.Name = "rp-two"

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource: &v1alpha1.ReleasePlanList{
						Items: []v1alpha1.ReleasePlan{
							*releasePlan,
							*secondReleasePlan,
						},
					},
				},
			})

			result, err := adapter.EnsureMatchingInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.ReleasePlans).To(HaveLen(2))
			for i := range adapter.releasePlanAdmission.Status.Conditions {
				if adapter.releasePlanAdmission.Status.Conditions[i].Type == "Matched" {
					Expect(adapter.releasePlanAdmission.Status.Conditions[i].Status).To(Equal(metav1.ConditionTrue))
				}
			}
			for _, releasePlan := range adapter.releasePlanAdmission.Status.ReleasePlans {
				Expect(releasePlan.Name).NotTo(Equal("foo"))
			}
		})

		It("should update the condition time if only the auto-release label on a ReleasePlan changes", func() {
			testReleasePlan := releasePlan.DeepCopy()
			testReleasePlan.Labels = map[string]string{}
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource: &v1alpha1.ReleasePlanList{
						Items: []v1alpha1.ReleasePlan{
							*testReleasePlan,
						},
					},
				},
			})

			adapter.EnsureMatchingInformationIsSet()
			condition := meta.FindStatusCondition(adapter.releasePlanAdmission.Status.Conditions, "Matched")
			lastTransitionTime := condition.LastTransitionTime
			time.Sleep(1 * time.Second)

			testReleasePlan.Labels[metadata.AutoReleaseLabel] = "true"
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource: &v1alpha1.ReleasePlanList{
						Items: []v1alpha1.ReleasePlan{
							*testReleasePlan,
						},
					},
				},
			})

			adapter.EnsureMatchingInformationIsSet()
			condition = meta.FindStatusCondition(adapter.releasePlanAdmission.Status.Conditions, "Matched")
			Expect(condition.LastTransitionTime).NotTo(Equal(lastTransitionTime))
			Expect(adapter.releasePlanAdmission.Status.ReleasePlans).To(HaveLen(1))
			Expect(adapter.releasePlanAdmission.Status.ReleasePlans[0].Active).To(BeTrue())
		})
	})

	Context("When EnsureRetryInformationIsSet is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.releasePlanAdmission)
		})

		BeforeEach(func() {
			adapter = createReleasePlanAdmissionAndAdapter()
			os.Setenv("SERVICE_NAMESPACE", "default")
		})

		It("should set RetryInfo to disabled when ReleaseServiceConfig is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleaseServiceConfigContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{Group: "appstudio.redhat.com", Resource: "releaseserviceconfigs"}, v1alpha1.ReleaseServiceConfigResourceName),
				},
				// No MatchedReleasePlansContextKey needed - we skip loading RPs when RSC is nil
			})

			result, err := adapter.EnsureRetryInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo).ToNot(BeNil())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Enabled).To(BeFalse())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Reason).To(Equal("no retry configuration available"))
		})

		It("should set RetryInfo to disabled when pipeline doesn't match any retryable pipeline", func() {
			// Update RPA with git resolver pipeline that doesn't match
			adapter.releasePlanAdmission.Spec.Pipeline = &tektonutils.Pipeline{
				PipelineRef: tektonutils.PipelineRef{
					Resolver: "git",
					Params: []tektonutils.Param{
						{Name: "url", Value: "https://github.com/org/repo"},
						{Name: "revision", Value: "main"},
						{Name: "pathInRepo", Value: "pipelines/release.yaml"},
					},
				},
			}
			Expect(k8sClient.Update(ctx, adapter.releasePlanAdmission)).To(Succeed())

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

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleaseServiceConfigContextKey,
					Resource:   rsc,
				},
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource:   &v1alpha1.ReleasePlanList{},
				},
			})

			result, err := adapter.EnsureRetryInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo).ToNot(BeNil())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Enabled).To(BeFalse())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Reason).To(Equal("pipeline not configured for retries"))
		})

		It("should set RetryInfo to enabled when pipeline matches retryable pipeline", func() {
			// Update RPA with git resolver pipeline
			adapter.releasePlanAdmission.Spec.Pipeline = &tektonutils.Pipeline{
				PipelineRef: tektonutils.PipelineRef{
					Resolver: "git",
					Params: []tektonutils.Param{
						{Name: "url", Value: "https://github.com/org/repo"},
						{Name: "revision", Value: "main"},
						{Name: "pathInRepo", Value: "pipelines/release.yaml"},
					},
				},
			}
			Expect(k8sClient.Update(ctx, adapter.releasePlanAdmission)).To(Succeed())

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

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleaseServiceConfigContextKey,
					Resource:   rsc,
				},
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource:   &v1alpha1.ReleasePlanList{},
				},
			})

			result, err := adapter.EnsureRetryInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo).ToNot(BeNil())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Enabled).To(BeTrue())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.MaxRetries).ToNot(BeNil())
			Expect(*adapter.releasePlanAdmission.Status.RetryInfo.MaxRetries).To(Equal(maxRetries))
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Reason).To(Equal("retries enabled by policy"))
		})

		It("should set RetryInfo to disabled when tags match disable condition", func() {
			// Update RPA with git resolver pipeline
			adapter.releasePlanAdmission.Spec.Pipeline = &tektonutils.Pipeline{
				PipelineRef: tektonutils.PipelineRef{
					Resolver: "git",
					Params: []tektonutils.Param{
						{Name: "url", Value: "https://github.com/org/repo"},
						{Name: "revision", Value: "main"},
						{Name: "pathInRepo", Value: "pipelines/release.yaml"},
					},
				},
			}
			Expect(k8sClient.Update(ctx, adapter.releasePlanAdmission)).To(Succeed())

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
									Tags: []string{"{{ release_timestamp }}", "{{ incrementer }}"},
								},
							},
						},
					},
				},
			}

			// Create matched RP with {{ release_timestamp }} tag in Data
			rpData, _ := json.Marshal(map[string]interface{}{
				"mapping": map[string]interface{}{
					"components": []interface{}{
						map[string]interface{}{
							"tags": []string{"{{ release_timestamp }}"},
						},
					},
				},
			})
			matchedRP := &v1alpha1.ReleasePlan{
				Spec: v1alpha1.ReleasePlanSpec{
					Application: "app",
					Data: &runtime.RawExtension{
						Raw: rpData,
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleaseServiceConfigContextKey,
					Resource:   rsc,
				},
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource: &v1alpha1.ReleasePlanList{
						Items: []v1alpha1.ReleasePlan{*matchedRP},
					},
				},
			})

			result, err := adapter.EnsureRetryInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo).ToNot(BeNil())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Enabled).To(BeFalse())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Reason).To(Equal("disabled by tag: {{ release_timestamp }}"))
		})

		It("should copy Mitigations from RSC to RetryInfo", func() {
			// Update RPA with git resolver pipeline
			adapter.releasePlanAdmission.Spec.Pipeline = &tektonutils.Pipeline{
				PipelineRef: tektonutils.PipelineRef{
					Resolver: "git",
					Params: []tektonutils.Param{
						{Name: "url", Value: "https://github.com/org/repo"},
						{Name: "revision", Value: "main"},
						{Name: "pathInRepo", Value: "pipelines/release.yaml"},
					},
				},
			}
			Expect(k8sClient.Update(ctx, adapter.releasePlanAdmission)).To(Succeed())

			maxRetries := 3
			mitigations := &v1alpha1.Mitigations{
				OOMKill: &v1alpha1.MemoryMitigation{
					Multiplier: "1.5",
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
								MaxRetries:  maxRetries,
								Mitigations: mitigations,
							},
						},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleaseServiceConfigContextKey,
					Resource:   rsc,
				},
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource:   &v1alpha1.ReleasePlanList{},
				},
			})

			result, err := adapter.EnsureRetryInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo).ToNot(BeNil())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Enabled).To(BeTrue())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Mitigations).To(Equal(mitigations))
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Mitigations.OOMKill.Multiplier).To(Equal("1.5"))
		})

		It("should not patch when RetryInfo is unchanged", func() {
			// Update RPA with git resolver pipeline
			adapter.releasePlanAdmission.Spec.Pipeline = &tektonutils.Pipeline{
				PipelineRef: tektonutils.PipelineRef{
					Resolver: "git",
					Params: []tektonutils.Param{
						{Name: "url", Value: "https://github.com/org/repo"},
						{Name: "revision", Value: "main"},
						{Name: "pathInRepo", Value: "pipelines/release.yaml"},
					},
				},
			}
			maxRetries := 3
			adapter.releasePlanAdmission.Status.RetryInfo = &v1alpha1.RetryInfo{
				Enabled:    true,
				MaxRetries: &maxRetries,
				Reason:     "retries enabled by policy",
			}
			Expect(k8sClient.Update(ctx, adapter.releasePlanAdmission)).To(Succeed())

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

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleaseServiceConfigContextKey,
					Resource:   rsc,
				},
				{
					ContextKey: loader.MatchedReleasePlansContextKey,
					Resource:   &v1alpha1.ReleasePlanList{},
				},
			})

			result, err := adapter.EnsureRetryInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			// RetryInfo should still be set but no patch should have been issued
			Expect(adapter.releasePlanAdmission.Status.RetryInfo).ToNot(BeNil())
			Expect(adapter.releasePlanAdmission.Status.RetryInfo.Enabled).To(BeTrue())
		})
	})

	createReleasePlanAdmissionAndAdapter = func() *adapter {
		releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rpa",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleasePlanAdmissionSpec{
				Applications: []string{"application"},
				Origin:       "default",
				Pipeline: &tektonutils.Pipeline{
					PipelineRef: tektonutils.PipelineRef{
						Resolver: "bundles",
						Params: []tektonutils.Param{
							{Name: "bundle", Value: "quay.io/some/bundle"},
							{Name: "name", Value: "release-pipeline"},
							{Name: "kind", Value: "pipeline"},
						},
					},
				},
				Policy: "policy",
			},
		}
		Expect(k8sClient.Create(ctx, releasePlanAdmission)).To(Succeed())
		releasePlan.Kind = "ReleasePlanAdmission"

		return newAdapter(ctx, k8sClient, releasePlanAdmission, loader.NewMockLoader(), &ctrl.Log)
	}

	createResources = func() {
		releasePlan = &v1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "releaseplan-",
				Namespace:    "default",
			},
			Spec: v1alpha1.ReleasePlanSpec{
				Application: "application",
				Target:      "default",
			},
		}
		Expect(k8sClient.Create(ctx, releasePlan)).To(Succeed())
	}

	deleteResources = func() {
		Expect(k8sClient.Delete(ctx, releasePlan)).To(Succeed())
	}

})
