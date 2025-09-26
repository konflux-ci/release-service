/*
Copyright 2023.

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

package utils

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Git Reference Resolver", func() {
	Describe("ResolveBranchToSHA", func() {
		Context("when successfully resolving branches", func() {
			var (
				mockServer *httptest.Server
				ctx        context.Context
			)

			BeforeEach(func() {
				mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")

					if r.URL.Path == "/repos/test/repo/commits/main" {
						response := map[string]string{"sha": "abc123def456789abc123def456789abc123def4"}
						_ = json.NewEncoder(w).Encode(response)
					} else if r.URL.Path == "/repos/test/repo/commits/master" {
						response := map[string]string{"sha": "def456abc123789def456abc123789def456abc1"}
						_ = json.NewEncoder(w).Encode(response)
					} else if r.URL.Path == "/repos/test/repo/commits/develop" {
						response := map[string]string{"sha": "789abc123def456789abc123def456789abc123d"}
						_ = json.NewEncoder(w).Encode(response)
					} else if r.URL.Path == "/repos/konflux-ci/release-service/commits/main" {
						response := map[string]string{"sha": "1234567890abcdef1234567890abcdef12345678"}
						_ = json.NewEncoder(w).Encode(response)
					} else {
						w.WriteHeader(http.StatusNotFound)
					}
				}))

				ctx = context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
			})

			AfterEach(func() {
				if mockServer != nil {
					mockServer.Close()
				}
			})

			It("should successfully resolve main branch to SHA", func() {
				sha, err := ResolveBranchToSHA(ctx, "https://github.com/test/repo.git", "main")
				Expect(err).NotTo(HaveOccurred())
				Expect(sha).To(Equal("abc123def456789abc123def456789abc123def4"))
				Expect(len(sha)).To(Equal(40))
			})

			It("should successfully resolve master branch to SHA", func() {
				sha, err := ResolveBranchToSHA(ctx, "https://github.com/test/repo.git", "master")
				Expect(err).NotTo(HaveOccurred())
				Expect(sha).To(Equal("def456abc123789def456abc123789def456abc1"))
				Expect(len(sha)).To(Equal(40))
			})

			It("should successfully resolve develop branch to SHA", func() {
				sha, err := ResolveBranchToSHA(ctx, "https://github.com/test/repo.git", "develop")
				Expect(err).NotTo(HaveOccurred())
				Expect(sha).To(Equal("789abc123def456789abc123def456789abc123d"))
				Expect(len(sha)).To(Equal(40))
			})

			It("should work with URLs without .git suffix", func() {
				sha, err := ResolveBranchToSHA(ctx, "https://github.com/test/repo", "main")
				Expect(err).NotTo(HaveOccurred())
				Expect(sha).To(Equal("abc123def456789abc123def456789abc123def4"))
				Expect(len(sha)).To(Equal(40))
			})

			It("should work with different repository paths", func() {
				sha, err := ResolveBranchToSHA(ctx, "https://github.com/konflux-ci/release-service.git", "main")
				Expect(err).NotTo(HaveOccurred())
				Expect(sha).To(Equal("1234567890abcdef1234567890abcdef12345678"))
				Expect(len(sha)).To(Equal(40))
			})

			It("should handle timeout gracefully", func() {
				shortCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
				defer cancel()

				_, err := ResolveBranchToSHA(shortCtx, "https://github.com/test/repo.git", "main")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when validating URL parsing", func() {
			It("should return error for truly invalid URL with less than 2 components", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				_, err := ResolveBranchToSHA(ctx, "single", "main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid GitHub URL format"))
			})

			It("should return error for empty URL", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				_, err := ResolveBranchToSHA(ctx, "", "main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid GitHub URL format"))
			})

			It("should return error for URL with only one slash", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				_, err := ResolveBranchToSHA(ctx, "a/", "main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).ToNot(ContainSubstring("invalid GitHub URL format"))
			})
		})

		Context("when handling network errors", func() {
			It("should handle timeout for malformed but parseable URLs", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				_, err := ResolveBranchToSHA(ctx, "https://github.com/single", "main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).ToNot(ContainSubstring("invalid GitHub URL format"))
			})

			It("should handle 404 for non-existent repos", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				_, err := ResolveBranchToSHA(ctx, "https://github.com/nonexistent/repo", "main")
				Expect(err).To(HaveOccurred())
			})

			It("should respect context timeout", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()

				_, err := ResolveBranchToSHA(ctx, "https://github.com/valid/repo.git", "main")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when calling real GitHub API", func() {
			It("should successfully resolve a real repository branch to a valid SHA", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				sha, err := ResolveBranchToSHA(ctx, "https://github.com/golang/go.git", "master")

				Expect(err).NotTo(HaveOccurred())
				Expect(sha).NotTo(BeEmpty())

				Expect(sha).To(MatchRegexp("^[a-f0-9]{40}$"))
				Expect(len(sha)).To(Equal(40))
			})

			It("should handle non-existent branch on real repository", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				_, err := ResolveBranchToSHA(ctx, "https://github.com/golang/go.git", "definitely-nonexistent-branch-name-12345")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("GitHub API returned status"))
			})

			It("should handle non-existent repository", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				_, err := ResolveBranchToSHA(ctx, "https://github.com/definitely-nonexistent-org-12345/nonexistent-repo.git", "main")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("GitHub API returned status"))
			})
		})
	})
})
