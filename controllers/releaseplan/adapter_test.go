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

package releaseplan

import (
	"reflect"
	"time"

	toolkit "github.com/konflux-ci/operator-toolkit/loader"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/loader"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("ReleasePlan adapter", Ordered, func() {
	var (
		createReleasePlanAndAdapter func() *adapter
		createResources             func()
		deleteResources             func()

		application          *applicationapiv1alpha1.Application
		releasePlanAdmission *v1alpha1.ReleasePlanAdmission
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

	Context("When EnsureOwnerReferenceIsSet is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.releasePlan)
		})

		BeforeEach(func() {
			adapter = createReleasePlanAndAdapter()
		})

		It("should not fail if the Application does not exist", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ApplicationContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result, err := adapter.EnsureOwnerReferenceIsSet()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(result.RequeueDelay).To(Equal(time.Minute))
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlan.OwnerReferences).To(HaveLen(0))
		})

		It("should set the owner reference", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ApplicationContextKey,
					Resource:   application,
				},
			})

			Expect(adapter.releasePlan.OwnerReferences).To(HaveLen(0))
			result, err := adapter.EnsureOwnerReferenceIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlan.OwnerReferences).To(HaveLen(1))
		})

		It("should delete the releasePlan if the owner is deleted", func() {
			newApplication := &applicationapiv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "application-",
					Namespace:    "default",
				},
			}
			Expect(k8sClient.Create(ctx, newApplication)).To(Succeed())

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ApplicationContextKey,
					Resource:   newApplication,
				},
			})

			Expect(adapter.releasePlan.OwnerReferences).To(HaveLen(0))
			result, err := adapter.EnsureOwnerReferenceIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlan.OwnerReferences).To(HaveLen(1))

			Expect(k8sClient.Delete(ctx, newApplication)).To(Succeed())
			_, err = adapter.loader.GetReleasePlan(ctx, k8sClient, &v1alpha1.Release{
				Spec: v1alpha1.ReleaseSpec{
					ReleasePlan: adapter.releasePlan.Name,
				},
			})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When EnsureMatchingInformationIsSet is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.releasePlan)
		})

		BeforeEach(func() {
			adapter = createReleasePlanAndAdapter()
		})

		It("should mark the ReleasePlan as unmatched if the ReleasePlanAdmission is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlanAdmissionContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result, err := adapter.EnsureMatchingInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlan.Status.ReleasePlanAdmission).To(Equal(v1alpha1.MatchedReleasePlanAdmission{}))
		})

		It("should mark the ReleasePlan as matched", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			result, err := adapter.EnsureMatchingInformationIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.releasePlan.Status.ReleasePlanAdmission.Name).To(Equal(
				releasePlanAdmission.Namespace + "/" + releasePlanAdmission.Name))
		})

		It("should not update the lastTransitionTime in the condition if the matched ReleasePlanAdmission hasn't changed", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.MatchedReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			adapter.EnsureMatchingInformationIsSet()
			condition := meta.FindStatusCondition(adapter.releasePlan.Status.Conditions, "Matched")
			lastTransitionTime := condition.LastTransitionTime
			adapter.EnsureMatchingInformationIsSet()
			condition = meta.FindStatusCondition(adapter.releasePlan.Status.Conditions, "Matched")
			Expect(condition.LastTransitionTime).To(Equal(lastTransitionTime))
		})
	})

	createReleasePlanAndAdapter = func() *adapter {
		parameterizedPipeline := &tektonutils.ParameterizedPipeline{}
		parameterizedPipeline.PipelineRef = tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: "my-url"},
				{Name: "revision", Value: "my-revision"},
				{Name: "pathInRepo", Value: "my-path"},
			},
		}
		parameterizedPipeline.Params = []tektonutils.Param{
			{Name: "parameter1", Value: "value1"},
			{Name: "parameter2", Value: "value2"},
		}
		parameterizedPipeline.ServiceAccount = "test-account"

		releasePlan := &v1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "releaseplan-",
				Namespace:    "default",
			},
			Spec: v1alpha1.ReleasePlanSpec{
				Application:            application.Name,
				Target:                 "default",
				Pipeline:               parameterizedPipeline,
				ReleaseGracePeriodDays: 6,
			},
		}

		Expect(k8sClient.Create(ctx, releasePlan)).To(Succeed())
		releasePlan.Kind = "ReleasePlan"

		return newAdapter(ctx, k8sClient, releasePlan, loader.NewMockLoader(), &ctrl.Log)
	}

	createResources = func() {
		application = &applicationapiv1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "application",
				Namespace: "default",
			},
			Spec: applicationapiv1alpha1.ApplicationSpec{
				DisplayName: "application",
			},
		}
		Expect(k8sClient.Create(ctx, application)).To(Succeed())

		releasePlanAdmission = &v1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rpa",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleasePlanAdmissionSpec{
				Applications: []string{application.Name},
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
	}

	deleteResources = func() {
		Expect(k8sClient.Delete(ctx, application)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlanAdmission)).To(Succeed())
	}

})
