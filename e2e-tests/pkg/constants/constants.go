// Package constants defines configuration constants for e2e tests.
package constants

import "time"

// =============================================================================
// Environment Variables
// =============================================================================

const (
	// E2E_APPLICATIONS_NAMESPACE_ENV overrides the test namespace name.
	E2E_APPLICATIONS_NAMESPACE_ENV string = "E2E_APPLICATIONS_NAMESPACE"
)

// =============================================================================
// Default Values
// =============================================================================

const (
	// ApplicationNameDefault is the default application name used in tests.
	ApplicationNameDefault string = "appstudio"

	// ReleaseStrategyPolicyDefault is the default release strategy policy.
	ReleaseStrategyPolicyDefault string = "mvp-policy"

	// ReleaseStrategyPolicy is the policy name.
	ReleaseStrategyPolicy string = "policy"
)

// =============================================================================
// Secret Names
// =============================================================================

const (
	RedhatAppstudioUserSecret            string = "hacbs-release-tests-token"
	ReleaseCatalogTAQuaySecret           string = "release-catalog-trusted-artifacts-quay-secret"
	ReleasePipelineServiceAccountDefault string = "release-service-account"
	PublicSecretNameAuth                 string = "cosign-public-key"
)

// =============================================================================
// Resource Names
// =============================================================================

const (
	SourceReleasePlanName          string = "source-releaseplan"
	SecondReleasePlanName          string = "the-second-releaseplan"
	TargetReleasePlanAdmissionName string = "demo"
	ApplicationName                string = "application"
	ReleasePvcName                 string = "release-pvc"
)

// =============================================================================
// Component & Pipeline Constants
// =============================================================================

const (
	ComponentName         string = "dc-metro-map"
	GitSourceComponentUrl string = "https://github.com/redhat-appstudio-qe/dc-metro-map-release"
)

// =============================================================================
// Timeouts & Intervals
// =============================================================================

const (
	// Polling intervals
	PipelineRunPollingInterval = 10 * time.Second
	DefaultInterval            = 100 * time.Millisecond

	// Timeouts
	ReleaseCreationTimeout              = 5 * time.Minute
	ReleasePipelineRunCompletionTimeout = 60 * time.Minute
	ReleasePlanStatusUpdateTimeout      = 1 * time.Minute
)
