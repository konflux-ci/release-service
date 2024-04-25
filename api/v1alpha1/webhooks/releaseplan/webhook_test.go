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

package releaseplan

import (
	"github.com/konflux-ci/release-service/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/konflux-ci/release-service/metadata"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("ReleasePlan webhook", func() {
	var releasePlan *v1alpha1.ReleasePlan

	BeforeEach(func() {
		releasePlan = &v1alpha1.ReleasePlan{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "ReleasePlan",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "releaseplan",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleasePlanSpec{
				Application: "application",
				Target:      "default",
			},
		}
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, releasePlan)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	When("a ReleasePlan is created without the auto-release label", func() {
		It("should get the label added with its value set to true", func() {
			Expect(k8sClient.Create(ctx, releasePlan)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      releasePlan.Name,
					Namespace: releasePlan.Namespace,
				}, releasePlan)

				labelValue, ok := releasePlan.GetLabels()[metadata.AutoReleaseLabel]

				return err == nil && ok && labelValue == "true"
			}, timeout).Should(BeTrue())
		})
	})

	When("a ReleasePlan is created with an invalid auto-release label value", func() {
		It("should get rejected until the value is valid", func() {
			releasePlan.Labels = map[string]string{metadata.AutoReleaseLabel: "foo"}
			err := k8sClient.Create(ctx, releasePlan)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'%s' label can only be set to true or false", metadata.AutoReleaseLabel))
		})
	})

	When("a ReleasePlan is created with a valid auto-release label value", func() {
		It("shouldn't be modified", func() {
			// Using value "true"
			localReleasePlan := releasePlan.DeepCopy()
			localReleasePlan.Labels = map[string]string{metadata.AutoReleaseLabel: "true"}
			Expect(k8sClient.Create(ctx, localReleasePlan)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      localReleasePlan.Name,
					Namespace: localReleasePlan.Namespace,
				}, localReleasePlan)

				labelValue, ok := localReleasePlan.GetLabels()[metadata.AutoReleaseLabel]

				return err == nil && ok && labelValue == "true"
			}, timeout).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, localReleasePlan)).To(Succeed())

			// Using value "false"
			localReleasePlan = releasePlan.DeepCopy()
			localReleasePlan.Labels = map[string]string{metadata.AutoReleaseLabel: "false"}
			Expect(k8sClient.Create(ctx, localReleasePlan)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      localReleasePlan.Name,
					Namespace: localReleasePlan.Namespace,
				}, localReleasePlan)

				labelValue, ok := localReleasePlan.GetLabels()[metadata.AutoReleaseLabel]

				return err == nil && ok && labelValue == "false"
			}, timeout).Should(BeTrue())
		})
	})

	When("a ReleasePlan is updated using an invalid auto-release label value", func() {
		It("shouldn't be modified", func() {
			Expect(k8sClient.Create(ctx, releasePlan)).Should(Succeed())
			releasePlan.GetLabels()[metadata.AutoReleaseLabel] = "foo"
			err := k8sClient.Update(ctx, releasePlan)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'%s' label can only be set to true or false", metadata.AutoReleaseLabel))
		})
	})

	When("ValidateDelete method is called", func() {
		It("should return nil", func() {
			releasePlan := &v1alpha1.ReleasePlan{}
			Expect(webhook.ValidateDelete(ctx, releasePlan)).To(BeNil())
		})
	})
})
