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

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ReleaseServiceConfig type", func() {
	When("IsPipelineOverridden method is called", func() {
		var releaseServiceConfig *ReleaseServiceConfig

		BeforeEach(func() {
			releaseServiceConfig = &ReleaseServiceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: "default",
				},
			}
		})

		It("should return false if the resource is not overridden", func() {
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeFalse())
		})

		It("should return true if the resource is overridden", func() {
			releaseServiceConfig.Spec.EmptyDirOverrides = []EmptyDirOverrides{
				{"foo", "bar", "baz"},
			}
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeTrue())
		})

		It("should return true if the resource is overridden using a regex expression in the url field", func() {
			releaseServiceConfig.Spec.EmptyDirOverrides = []EmptyDirOverrides{
				{".*", "bar", "baz"},
			}
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeTrue())
		})

		It("should return true if the resource is overridden using a regex expression in the revision field", func() {
			releaseServiceConfig.Spec.EmptyDirOverrides = []EmptyDirOverrides{
				{"foo", ".*", "baz"},
			}
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeTrue())
		})

		It("should return false if the resource is overridden using a regex expression in the pathInRepo field", func() {
			releaseServiceConfig.Spec.EmptyDirOverrides = []EmptyDirOverrides{
				{"foo", "bar", ".*"},
			}
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeFalse())
		})
	})

	When("IsGitResolverAllowed method is called", func() {
		var releaseServiceConfig *ReleaseServiceConfig

		BeforeEach(func() {
			releaseServiceConfig = &ReleaseServiceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: "default",
				},
			}
		})

		It("should return true when AllowedGitResolvers is empty", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{}
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/any/repo",
				"any-branch",
			)).To(BeTrue())
		})

		It("should return true when URL and revision match an allowed entry using exact string match", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL:       "https://github.com/konflux-ci/release-service-catalog",
					Revisions: []string{"main", "production"},
				},
			}
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/release-service-catalog",
				"main",
			)).To(BeTrue())
		})

		It("should return false when URL has .git suffix but allowed entry does not (exact match)", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL:       "https://github.com/konflux-ci/release-service-catalog",
					Revisions: []string{"main"},
				},
			}
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/release-service-catalog.git",
				"main",
			)).To(BeFalse())
		})

		It("should return true when using regex pattern that matches both with and without .git", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL:       "^https://github\\.com/konflux-ci/release-service-catalog(\\.git)?$",
					Revisions: []string{"main", "production"},
				},
			}
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/release-service-catalog",
				"main",
			)).To(BeTrue())

			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/release-service-catalog.git",
				"main",
			)).To(BeTrue())
		})

		It("should return false when URL matches but revision does not", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL:       "https://github.com/konflux-ci/release-service-catalog",
					Revisions: []string{"main", "production"},
				},
			}
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/release-service-catalog",
				"develop",
			)).To(BeFalse())
		})

		It("should return false when URL does not match", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL:       "https://github.com/konflux-ci/release-service-catalog",
					Revisions: []string{"main"},
				},
			}
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/unauthorized/repo",
				"main",
			)).To(BeFalse())
		})

		It("should return false when regex pattern does not match unintended URLs", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL:       "^https://github\\.com/konflux-ci/foo(\\.git)?$",
					Revisions: []string{"main"},
				},
			}
			// Should not match foo.bar
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/foo.bar",
				"main",
			)).To(BeFalse())
		})

		It("should return true for any revision when URL matches and has no revision restrictions", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL: "https://gitlab.cee.redhat.com/signing/signing",
					// No revisions specified
				},
			}
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://gitlab.cee.redhat.com/signing/signing",
				"any-branch-name",
			)).To(BeTrue())
		})

		It("should handle multiple allowed repositories with mixed exact and regex patterns", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL:       "https://github.com/konflux-ci/release-service-catalog",
					Revisions: []string{"main", "production"},
				},
				{
					URL:       "https://github.com/konflux-ci/release-service-catalog.git",
					Revisions: []string{"main", "production"},
				},
				{
					URL: "^https://gitlab\\.cee\\.redhat\\.com/signing/signing(\\.git)?$",
				},
			}

			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/release-service-catalog.git",
				"main",
			)).To(BeTrue())

			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://gitlab.cee.redhat.com/signing/signing",
				"feature-branch",
			)).To(BeTrue())

			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://gitlab.cee.redhat.com/signing/signing.git",
				"feature-branch",
			)).To(BeTrue())

			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/release-service-catalog",
				"develop",
			)).To(BeFalse())
		})

		It("should skip invalid regex patterns and continue checking other entries", func() {
			releaseServiceConfig.Spec.AllowedGitResolvers = []AllowedGitResolver{
				{
					URL:       "^[invalid(regex$", // Invalid regex
					Revisions: []string{"main"},
				},
				{
					URL:       "https://github.com/konflux-ci/release-service-catalog",
					Revisions: []string{"main"},
				},
			}
			Expect(releaseServiceConfig.IsGitResolverAllowed(
				"https://github.com/konflux-ci/release-service-catalog",
				"main",
			)).To(BeTrue())
		})
	})
})
