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
	"fmt"
	"regexp"
	"strings"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ReleaseServiceConfigResourceName string = "release-service-config"

// AllowedGitResolver defines an allowed git repository and optional revision restrictions.
type AllowedGitResolver struct {
	// URL specifies the allowed git repository URL.
	// Two modes are supported:
	//
	// 1. Exact string match (no anchors): matches the URL exactly.
	//    Example: "https://github.com/org/repo" only matches that exact URL,
	//    not "https://github.com/org/repo.git"
	//
	// 2. Regular expression (starts with ^ and ends with $): matched as a regex pattern.
	//    Example: "^https://github\\.com/org/repo(\\.git)?$" matches both
	//    "https://github.com/org/repo" and "https://github.com/org/repo.git"
	//
	// Note: This differs from EmptyDirOverrides.Url which always treats patterns as regex.
	// The hybrid approach provides safe defaults (exact match) while enabling flexible patterns.
	// +required
	URL string `json:"url"`

	// Revisions is the list of allowed revisions (branches/tags/commits) for this URL.
	// Supports the same two matching modes as URL:
	//
	// 1. Exact string match (no anchors): "main", "production"
	// 2. Regular expression (starts with ^ and ends with $): "^release-.*$", "^v[0-9]+\\.[0-9]+$"
	//
	// When non-empty, only pipeline references with a revision matching one of these entries
	// will be allowed. When empty, all revisions are allowed for this URL.
	// +optional
	Revisions []string `json:"revisions,omitempty"`
}

// ReleaseServiceConfigSpec defines the desired state of ReleaseServiceConfig.
type ReleaseServiceConfigSpec struct {
	// AllowedGitResolvers is the list of git repositories and revisions allowed for managed pipeline references.
	// This restriction applies ONLY to managed pipelines (from ReleasePlanAdmission) that run in the managed namespace.
	// Tenant and final pipelines run in user-controlled namespaces and are not restricted by this field.
	//
	// When non-empty and the resolver is "git", only managed pipelines with a URL and revision matching one of
	// these entries will be allowed. When empty or omitted, all git resolver URLs and revisions are permitted
	// (backwards compatible default).
	//
	// Invalid regex patterns cause ALL validation to fail (not just the invalid entry). This prevents partial
	// enforcement where some entries work while others silently fail. Use a validating webhook to catch
	// invalid patterns at admission time.
	//
	// First-match semantics: entries are evaluated in order; when a URL matches, only that entry's
	// revision list is consulted. If the URL matches but the revision does not, validation fails
	// without checking remaining entries.
	// +optional
	AllowedGitResolvers []AllowedGitResolver `json:"allowedGitResolvers,omitempty"`

	// Debug is the boolean that specifies whether or not the Release Service should run
	// in debug mode
	// +optional
	Debug bool `json:"debug,omitempty"`

	// DefaultTimeouts contain the default Tekton timeouts to be used in case they are
	// not specified in the ReleasePlanAdmission resource.
	DefaultTimeouts tektonv1.TimeoutFields `json:"defaultTimeouts,omitempty"`

	// VolumeOverrides is a map containing the volume type for specific Pipeline git refs
	// +optional
	EmptyDirOverrides []EmptyDirOverrides `json:"EmptyDirOverrides,omitempty"`

	// RetryablePipelines is a list of pipelines that are safe to automatically retry on failure.
	// +optional
	RetryablePipelines []RetryablePipeline `json:"retryablePipelines,omitempty"`
}

// EmptyDirOverrides defines the values usually set in a PipelineRef using a git resolver.
type EmptyDirOverrides struct {
	// Url is the url to the git repo
	// +required
	Url string `json:"url"`

	// Revision is the git revision where the Pipeline definition can be found
	// +required
	Revision string `json:"revision"`

	// PathInRepo is the path within the git repository where the Pipeline definition can be found
	// +required
	PathInRepo string `json:"pathInRepo"`
}

// ReleaseServiceConfigStatus defines the observed state of ReleaseServiceConfig.
type ReleaseServiceConfigStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=rsc
//+kubebuilder:subresource:status

// ReleaseServiceConfig is the Schema for the releaseserviceconfigs API
type ReleaseServiceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseServiceConfigSpec   `json:"spec,omitempty"`
	Status ReleaseServiceConfigStatus `json:"status,omitempty"`
}

// IsPipelineOverridden checks whether there is a EmptyDirOverride matching the given url, revision and pathInRepo.
func (rsc *ReleaseServiceConfig) IsPipelineOverridden(url, revision, pathInRepo string) bool {
	for _, override := range rsc.Spec.EmptyDirOverrides {
		urlRegex, err := regexp.Compile(override.Url)
		if err != nil || !urlRegex.MatchString(url) {
			continue
		}

		revisionRegex, err := regexp.Compile(override.Revision)
		if err != nil || !revisionRegex.MatchString(revision) {
			continue
		}

		if override.PathInRepo == pathInRepo {
			return true
		}
	}

	return false
}

