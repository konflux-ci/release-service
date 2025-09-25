/*
Copyright 2025.

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

package git

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Git References", func() {
	Describe("IsSHA", func() {
		It("should return true for valid 40-character SHA", func() {
			sha := "1234567890abcdef1234567890abcdef12345678"
			Expect(IsSHA(sha)).To(BeTrue())
		})

		It("should return false for short string", func() {
			Expect(IsSHA("abc123")).To(BeFalse())
		})

		It("should return false for long string", func() {
			longString := "1234567890abcdef1234567890abcdef1234567890abcdef"
			Expect(IsSHA(longString)).To(BeFalse())
		})

		It("should return false for non-hex characters", func() {
			Expect(IsSHA("1234567890abcdef1234567890abcdef1234567g")).To(BeFalse())
		})
	})

	Describe("isRateLimitError", func() {
		It("should return false for nil error", func() {
			Expect(isRateLimitError(nil)).To(BeFalse())
		})

		It("should return true for rate limit errors", func() {
			err := fmt.Errorf("rate limit exceeded")
			Expect(isRateLimitError(err)).To(BeTrue())

			err = fmt.Errorf("API rate limit")
			Expect(isRateLimitError(err)).To(BeTrue())

			err = fmt.Errorf("status code 403")
			Expect(isRateLimitError(err)).To(BeTrue())
		})

		It("should return false for other errors", func() {
			err := fmt.Errorf("authentication required")
			Expect(isRateLimitError(err)).To(BeFalse())

			err = fmt.Errorf("connection refused")
			Expect(isRateLimitError(err)).To(BeFalse())

			err = fmt.Errorf("some other error")
			Expect(isRateLimitError(err)).To(BeFalse())
		})
	})

	Describe("ResolveBranchToSHA", func() {
		It("should return SHA directly if input is already a SHA", func() {
			sha := "1234567890abcdef1234567890abcdef12345678"
			result, err := ResolveBranchToSHA("https://github.com/org/repo.git", sha)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(sha))
		})

		It("should return error for empty URL", func() {
			_, err := ResolveBranchToSHA("", "main")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid configuration"))
			Expect(err.Error()).To(ContainSubstring("repository URL and revision cannot be empty"))
		})

		It("should return error for empty revision", func() {
			_, err := ResolveBranchToSHA("https://github.com/org/repo.git", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid configuration"))
			Expect(err.Error()).To(ContainSubstring("repository URL and revision cannot be empty"))
		})

		It("should return error for branch not found in repository", func() {
			// Use a real public repository but with a non-existent branch
			_, err := ResolveBranchToSHA("https://github.com/octocat/Hello-World.git", "nonexistent-branch-xyz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("branch lookup failed"))
			Expect(err.Error()).To(ContainSubstring("branch 'nonexistent-branch-xyz' not found"))
		})

		It("should resolve a valid branch to a SHA", func() {
			// Use a real public repository with a known branch
			resolvedSHA, err := ResolveBranchToSHA("https://github.com/octocat/Hello-World.git", "master")
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedSHA).NotTo(BeEmpty())
			Expect(IsSHA(resolvedSHA)).To(BeTrue())
		})

		It("should normalize GitHub URLs by adding .git suffix when missing", func() {
			// Test URL without .git suffix
			resolvedSHA, err := ResolveBranchToSHA("https://github.com/octocat/Hello-World", "master")
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedSHA).NotTo(BeEmpty())
			Expect(IsSHA(resolvedSHA)).To(BeTrue())
		})
	})
})
