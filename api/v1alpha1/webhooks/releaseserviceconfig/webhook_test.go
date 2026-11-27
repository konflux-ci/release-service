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

package releaseserviceconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konflux-ci/release-service/api/v1alpha1"
)

var _ = Describe("ReleaseServiceConfig validating webhook", func() {
	var releaseServiceConfig *v1alpha1.ReleaseServiceConfig

	BeforeEach(func() {
		releaseServiceConfig = &v1alpha1.ReleaseServiceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rsc",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleaseServiceConfigSpec{},
		}
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, releaseServiceConfig)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	Context("when validating AllowedGitResolvers", func() {
		It("should accept valid exact string URLs", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{
				{
					Url:       "https://github.com/org/repo",
					Revisions: []string{"main"},
				},
			}
			Expect(k8sClient.Create(ctx, releaseServiceConfig)).To(Succeed())
		})

		It("should accept valid regex URLs", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{
				{
					Url:       "^https://github\\.com/org/.*$",
					Revisions: []string{"main"},
				},
			}
			Expect(k8sClient.Create(ctx, releaseServiceConfig)).To(Succeed())
		})

		It("should accept valid regex revisions", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{
				{
					Url:       "https://github.com/org/repo",
					Revisions: []string{"^release-.*$", "main"},
				},
			}
			Expect(k8sClient.Create(ctx, releaseServiceConfig)).To(Succeed())
		})

		It("should reject invalid regex URL", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{
				{
					Url:       "^[invalid$",
					Revisions: []string{"main"},
				},
			}
			err := k8sClient.Create(ctx, releaseServiceConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid regex patterns"))
		})

		It("should reject invalid regex revision", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{
				{
					Url:       "https://github.com/org/repo",
					Revisions: []string{"^[unclosed$"}, // Invalid regex: unclosed character class
				},
			}
			err := k8sClient.Create(ctx, releaseServiceConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid regex patterns"))
		})

		It("should accept empty AllowedGitResolvers", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{}
			Expect(k8sClient.Create(ctx, releaseServiceConfig)).To(Succeed())
		})

		It("should accept nil AllowedGitResolvers", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = nil
			Expect(k8sClient.Create(ctx, releaseServiceConfig)).To(Succeed())
		})

		It("should reject multiple invalid patterns", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{
				{
					Url:       "^[invalid$",               // Invalid: unclosed character class
					Revisions: []string{"^[alsoinvalid$"}, // Invalid: unclosed character class
				},
			}
			err := k8sClient.Create(ctx, releaseServiceConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid regex patterns"))
		})

		It("should reject invalid regex on update", func() {
			// Create a valid config first
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{
				{
					Url:       "https://github.com/org/repo",
					Revisions: []string{"main"},
				},
			}
			Expect(k8sClient.Create(ctx, releaseServiceConfig)).To(Succeed())

			// Update with invalid regex
			releaseServiceConfig.Spec.AllowedGitResolvers = []v1alpha1.GitResolverRule{
				{
					Url:       "^[invalid$", // Invalid: unclosed character class
					Revisions: []string{"main"},
				},
			}
			err := k8sClient.Update(ctx, releaseServiceConfig)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid regex patterns"))
		})
	})
})