// ValidateGitResolverPatterns validates all regex patterns in AllowedGitResolvers.
// Returns a slice of validation error messages, one for each invalid pattern.
// Empty slice means all patterns are valid.
func (rsc *ReleaseServiceConfig) ValidateGitResolverPatterns() []string {
	var errors []string

	for i, allowed := range rsc.Spec.AllowedGitResolvers {
		if strings.HasPrefix(allowed.URL, "^") && strings.HasSuffix(allowed.URL, "$") {
			if _, err := regexp.Compile(allowed.URL); err != nil {
				errors = append(errors, fmt.Sprintf("AllowedGitResolvers[%d].URL=%q: %v", i, allowed.URL, err))
			}
		}

		for j, rev := range allowed.Revisions {
			if strings.HasPrefix(rev, "^") && strings.HasSuffix(rev, "$") {
				if _, err := regexp.Compile(rev); err != nil {
					errors = append(errors, fmt.Sprintf("AllowedGitResolvers[%d].Revisions[%d]=%q: %v", i, j, rev, err))
				}
			}
		}
	}

	return errors
}

// IsGitResolverAllowed checks whether the given git resolver URL and revision are allowed.
// Returns true if the URL matches an allowed entry and the revision is permitted, false otherwise.
// When AllowedGitResolvers is empty, all URLs and revisions are permitted (backwards compatible default).
//
// URL and revision matching use exact string match by default, or regex if the pattern starts with ^ and ends with $.
// This differs from IsPipelineOverridden which always uses regex. The hybrid approach was chosen to:
// 1. Provide safe defaults (exact match) for simple configurations
// 2. Enable flexible patterns (regex) for complex scenarios like matching URLs with/without .git suffix,
//    or matching multiple release branches with a single pattern
// 3. Prevent accidental substring matches that could over-permit unauthorized repositories
//
// Security considerations:
//
//   - Empty list permits all (backwards compatible). Ensure ReleaseServiceConfig resource has restricted
//     edit permissions in SERVICE_NAMESPACE to prevent unauthorized removal of restrictions.
//   - Invalid regex patterns cause ALL requests to fail (not just the failing entry). This prevents
//     partial enforcement where an admin thinks they've configured restrictions but some entries
//     silently don't work. Consider adding a validating webhook to catch invalid regex at admission time.
//   - First-match semantics: when a URL matches an entry, only that entry's revision list is consulted.
//     If the revision is not in that list, the function returns false without checking remaining entries.
func (rsc *ReleaseServiceConfig) IsGitResolverAllowed(url, revision string) bool {
	// Empty list permits all (backwards compatible)
	if len(rsc.Spec.AllowedGitResolvers) == 0 {
		return true
	}

	// Pre-validate all regex patterns (URLs and revisions) - fail closed if ANY pattern is invalid
	// This ensures consistent fail-closed behavior and prevents partial enforcement
	// where some entries work while others silently fail due to invalid regex
	for _, allowed := range rsc.Spec.AllowedGitResolvers {
		if strings.HasPrefix(allowed.URL, "^") && strings.HasSuffix(allowed.URL, "$") {
			if _, err := regexp.Compile(allowed.URL); err != nil {
				return false
			}
		}
		// Also validate revision regex patterns
		for _, rev := range allowed.Revisions {
			if strings.HasPrefix(rev, "^") && strings.HasSuffix(rev, "$") {
				if _, err := regexp.Compile(rev); err != nil {
					return false
				}
			}
		}
	}

	for _, allowed := range rsc.Spec.AllowedGitResolvers {
		var matched bool

		// Regex pattern (anchored with ^ and $) or exact string match
		if strings.HasPrefix(allowed.URL, "^") && strings.HasSuffix(allowed.URL, "$") {
			// Already validated above, so this won't error
			urlRegex, _ := regexp.Compile(allowed.URL)
			matched = urlRegex.MatchString(url)
		} else {
			matched = (url == allowed.URL)
		}

		if matched {
			// No revision restrictions - allow any revision
			if len(allowed.Revisions) == 0 {
				return true
			}

			for _, allowedRevision := range allowed.Revisions {
				var revisionMatched bool

				// Regex pattern (anchored with ^ and $) or exact string match
				if strings.HasPrefix(allowedRevision, "^") && strings.HasSuffix(allowedRevision, "$") {
					// Already validated above, so this won't error
					revRegex, _ := regexp.Compile(allowedRevision)
					revisionMatched = revRegex.MatchString(revision)
				} else {
					revisionMatched = (allowedRevision == revision)
				}

				if revisionMatched {
					return true
				}
			}

			// URL matched but revision didn't
			return false
		}
	}

	return false
}

//+kubebuilder:object:root=true

// ReleaseServiceConfigList contains a list of ReleaseServiceConfig
type ReleaseServiceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReleaseServiceConfig `json:"items"`
}
