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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	toolkit "github.com/redhat-appstudio/operator-toolkit/loader"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"k8s.io/apimachinery/pkg/api/meta"

	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"

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
		releasePlan := &v1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "releaseplan-",
				Namespace:    "default",
			},
			Spec: v1alpha1.ReleasePlanSpec{
				Application: application.Name,
				Target:      "default",
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
	}

	deleteResources = func() {
		Expect(k8sClient.Delete(ctx, application)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlanAdmission)).To(Succeed())
	}

})
