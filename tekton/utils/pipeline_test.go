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
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

var _ = Describe("Pipeline", func() {
	var (
		clusterRef PipelineRef
		gitRef     PipelineRef
		bundleRef  PipelineRef
	)

	BeforeEach(func() {
		clusterRef = PipelineRef{
			Resolver: "cluster",
			Params: []Param{
				{Name: "kind", Value: "pipeline"},
				{Name: "name", Value: "my-cluster-pipeline"},
				{Name: "namespace", Value: "my-namespace"},
			},
		}
		gitRef = PipelineRef{
			Resolver: "git",
			Params: []Param{
				{Name: "url", Value: "my-git-url"},
				{Name: "revision", Value: "my-revision"},
				{Name: "pathInRepo", Value: "my-path-in-repo"},
			},
		}
		bundleRef = PipelineRef{
			Resolver: "bundles",
			Params: []Param{
				{Name: "bundle", Value: "my-bundle"},
				{Name: "name", Value: "my-pipeline"},
				{Name: "kind", Value: "pipeline"},
			},
		}
	})

	When("GetGitResolverParams method is called", func() {
		It("should return all the common parameters", func() {
			url, revision, pathInRepo, err := gitRef.GetGitResolverParams()
			Expect(url).To(Equal("my-git-url"))
			Expect(revision).To(Equal("my-revision"))
			Expect(pathInRepo).To(Equal("my-path-in-repo"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail if a git resolver is not used", func() {
			_, _, _, err := bundleRef.GetGitResolverParams()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not a git ref"))
		})
	})

	When("GetUrl method is called", func() {
		It("should return the url if it exists", func() {
			url, err := gitRef.GetUrl()
			Expect(url).To(Equal("my-git-url"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return the url if it does not exist", func() {
			url, err := bundleRef.GetUrl()
			Expect(url).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})
	})

	When("GetRevision method is called", func() {
		It("should return the revision if it exists", func() {
			revision, err := gitRef.GetRevision()
			Expect(revision).To(Equal("my-revision"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not return the revision if it does not exist", func() {
			revision, err := bundleRef.GetRevision()
			Expect(revision).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})
	})

	When("ToTektonPipelineRef method is called", func() {
		It("should return Tekton PipelineRef representation of the PipelineRef", func() {
			ref := clusterRef.ToTektonPipelineRef()
			Expect(string(ref.ResolverRef.Resolver)).To(Equal("cluster"))
			params := ref.ResolverRef.Params
			Expect(params[0].Name).To(Equal("kind"))
			Expect(params[0].Value.StringVal).To(Equal("pipeline"))
			Expect(params[1].Name).To(Equal("name"))
			Expect(params[1].Value.StringVal).To(Equal("my-cluster-pipeline"))
			Expect(params[2].Name).To(Equal("namespace"))
			Expect(params[2].Value.StringVal).To(Equal("my-namespace"))

			ref = gitRef.ToTektonPipelineRef()
			Expect(string(ref.ResolverRef.Resolver)).To(Equal("git"))
			params = ref.ResolverRef.Params
			Expect(params[0].Name).To(Equal("url"))
			Expect(params[0].Value.StringVal).To(Equal("my-git-url"))
			Expect(params[1].Name).To(Equal("revision"))
			Expect(params[1].Value.StringVal).To(Equal("my-revision"))
			Expect(params[2].Name).To(Equal("pathInRepo"))
			Expect(params[2].Value.StringVal).To(Equal("my-path-in-repo"))

			ref = bundleRef.ToTektonPipelineRef()
			Expect(string(ref.ResolverRef.Resolver)).To(Equal("bundles"))
			params = ref.ResolverRef.Params
			Expect(params[0].Name).To(Equal("bundle"))
			Expect(params[0].Value.StringVal).To(Equal("my-bundle"))
			Expect(params[1].Name).To(Equal("name"))
			Expect(params[1].Value.StringVal).To(Equal("my-pipeline"))
			Expect(params[2].Name).To(Equal("kind"))
			Expect(params[2].Value.StringVal).To(Equal("pipeline"))
		})
	})

	When("GetTektonParams method is called", func() {
		It("should return a tekton Param list", func() {
			parameterizedPipeline := ParameterizedPipeline{}
			parameterizedPipeline.Params = []Param{
				{Name: "parameter1", Value: "value1"},
				{Name: "parameter2", Value: "value2"},
			}

			params := parameterizedPipeline.GetTektonParams()
			Expect(reflect.TypeOf(params[0])).To(Equal(reflect.TypeOf(tektonv1.Param{})))
			Expect(params[0].Name).To(Equal("parameter1"))
			Expect(params[0].Value.StringVal).To(Equal("value1"))

			Expect(reflect.TypeOf(params[1])).To(Equal(reflect.TypeOf(tektonv1.Param{})))
			Expect(params[1].Name).To(Equal("parameter2"))
			Expect(params[1].Value.StringVal).To(Equal("value2"))
		})
	})

	When("IsClusterScoped method is called", func() {
		It("should return true for a cluster pipeline", func() {
			Expect(clusterRef.IsClusterScoped()).To(BeTrue())
		})

		It("should return false for non-cluster pipelines", func() {
			Expect(gitRef.IsClusterScoped()).To(BeFalse())
			Expect(bundleRef.IsClusterScoped()).To(BeFalse())
		})
	})

	Describe("Git Reference SHA Resolution", func() {
		Context("when validating hex strings", func() {
			It("should correctly identify valid SHA (lowercase)", func() {
				result := isHexString("abcdef123456789abcdef123456789abcdef1234")
				Expect(result).To(BeTrue())
			})

			It("should correctly identify valid SHA (uppercase)", func() {
				result := isHexString("ABCDEF123456789ABCDEF123456789ABCDEF1234")
				Expect(result).To(BeTrue())
			})

			It("should correctly identify valid SHA (mixed case)", func() {
				result := isHexString("aBcDeF123456789aBcDeF123456789aBcDeF1234")
				Expect(result).To(BeTrue())
			})

			It("should reject invalid strings with special characters", func() {
				result := isHexString("abcdef123456789-bcdef123456789abcdef1234")
				Expect(result).To(BeFalse())
			})

			It("should reject invalid strings with letters outside hex range", func() {
				result := isHexString("ghijkl123456789abcdef123456789abcdef1234")
				Expect(result).To(BeFalse())
			})

			It("should accept empty string", func() {
				result := isHexString("")
				Expect(result).To(BeTrue())
			})
		})

		Context("when resolving git references to commit SHA", func() {
			It("should return error for non-git resolver", func() {
				pipelineRef := &PipelineRef{
					Resolver: "cluster",
					Params:   []Param{},
				}

				ctx := context.Background()
				sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

				Expect(err).To(HaveOccurred())
				Expect(sha).To(BeEmpty())
			})

			It("should return SHA for git resolver with existing SHA", func() {
				pipelineRef := &PipelineRef{
					Resolver: "git",
					Params: []Param{
						{Name: "url", Value: "https://github.com/konflux-ci/release-service.git"},
						{Name: "revision", Value: "abcdef1234567890abcdef1234567890abcdef12"},
						{Name: "pathInRepo", Value: "pipelines/test.yaml"},
					},
				}

				ctx := context.Background()
				sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(sha).NotTo(BeEmpty())
				Expect(len(sha)).To(Equal(40)) // Valid SHA format
			})

			It("should return error for git resolver with missing parameters", func() {
				pipelineRef := &PipelineRef{
					Resolver: "git",
					Params:   []Param{},
				}

				ctx := context.Background()
				sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

				Expect(err).To(HaveOccurred())
				Expect(sha).To(BeEmpty())
			})

			Context("when GitHub API returns invalid SHA format", func() {
				var mockServer *httptest.Server

				BeforeEach(func() {
					mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.Contains(r.URL.Path, "/commits/") {
							if strings.Contains(r.URL.Path, "invalid-short") {
								w.WriteHeader(http.StatusOK)
								_ = json.NewEncoder(w).Encode(map[string]string{"sha": "too-short"})
							} else if strings.Contains(r.URL.Path, "invalid-chars") {
								w.WriteHeader(http.StatusOK)
								_ = json.NewEncoder(w).Encode(map[string]string{"sha": "gggggggggggggggggggggggggggggggggggggggg"}) // Invalid hex chars
							} else if strings.Contains(r.URL.Path, "invalid-long") {
								w.WriteHeader(http.StatusOK)
								_ = json.NewEncoder(w).Encode(map[string]string{"sha": "this-is-way-too-long-to-be-a-valid-sha-format-and-should-fail-validation"})
							} else {
								w.WriteHeader(http.StatusNotFound)
							}
						}
					}))
				})

				AfterEach(func() {
					if mockServer != nil {
						mockServer.Close()
					}
				})

				It("should return error when GitHub API returns too short SHA", func() {
					pipelineRef := &PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "https://github.com/test/repo.git"},
							{Name: "revision", Value: "invalid-short"},
							{Name: "pathInRepo", Value: "pipelines/test.yaml"},
						},
					}

					ctx := context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
					sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("GitHub API returned invalid SHA format"))
					Expect(sha).To(BeEmpty())
				})

				It("should return error when GitHub API returns SHA with invalid characters", func() {
					pipelineRef := &PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "https://github.com/test/repo.git"},
							{Name: "revision", Value: "invalid-chars"},
							{Name: "pathInRepo", Value: "pipelines/test.yaml"},
						},
					}

					ctx := context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
					sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("GitHub API returned invalid SHA format"))
					Expect(sha).To(BeEmpty())
				})

				It("should return error when GitHub API returns too long SHA", func() {
					pipelineRef := &PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "https://github.com/test/repo.git"},
							{Name: "revision", Value: "invalid-long"},
							{Name: "pathInRepo", Value: "pipelines/test.yaml"},
						},
					}

					ctx := context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
					sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("GitHub API returned invalid SHA format"))
					Expect(sha).To(BeEmpty())
				})
			})

			Context("when GitHub API returns 404 errors", func() {
				var mockServer *httptest.Server

				BeforeEach(func() {
					mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.Contains(r.URL.Path, "/commits/") {
							w.WriteHeader(http.StatusNotFound)
						}
					}))
				})

				AfterEach(func() {
					if mockServer != nil {
						mockServer.Close()
					}
				})

				It("should return error when branch/repo doesn't exist (404)", func() {
					pipelineRef := &PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "https://github.com/test/repo.git"},
							{Name: "revision", Value: "nonexistent-branch"},
							{Name: "pathInRepo", Value: "pipelines/test.yaml"},
						},
					}

					ctx := context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
					sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("GitHub API returned status 404"))
					Expect(sha).To(BeEmpty())
				})
			})

			Context("when GitHub API returns server errors", func() {
				var mockServer *httptest.Server

				BeforeEach(func() {
					mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.Contains(r.URL.Path, "/commits/") {
							if strings.Contains(r.URL.Path, "rate-limited") {
								w.WriteHeader(http.StatusTooManyRequests) // 429
							} else if strings.Contains(r.URL.Path, "forbidden") {
								w.WriteHeader(http.StatusForbidden) // 403
							} else if strings.Contains(r.URL.Path, "server-error") {
								w.WriteHeader(http.StatusInternalServerError) // 500
							} else if strings.Contains(r.URL.Path, "bad-json") {
								w.WriteHeader(http.StatusOK)
								_, _ = w.Write([]byte("invalid json response"))
							}
						}
					}))
				})

				AfterEach(func() {
					if mockServer != nil {
						mockServer.Close()
					}
				})

				It("should return error when rate limited (429)", func() {
					pipelineRef := &PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "https://github.com/test/repo.git"},
							{Name: "revision", Value: "rate-limited"},
							{Name: "pathInRepo", Value: "pipelines/test.yaml"},
						},
					}

					ctx := context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
					sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("GitHub API returned status 429"))
					Expect(sha).To(BeEmpty())
				})

				It("should return error when forbidden (403)", func() {
					pipelineRef := &PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "https://github.com/test/repo.git"},
							{Name: "revision", Value: "forbidden"},
							{Name: "pathInRepo", Value: "pipelines/test.yaml"},
						},
					}

					ctx := context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
					sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("GitHub API returned status 403"))
					Expect(sha).To(BeEmpty())
				})

				It("should return error when server error (500)", func() {
					pipelineRef := &PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "https://github.com/test/repo.git"},
							{Name: "revision", Value: "server-error"},
							{Name: "pathInRepo", Value: "pipelines/test.yaml"},
						},
					}

					ctx := context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
					sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("GitHub API returned status 500"))
					Expect(sha).To(BeEmpty())
				})

				It("should return error when JSON parsing fails", func() {
					pipelineRef := &PipelineRef{
						Resolver: "git",
						Params: []Param{
							{Name: "url", Value: "https://github.com/test/repo.git"},
							{Name: "revision", Value: "bad-json"},
							{Name: "pathInRepo", Value: "pipelines/test.yaml"},
						},
					}

					ctx := context.WithValue(context.Background(), GitHubAPIBaseURLKey, mockServer.URL)
					sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("failed to parse GitHub response"))
					Expect(sha).To(BeEmpty())
				})
			})
		})

		Context("when creating pipeline ref with resolved SHA", func() {
			It("should create new ref for git resolver with existing SHA", func() {
				pipelineRef := &PipelineRef{
					Resolver: "git",
					Params: []Param{
						{Name: "url", Value: "https://github.com/konflux-ci/release-service.git"},
						{Name: "revision", Value: "abcdef1234567890abcdef1234567890abcdef12"},
						{Name: "pathInRepo", Value: "pipelines/test.yaml"},
					},
				}

				ctx := context.Background()
				newRef, err := pipelineRef.CreatePipelineRefWithResolvedSHA(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(newRef).NotTo(BeNil())
				Expect(newRef.Resolver).To(Equal(pipelineRef.Resolver))
				Expect(len(newRef.Params)).To(Equal(len(pipelineRef.Params)))

				var foundRevision bool
				for _, param := range newRef.Params {
					if param.Name == "revision" {
						foundRevision = true
						Expect(isSHA(param.Value)).To(BeTrue())
						break
					}
				}
				Expect(foundRevision).To(BeTrue())
			})

			It("should return error for non-git resolver", func() {
				pipelineRef := &PipelineRef{
					Resolver: "cluster",
					Params:   []Param{},
				}

				ctx := context.Background()
				newRef, err := pipelineRef.CreatePipelineRefWithResolvedSHA(ctx)

				Expect(err).To(HaveOccurred())
				Expect(newRef).To(BeNil())
			})

			It("should return error for git resolver with missing parameters", func() {
				pipelineRef := &PipelineRef{
					Resolver: "git",
					Params:   []Param{},
				}

				ctx := context.Background()
				newRef, err := pipelineRef.CreatePipelineRefWithResolvedSHA(ctx)

				Expect(err).To(HaveOccurred())
				Expect(newRef).To(BeNil())
			})

		})

		Context("when testing critical configuration errors", func() {
			It("should return error for invalid URL format", func() {
				pipelineRef := &PipelineRef{
					Resolver: "git",
					Params: []Param{
						{Name: "url", Value: "not-a-valid-url"},
						{Name: "revision", Value: "main"},
						{Name: "pathInRepo", Value: "pipelines/test.yaml"},
					},
				}

				ctx := context.Background()
				sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid GitHub URL format"))
				Expect(sha).To(BeEmpty())
			})

			It("should return error for missing git resolver parameters", func() {
				pipelineRef := &PipelineRef{
					Resolver: "git",
					Params:   []Param{},
				}

				ctx := context.Background()
				sha, err := pipelineRef.ResolveGitReferenceToCommitSHA(ctx)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid GitHub URL format"))
				Expect(sha).To(BeEmpty())
			})
		})

	})

})
