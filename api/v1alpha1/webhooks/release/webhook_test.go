//
// Copyright 2022 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package release

import (
	"context"

	toolkit "github.com/konflux-ci/operator-toolkit/loader"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Release validation webhook", func() {
	var (
		createResources func()

		release     *v1alpha1.Release
		releasePlan *v1alpha1.ReleasePlan
	)

	When("Default method is called", func() {
		var mockedWebhook *Webhook

		BeforeEach(func() {
			createResources()

			mockedWebhook = &Webhook{
				client: k8sClient,
				loader: loader.NewMockLoader(),
			}
		})

		It("should set GracePeriodDays to ReleasePlan's value and return nil", func() {
			mockedCtx := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
			})

			Expect(mockedWebhook.Default(mockedCtx, release)).To(BeNil())
			Expect(release.Spec.GracePeriodDays).To(Equal(releasePlan.Spec.ReleaseGracePeriodDays))
		})

		It("should return nil and keep the default value of a go `int` for GracePeriodDays when the specified ReleasePlan does not exist", func() {
			mockedCtx := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			Expect(mockedWebhook.Default(mockedCtx, release)).To(BeNil())
			Expect(release.Spec.GracePeriodDays).To(Equal(0))
		})
	})

	When("When ValidateUpdate is called", func() {
		It("should error out when updating the resource", func() {
			updatedRelease := release.DeepCopy()
			updatedRelease.Spec.Snapshot = "another-snapshot"

			_, err := webhook.ValidateUpdate(ctx, release, updatedRelease)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("release resources spec cannot be updated"))
		})

		It("should not error out when updating the resource metadata", func() {
			ctx := context.Background()

			updatedRelease := release.DeepCopy()
			updatedRelease.ObjectMeta.Annotations = map[string]string{
				"foo": "bar",
			}

			_, err := webhook.ValidateUpdate(ctx, release, updatedRelease)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("ValidateDelete method is called", func() {
		It("should return nil", func() {
			_, err := webhook.ValidateDelete(ctx, &v1alpha1.Release{})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	createResources = func() {
		release = &v1alpha1.Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-release-",
				Namespace:    "default",
			},
			Spec: v1alpha1.ReleaseSpec{
				Snapshot:    "test-snapshot",
				ReleasePlan: "test-releaseplan",
			},
		}

		releasePlan = &v1alpha1.ReleasePlan{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "ReleasePlan",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-releaseplan",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleasePlanSpec{
				Application:            "test-application",
				Target:                 "default",
				ReleaseGracePeriodDays: 7,
			},
		}
	}
})
