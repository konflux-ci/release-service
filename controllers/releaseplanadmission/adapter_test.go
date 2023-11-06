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
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	toolkit "github.com/redhat-appstudio/operator-toolkit/loader"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"github.com/redhat-appstudio/release-service/metadata"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	createReleasePlanAdmissionAndAdapter = func() *adapter {
		releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rpa",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleasePlanAdmissionSpec{
				Applications: []string{"application"},
				Origin:       "default",
				Policy:       "policy",
				PipelineRef: &tektonutils.PipelineRef{
					Resolver: "bundles",
					Params: []tektonutils.Param{
						{Name: "bundle", Value: "quay.io/some/bundle"},
						{Name: "name", Value: "release-pipeline"},
						{Name: "kind", Value: "pipeline"},
					},
				},
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
