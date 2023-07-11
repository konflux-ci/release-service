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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"reflect"

	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("ReleasePlan Adapter", Ordered, func() {
	var (
		createReleasePlanAndAdapter func() *Adapter
		createResources             func()
		deleteResources             func()

		application *applicationapiv1alpha1.Application
	)

	AfterAll(func() {
		deleteResources()
	})

	BeforeAll(func() {
		createResources()
	})

	Context("When NewAdapter is called", func() {
		It("creates and return a new adapter", func() {
			Expect(reflect.TypeOf(NewAdapter(ctx, k8sClient, nil, loader.NewLoader(), ctrl.Log))).To(Equal(reflect.TypeOf(&Adapter{})))
		})
	})

	Context("When EnsureOwnerReferenceIsSet is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.releasePlan)
		})

		BeforeEach(func() {
			adapter = createReleasePlanAndAdapter()
		})

		It("should set the owner reference", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
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

			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
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

	createReleasePlanAndAdapter = func() *Adapter {
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

		return NewAdapter(ctx, k8sClient, releasePlan, loader.NewMockLoader(), ctrl.Log)
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
	}

	deleteResources = func() {
		Expect(k8sClient.Delete(ctx, application)).To(Succeed())
	}

})
