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
	"fmt"
	"net/http"
	"strings"
	"time"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// Common branch names that typically cause race conditions when used as mutable references
var commonBranches = []string{
	"main",
	"production",
	"staging",
	"development",
}

// Repositories that are allowed for git reference rewriting
var allowedRepos = []string{
	"github.com/konflux-ci/release-service-catalog",
	"github.com/konflux-ci/tekton-integration-catalog",
	"github.com/konflux-ci/release-service",
	"github.com/test/repo", // test repo
}

// Context key for mock GitHub API base URL (used in tests)
type contextKey string

const GitHubAPIBaseURLKey contextKey = "github_api_base_url"

// WithGitReferenceRewriting enables git reference rewriting for race condition prevention
func (b *PipelineRunBuilder) WithGitReferenceRewriting(resolvedSHA, gitURL string) *PipelineRunBuilder {
	if b.gitRefManager == nil {
		b.gitRefManager = &GitReferenceManager{
			EnableAutoRewrite: false,
		}
	}

	b.gitRefManager.EnableAutoRewrite = true
	b.gitRefManager.CentralGitRevision = resolvedSHA
	b.gitRefManager.CentralGitURL = gitURL

	return b
}

// processGitReferences performs git reference rewriting
func (b *PipelineRunBuilder) processGitReferences() {
	if b.gitRefManager == nil || !b.gitRefManager.EnableAutoRewrite || b.pipelineRun.Spec.PipelineSpec == nil {
		return
	}

	for i := range b.pipelineRun.Spec.PipelineSpec.Tasks {
		b.rewriteTaskRef(&b.pipelineRun.Spec.PipelineSpec.Tasks[i])
	}

	for i := range b.pipelineRun.Spec.PipelineSpec.Finally {
		b.rewriteTaskRef(&b.pipelineRun.Spec.PipelineSpec.Finally[i])
	}
}

// rewriteTaskRef rewrites a single task's git reference
func (b *PipelineRunBuilder) rewriteTaskRef(task *tektonv1.PipelineTask) {
	if task.TaskRef == nil || task.TaskRef.ResolverRef.Resolver != "git" {
		return
	}

	var url, revision string
	var revisionIndex = -1

	for i, param := range task.TaskRef.ResolverRef.Params {
		switch param.Name {
		case "url":
			url = param.Value.StringVal
		case "revision":
			revision = param.Value.StringVal
			revisionIndex = i
		}
	}

	if revisionIndex >= 0 && shouldRewrite(url, revision, b.gitRefManager.CentralGitURL) {
		task.TaskRef.ResolverRef.Params[revisionIndex].Value.StringVal = b.gitRefManager.CentralGitRevision
	}
}

// matchesAllowedRepos checks if the URL matches any of the allowed repositories
func matchesAllowedRepos(url string) bool {
	for _, allowed := range allowedRepos {
		if strings.Contains(url, allowed) {
			return true
		}
	}
	return false
}

// shouldRewrite determines if a git reference should be rewritten
func shouldRewrite(url, revision, centralURL string) bool {
	if isSHA(revision) {
		return false
	}

	urlMatches := strings.Contains(url, centralURL) || matchesAllowedRepos(url)

	if !urlMatches {
		return false
	}

	for _, branch := range commonBranches {
		if revision == branch {
			return true
		}
	}

	return false
}

// ResolveBranchToSHA resolves a git branch to SHA using GitHub API
func ResolveBranchToSHA(ctx context.Context, url, branch string) (string, error) {
	// Check if we have a mock server URL in context (for testing)
	if mockURL, ok := ctx.Value(GitHubAPIBaseURLKey).(string); ok {
		return ResolveBranchToSHAWithBaseURL(ctx, url, branch, mockURL)
	}
	return ResolveBranchToSHAWithBaseURL(ctx, url, branch, "https://api.github.com")
}

func ResolveBranchToSHAWithBaseURL(ctx context.Context, url, branch, apiBaseURL string) (string, error) {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid GitHub URL format")
	}

	org, repo := parts[len(parts)-2], parts[len(parts)-1]

	apiURL := fmt.Sprintf("%s/repos/%s/%s/commits/%s", apiBaseURL, org, repo, branch)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub API request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var result struct {
		SHA string `json:"sha"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	if !isSHA(result.SHA) {
		return "", fmt.Errorf("GitHub API returned invalid SHA format: %q", result.SHA)
	}

	return result.SHA, nil
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isSHA checks if a string is a valid 40-character SHA
func isSHA(revision string) bool {
	return len(revision) == 40 && isHexString(revision)
}
