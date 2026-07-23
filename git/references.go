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
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// GitHubTokenEnvVar is the environment variable used to authenticate GitHub API
	// requests when resolving branch names to commit SHAs.
	GitHubTokenEnvVar = "GITHUB_TOKEN" // #nosec G101 -- env var name, not a credential
)

// Sentinel errors for git configuration problems that won't resolve on retry.
var (
	// ErrInvalidGitResolverConfig indicates the git resolver configuration is invalid (e.g., empty URL or revision).
	ErrInvalidGitResolverConfig = errors.New("invalid git configuration")
	// ErrBranchNotFound indicates the specified branch does not exist in the repository.
	ErrBranchNotFound = errors.New("branch not found")
)

var (
	shaRegex       = regexp.MustCompile("^[a-f0-9]{40}$")
	gitLog         = ctrl.Log.WithName("git")
	listRemoteRefs = func(remote *git.Remote, opts *git.ListOptions) ([]*plumbing.Reference, error) {
		return remote.List(opts)
	}
	retryDelay = time.Sleep
)

// IsSHA checks if a reference is already a 40-character SHA (optimized)
func IsSHA(ref string) bool {
	return shaRegex.MatchString(ref)
}

// isRateLimitError checks if an error is a rate limit error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "API rate limit") ||
		strings.Contains(errStr, "403")
}

// ValidateGitResolverConfig validates the git resolver configuration parameters.
// Returns ErrInvalidGitResolverConfig if any required parameter is empty or whitespace-only.
func ValidateGitResolverConfig(url, revision, pathInRepo string) error {
	var missing []string
	if strings.TrimSpace(url) == "" {
		missing = append(missing, "url")
	}
	if strings.TrimSpace(revision) == "" {
		missing = append(missing, "revision")
	}
	if strings.TrimSpace(pathInRepo) == "" {
		missing = append(missing, "pathInRepo")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s cannot be empty", ErrInvalidGitResolverConfig, strings.Join(missing, ", "))
	}
	return nil
}

// HasGitHubToken reports whether GITHUB_TOKEN is set in the environment.
func HasGitHubToken() bool {
	return strings.TrimSpace(os.Getenv(GitHubTokenEnvVar)) != ""
}

// IsGitHubURL reports whether repoURL points at a GitHub repository over HTTPS.
func IsGitHubURL(repoURL string) bool {
	return strings.HasPrefix(repoURL, "https://github.com/")
}

func remoteListOptions(repoURL string) *git.ListOptions {
	opts := &git.ListOptions{}
	if IsGitHubURL(repoURL) {
		if token := strings.TrimSpace(os.Getenv(GitHubTokenEnvVar)); token != "" {
			opts.Auth = &githttp.BasicAuth{
				Username: "x-access-token",
				Password: token,
			}
		}
	}
	return opts
}

// ResolveBranchToSHA resolves a git branch reference to a commit SHA using go-git
func ResolveBranchToSHA(repoURL, revision string) (string, error) {
	if repoURL == "" || revision == "" {
		return "", fmt.Errorf("%w: repository URL and revision cannot be empty", ErrInvalidGitResolverConfig)
	}

	if IsSHA(revision) {
		return revision, nil
	}

	gitURL := repoURL
	if strings.HasPrefix(repoURL, "https://github.com/") && !strings.HasSuffix(repoURL, ".git") {
		gitURL = repoURL + ".git"
	}

	remote := git.NewRemote(nil, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})

	maxRetries := 3
	baseDelay := 2 * time.Second
	listOpts := remoteListOptions(repoURL)

	var refs []*plumbing.Reference
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		refs, err = listRemoteRefs(remote, listOpts)
		if err == nil {
			break
		}

		if !isRateLimitError(err) {
			return "", fmt.Errorf("remote repository access failed: %w", err)
		}

		if attempt == maxRetries {
			gitLog.Error(err, "failed to resolve branch to SHA after rate limit retries",
				"url", repoURL, "revision", revision, "authenticated", listOpts.Auth != nil)
			return "", fmt.Errorf("remote repository access failed after %d retries (rate limited): %w", maxRetries, err)
		}

		delay := baseDelay * time.Duration(1<<uint(attempt))
		retryDelay(delay)
	}

	refName := plumbing.ReferenceName("refs/heads/" + revision)

	for _, ref := range refs {
		if ref.Name() == refName {
			return ref.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("%w: branch '%s' not found in repository", ErrBranchNotFound, revision)
}
