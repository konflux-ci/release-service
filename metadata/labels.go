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

package metadata

import "fmt"

// Common constants
const (
	// RhtapDomain is the prefix of the application label
	RhtapDomain = "appstudio.openshift.io"

	// MaxLabelLength is the maximum allowed characters in a label value
	MaxLabelLength = 63

	// Release service name
	ServiceName = "release"
)

// Prefixes used by the release controller package
var (
	// PipelinesAsCodePrefix contains the prefix applied to labels and annotations copied from Pipelines as Code resources.
	PipelinesAsCodePrefix = "pac.test.appstudio.openshift.io"
)

// Labels used by the release api package
var (
	// AttributionLabel is the label name for the standing-attribution label
	AttributionLabel = fmt.Sprintf("release.%s/standing-attribution", RhtapDomain)

	// AutoReleaseLabel is the label name for the auto-release setting
	AutoReleaseLabel = fmt.Sprintf("release.%s/auto-release", RhtapDomain)

	// AuthorLabel is the label name for the user who creates a CR
	AuthorLabel = fmt.Sprintf("release.%s/author", RhtapDomain)

	// AutomatedLabel is the label name for marking a Release as automated
	AutomatedLabel = fmt.Sprintf("release.%s/automated", RhtapDomain)

	// ServiceNameLabel is the label used to specify the service associated with an object
	ServiceNameLabel = fmt.Sprintf("%s/%s", RhtapDomain, "service")

	// ReleasePlanAdmissionLabel is the ReleasePlan label for the name of the ReleasePlanAdmission to use
	ReleasePlanAdmissionLabel = fmt.Sprintf("release.%s/releasePlanAdmission", RhtapDomain)
)

// Prefixes to be used by Release Pipelines labels
var (
	// pipelinesLabelPrefix is the prefix of the pipelines label
	pipelinesLabelPrefix = fmt.Sprintf("pipelines.%s", RhtapDomain)

	// releaseLabelPrefix is the prefix of the release labels
	releaseLabelPrefix = fmt.Sprintf("release.%s", RhtapDomain)
)

// Labels to be used within Release PipelineRuns
var (
	// ApplicationNameLabel is the label used to specify the application associated with the PipelineRun
	ApplicationNameLabel = fmt.Sprintf("%s/%s", RhtapDomain, "application")

	// FinalPipelineType is the value to be used in the PipelinesTypeLabel for final Pipelines
	FinalPipelineType = "final"

	// ManagedPipelineType is the value to be used in the PipelinesTypeLabel for managed Pipelines
	ManagedPipelineType = "managed"

	// TenantPipelineType is the value to be used in the PipelinesTypeLabel for tenant Pipelines
	TenantPipelineType = "tenant"

	// PipelinesTypeLabel is the label used to describe the type of pipeline
	PipelinesTypeLabel = fmt.Sprintf("%s/%s", pipelinesLabelPrefix, "type")

	// ReleaseNameLabel is the label used to specify the name of the Release associated with the PipelineRun
	ReleaseNameLabel = fmt.Sprintf("%s/%s", releaseLabelPrefix, "name")

	// ReleaseNamespaceLabel is the label used to specify the namespace of the Release associated with the PipelineRun
	ReleaseNamespaceLabel = fmt.Sprintf("%s/%s", releaseLabelPrefix, "namespace")

	// ReleaseSnapshotLabel is the label used to specify the snapshot associated with the PipelineRun
	ReleaseSnapshotLabel = fmt.Sprintf("%s/%s", RhtapDomain, "snapshot")
)
