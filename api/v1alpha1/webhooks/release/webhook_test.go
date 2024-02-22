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

	"github.com/redhat-appstudio/release-service/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Release validation webhook", func() {
	var release *v1alpha1.Release
	var releasePlan *v1alpha1.ReleasePlan

	BeforeEach(func() {
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
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, release)
	})

	When("release CR fields are updated", func() {
		It("Should error out when updating the resource", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, release)).Should(Succeed())

			// Try to update the Release snapshot
			release.Spec.Snapshot = "another-snapshot"

			err := k8sClient.Update(ctx, release)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("release resources spec cannot be updated"))
		})

		It("Should not error out when updating the resource metadata", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, release)).Should(Succeed())

			// Try to update the Release annotations
			release.ObjectMeta.Annotations = map[string]string{
				"foo": "bar",
			}

			Expect(k8sClient.Update(ctx, release)).ShouldNot(HaveOccurred())
		})
	})

	When("ValidateDelete method is called", func() {
		It("should return nil", func() {
			release := &v1alpha1.Release{}
			Expect(webhook.ValidateDelete(ctx, release)).To(BeNil())
		})
	})

	When("a new Release is created", func() {
		It("should set GracePeriodDays to ReleasePlan's value and return nil", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, releasePlan)).Should(Succeed())
			Expect(k8sClient.Create(ctx, release)).Should(Succeed())
			Expect(release.Spec.GracePeriodDays).To(Equal(releasePlan.Spec.ReleaseGracePeriodDays))

			Expect(k8sClient.Delete(ctx, releasePlan)).Should(Succeed())
		})

		It("should return nil and keep the default value of a go `int` for GracePeriodDays when the specified ReleasePlan does not exist", func() {
			ctx := context.Background()
			Expect(k8sClient.Create(ctx, release)).Should(Succeed())
			Expect(release.Spec.GracePeriodDays).To(Equal(0))
		})
	})
})
