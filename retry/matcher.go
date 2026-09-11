/*
Copyright 2026.

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

package retry

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/konflux-ci/release-service/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
)

// DetermineRetryInfo determines the retry information for a given ReleasePlanAdmission.
// It checks if the RPA's pipeline matches any configured retryable pipelines and if
// tags from the RPA or matched ReleasePlans disable retries.
// The RPA's Pipeline.MaxRetries can override the RSC configuration.
func DetermineRetryInfo(rpa *v1alpha1.ReleasePlanAdmission, matchedRPs *v1alpha1.ReleasePlanList, rsc *v1alpha1.ReleaseServiceConfig, logger *logr.Logger) *v1alpha1.RetryInfo {
	// Handle no RSC case
	if rsc == nil {
		return &v1alpha1.RetryInfo{
			Enabled: false,
			Reason:  "no retry configuration available",
		}
	}

	// Handle no pipeline case
	if rpa.Spec.Pipeline == nil {
		return &v1alpha1.RetryInfo{
			Enabled: false,
			Reason:  "no pipeline defined",
		}
	}

	// Check if retries are explicitly disabled by RPA pipeline
	if rpa.Spec.Pipeline.MaxRetries != nil && *rpa.Spec.Pipeline.MaxRetries == 0 {
		return &v1alpha1.RetryInfo{
			Enabled: false,
			Reason:  "retries disabled by RPA pipeline override",
		}
	}

	// Try to match the pipeline against RSC retryable pipelines
	matchedRetryable := GetMatchingRetryablePipeline(rpa.Spec.Pipeline, rsc.Spec.RetryablePipelines, logger)
	if matchedRetryable == nil {
		return &v1alpha1.RetryInfo{
			Enabled: false,
			Reason:  "pipeline not configured for retries",
		}
	}

	// Check disable conditions (tags)
	if matchedRetryable.RetryPolicy.DisableOn != nil &&
		len(matchedRetryable.RetryPolicy.DisableOn.Tags) > 0 {

		// Extract tags from RPA
		rpaTags := ExtractTags(rpa.Spec.Data, logger)

		// Extract tags from all matched RPs
		rpTags := []string{}
		if matchedRPs != nil {
			for _, rp := range matchedRPs.Items {
				rpTags = append(rpTags, ExtractTags(rp.Spec.Data, logger)...)
			}
		}

		// Combine tags
		allTags := append(rpaTags, rpTags...)

		// Check if disabled by tags
		if matchedTag, disabled := IsDisabledByTags(allTags, matchedRetryable.RetryPolicy.DisableOn.Tags, logger); disabled {
			return &v1alpha1.RetryInfo{
				Enabled: false,
				Reason:  fmt.Sprintf("disabled by ReleasePlan or ReleasePlanAdmission tag(s): %s", matchedTag),
			}
		}
	}

	// Collect disable tags from the matched pipeline so the Release adapter can check
	// Release tags at a retry attempt.
	var disableTags []string
	if matchedRetryable.RetryPolicy.DisableOn != nil {
		disableTags = matchedRetryable.RetryPolicy.DisableOn.Tags
	}

	// RPA can override the retry count but mitigations always come from the RSC
	if rpa.Spec.Pipeline.MaxRetries != nil {
		maxRetries := *rpa.Spec.Pipeline.MaxRetries
		return &v1alpha1.RetryInfo{
			Enabled:     true,
			MaxRetries:  &maxRetries,
			Mitigations: matchedRetryable.RetryPolicy.Mitigations,
			DisableTags: disableTags,
			Reason:      "retries enabled by RPA pipeline override",
		}
	}

	maxRetries := matchedRetryable.RetryPolicy.MaxRetries
	return &v1alpha1.RetryInfo{
		Enabled:     true,
		MaxRetries:  &maxRetries,
		Mitigations: matchedRetryable.RetryPolicy.Mitigations,
		DisableTags: disableTags,
		Reason:      "retries enabled by policy",
	}
}

// GetMatchingRetryablePipeline checks if a pipeline matches any of the configured retryable pipelines.
// It returns the first matching RetryablePipeline or nil if no match is found.
// Follows the same pattern as IsPipelineOverridden from releaseserviceconfig_types.go.
func GetMatchingRetryablePipeline(pipeline *tektonutils.Pipeline, retryablePipelines []v1alpha1.RetryablePipeline, logger *logr.Logger) *v1alpha1.RetryablePipeline {
	if pipeline == nil {
		return nil
	}

	// Extract git resolver params
	url, revision, pathInRepo, err := pipeline.PipelineRef.GetGitResolverParams()
	if err != nil {
		// Not a git resolver or error extracting params, can't match
		if logger != nil {
			logger.V(1).Info("Failed to extract git resolver params", "error", err)
		}
		return nil
	}

	// Normalize URL to strip trailing .git so both bare and .git forms match
	url = strings.TrimSuffix(url, ".git")

	// Try to match against each retryable pipeline
	for i, retryable := range retryablePipelines {
		// Match URL with regex (auto-anchor to prevent substring matches)
		anchoredUrl := fmt.Sprintf("^(?:%s)$", retryable.Url)
		urlRegex, err := regexp.Compile(anchoredUrl)
		if err != nil {
			if logger != nil {
				logger.V(1).Info("Failed to compile URL regex", "url", retryable.Url, "error", err)
			}
			continue
		}
		// URLs can be written with or without a trailing .git, so accept both
		if !urlRegex.MatchString(url) && !urlRegex.MatchString(url+".git") {
			continue
		}

		// Match revision with regex (auto-anchor to prevent substring matches)
		anchoredRevision := fmt.Sprintf("^(?:%s)$", retryable.Revision)
		revisionRegex, err := regexp.Compile(anchoredRevision)
		if err != nil {
			if logger != nil {
				logger.V(1).Info("Failed to compile revision regex", "revision", retryable.Revision, "error", err)
			}
			continue
		}
		if !revisionRegex.MatchString(revision) {
			continue
		}

		// Exact match on pathInRepo
		if retryable.PathInRepo == pathInRepo {
			return &retryablePipelines[i]
		}
	}

	return nil
}

// ExtractTags extracts tags from a runtime.RawExtension data field.
// Tags can be in four locations according to the release-service-catalog schema:
// 1. mapping.defaults.tags
// 2. mapping.components[].tags
// 3. mapping.components[].componentTags
// 4. mapping.components[].repositories[].tags
func ExtractTags(data *runtime.RawExtension, logger *logr.Logger) []string {
	if data == nil || data.Raw == nil {
		return []string{}
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data.Raw, &parsed); err != nil {
		if logger != nil {
			logger.V(1).Info("Failed to unmarshal data for tag extraction", "error", err)
		}
		return []string{}
	}

	tags := []string{}

	// Get mapping object
	mapping, ok := parsed["mapping"].(map[string]interface{})
	if !ok {
		return []string{}
	}

	// 1. Extract mapping.defaults.tags
	if defaults, ok := mapping["defaults"].(map[string]interface{}); ok {
		tags = append(tags, extractTagArray(defaults["tags"])...)
	}

	// 2. Extract from components array
	if components, ok := mapping["components"].([]interface{}); ok {
		for _, comp := range components {
			if compMap, ok := comp.(map[string]interface{}); ok {
				// Component-level tags
				tags = append(tags, extractTagArray(compMap["tags"])...)

				// Component-level componentTags
				tags = append(tags, extractTagArray(compMap["componentTags"])...)

				// Repository-level tags
				if repos, ok := compMap["repositories"].([]interface{}); ok {
					for _, repo := range repos {
						if repoMap, ok := repo.(map[string]interface{}); ok {
							tags = append(tags, extractTagArray(repoMap["tags"])...)
						}
					}
				}
			}
		}
	}

	return tags
}

// extractTagArray is a helper that extracts string tags from an interface{} array.
func extractTagArray(tagsInterface interface{}) []string {
	tagArray, ok := tagsInterface.([]interface{})
	if !ok {
		return []string{}
	}

	result := []string{}
	for _, tag := range tagArray {
		if tagStr, ok := tag.(string); ok {
			result = append(result, tagStr)
		}
	}
	return result
}

// IsDisabledByTags checks if any of the combined tags match the disable tags.
func IsDisabledByTags(combinedTags, disableTags []string, logger *logr.Logger) (string, bool) {
	var matchedTags []string
	seenTags := make(map[string]bool)

	addMatch := func(tag string) {
		if !seenTags[tag] {
			seenTags[tag] = true
			matchedTags = append(matchedTags, tag)
		}
	}

	for _, disableTag := range disableTags {
		// Exact match first so a literal tag always matches itself, even one that
		// isn't valid regex syntax.
		for _, tag := range combinedTags {
			if tag == disableTag {
				addMatch(tag)
			}
		}

		// Also try disableTag as a regex with auto-anchoring, to support patterns
		// like unresolved template variables.
		anchoredPattern := fmt.Sprintf("^(?:%s)$", disableTag)
		tagRegex, err := regexp.Compile(anchoredPattern)
		if err != nil {
			if logger != nil {
				logger.V(1).Info("Failed to compile disable tag regex", "tag", disableTag, "error", err)
			}
			continue
		}
		for _, tag := range combinedTags {
			if tagRegex.MatchString(tag) {
				addMatch(tag)
			}
		}
	}

	if len(matchedTags) > 0 {
		matched := strings.Join(matchedTags, ", ")
		return matched, true
	}

	return "", false
}
