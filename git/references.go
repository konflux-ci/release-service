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
	"regexp"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

var shaRegex = regexp.MustCompile("^[a-f0-9]{40}$")

// IsSHA checks if a reference is already a 40-character SHA (optimized)
func IsSHA(ref string) bool {
	return shaRegex.MatchString(ref)
}

// ResolveBranchToSHA resolves a git branch reference to a commit SHA using go-git
func ResolveBranchToSHA(repoURL, revision string) (string, error) {
	if repoURL == "" || revision == "" {
		return "", fmt.Errorf("invalid configuration: repository URL and revision cannot be empty")
	}

	if IsSHA(revision) {
		return revision, nil
	}

	remote := git.NewRemote(nil, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})

	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("remote repository access failed: %w", err)
	}

	refName := plumbing.ReferenceName("refs/heads/" + revision)

	for _, ref := range refs {
		if ref.Name() == refName {
			return ref.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("branch lookup failed: branch '%s' not found in repository", revision)
}
