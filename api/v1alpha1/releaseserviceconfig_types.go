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
	// 2. Regular expression (contains ^ or $): matched as a regex pattern.
	//    Example: "^https://github\\.com/org/repo(\\.git)?$" matches both
	//    "https://github.com/org/repo" and "https://github.com/org/repo.git"
	//
	// +required
	URL string `json:"url"`

	// Revisions is the list of allowed revisions (branches/tags/commits) for this URL.
	// When non-empty, only pipeline references with a revision matching one of these entries
	// will be allowed. When empty, all revisions are allowed for this URL.
	// +optional
	Revisions []string `json:"revisions,omitempty"`
}

// ReleaseServiceConfigSpec defines the desired state of ReleaseServiceConfig.
type ReleaseServiceConfigSpec struct {
	// AllowedGitResolvers is the list of git repositories and revisions allowed for pipeline references.
	// When non-empty and the resolver is "git", only pipelines with a URL and revision matching one of
	// these entries will be allowed. When empty, all git resolver URLs and revisions are permitted.
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

// IsGitResolverAllowed checks whether the given git resolver URL and revision are allowed.
// Returns true if the URL matches an allowed entry and the revision is permitted, false otherwise.
// When AllowedGitResolvers is empty, all URLs and revisions are permitted.
// URL matching uses exact string match by default, or regex if the pattern starts with ^ and ends with $.
func (rsc *ReleaseServiceConfig) IsGitResolverAllowed(url, revision string) bool {
	// If no restrictions are configured, allow everything
	if len(rsc.Spec.AllowedGitResolvers) == 0 {
		return true
	}

	// Find matching URL
	for _, allowed := range rsc.Spec.AllowedGitResolvers {
		var matched bool

		// Check if this is a regex pattern (starts with ^ and ends with $) or exact string match
		if strings.HasPrefix(allowed.URL, "^") && strings.HasSuffix(allowed.URL, "$") {
			// Treat as regex pattern
			urlRegex, err := regexp.Compile(allowed.URL)
			if err != nil {
				// Invalid regex pattern - skip this entry
				continue
			}
			matched = urlRegex.MatchString(url)
		} else {
			// Exact string match
			matched = (url == allowed.URL)
		}

		if matched {
			// URL matches - check revision restrictions
			if len(allowed.Revisions) == 0 {
				// No revision restrictions for this URL - allow any revision
				return true
			}

			// Check if revision matches one of the allowed revisions
			for _, allowedRevision := range allowed.Revisions {
				if allowedRevision == revision {
					return true
				}
			}

			// URL matched but revision didn't - reject
			return false
		}
	}

	// URL doesn't match any allowed entry - reject
	return false
}

//+kubebuilder:object:root=true

// ReleaseServiceConfigList contains a list of ReleaseServiceConfig
type ReleaseServiceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReleaseServiceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReleaseServiceConfig{}, &ReleaseServiceConfigList{})
}
