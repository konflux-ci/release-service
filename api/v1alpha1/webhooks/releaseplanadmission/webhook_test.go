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

package releaseplanadmission

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/redhat-appstudio/release-service/metadata"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("ReleasePlanAdmission webhook", func() {
	var releasePlanAdmission *v1alpha1.ReleasePlanAdmission

	BeforeEach(func() {
		releasePlanAdmission = &v1alpha1.ReleasePlanAdmission{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "ReleasePlanAdmission",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "releaseplanadmission",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleasePlanAdmissionSpec{
				DisplayName:     "Test release plan",
				Application:     "application",
				Origin:          "default",
				Environment:     "environment",
				ReleaseStrategy: "strategy",
			},
		}
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, releasePlanAdmission)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	When("a ReleasePlanAdmission is created without the auto-release label", func() {
		It("should get the label added with its value set to true", func() {
			Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      releasePlanAdmission.Name,
					Namespace: releasePlanAdmission.Namespace,
				}, releasePlanAdmission)

				labelValue, ok := releasePlanAdmission.GetLabels()[metadata.AutoReleaseLabel]

				return err == nil && ok && labelValue == "true"
			}, timeout).Should(BeTrue())
		})
	})

	When("a ReleasePlanAdmission is created with an invalid auto-release label value", func() {
		It("should get rejected until the value is valid", func() {
			releasePlanAdmission.Labels = map[string]string{metadata.AutoReleaseLabel: "foo"}
			err := k8sClient.Create(ctx, releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'%s' label can only be set to true or false", metadata.AutoReleaseLabel))
		})
	})

	When("a ReleasePlanAdmission is created with a valid auto-release label value", func() {
		It("shouldn't be modified", func() {
			By("setting label to true")
			localReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			localReleasePlanAdmission.Labels = map[string]string{metadata.AutoReleaseLabel: "true"}
			Expect(k8sClient.Create(ctx, localReleasePlanAdmission)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      localReleasePlanAdmission.Name,
					Namespace: localReleasePlanAdmission.Namespace,
				}, localReleasePlanAdmission)

				labelValue, ok := localReleasePlanAdmission.GetLabels()[metadata.AutoReleaseLabel]

				return err == nil && ok && labelValue == "true"
			}, timeout).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, localReleasePlanAdmission)).To(Succeed())

			By("setting label to false")
			localReleasePlanAdmission = releasePlanAdmission.DeepCopy()
			localReleasePlanAdmission.Labels = map[string]string{metadata.AutoReleaseLabel: "false"}
			Expect(k8sClient.Create(ctx, localReleasePlanAdmission)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      localReleasePlanAdmission.Name,
					Namespace: localReleasePlanAdmission.Namespace,
				}, localReleasePlanAdmission)

				labelValue, ok := localReleasePlanAdmission.GetLabels()[metadata.AutoReleaseLabel]

				return err == nil && ok && labelValue == "false"
			}, timeout).Should(BeTrue())
		})
	})

	When("a ReleasePlanAdmission is updated using an invalid auto-release label value", func() {
		It("shouldn't be modified", func() {
			Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())
			releasePlanAdmission.GetLabels()[metadata.AutoReleaseLabel] = "foo"
			err := k8sClient.Update(ctx, releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("'%s' label can only be set to true or false", metadata.AutoReleaseLabel))
		})
	})

	When("ValidateDelete method is called", func() {
		It("should return nil", func() {
			releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{}
			Expect(webhook.ValidateDelete(ctx, releasePlanAdmission)).To(BeNil())
		})
	})
})
