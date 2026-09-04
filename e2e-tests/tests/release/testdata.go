// Package common provides test data and helper functions for release e2e tests.
package common

import (
	ginkgo "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"

	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
)

// Test suite labels for release-service e2e tests.
var (
	LabelReleaseService                            = ginkgo.Label("release-service")
	LabelHappyPath                                 = ginkgo.Label("release-service", "happy-path")
	LabelTenant                                    = ginkgo.Label("release-service", "tenant")
	LabelReleasePlanAdm                            = ginkgo.Label("release-service", "release_plan_and_admission")
	LabelNegative                                  = ginkgo.Label("release-service", "release-neg", "negMissingReleasePlan")
	LabelNegBlockReleases                          = ginkgo.Label("release-service", "release-neg", "negBlockReleases")
	LabelNegManagedPipelineRunCreationDenied       = ginkgo.Label("release-service", "release-neg", "negManagedPipelineRunCreationDenied")
	LabelManagedPipelineOOMRetry                   = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-oom-retry")
	LabelManagedPipelineTaskTimeoutRetry           = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-task-timeout-retry")
	LabelManagedPipelinePipelineTimeoutRetry       = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-pipeline-timeout-retry")
	LabelManagedPipelineErrorNoRetry               = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-error-no-retry")
	LabelManagedPipelineMaxRetriesZero             = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-max-retries-zero")
	LabelManagedPipelineTaskRunSpecsOOMRetry       = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-taskrunspecs-oom-retry")
	LabelManagedPipelineRetryFinalPipelineOrdering = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-retry-final-pipeline-ordering")
	LabelManagedPipelineDisabledByTagNoRetry       = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-disabled-by-tag-no-retry")
	LabelManagedPipelineRetryExhausted             = ginkgo.Label("release-service", "retry-managed", "managed-pipeline-retry-exhausted")
	LabelNegTenantPipelineInvalidGitRef            = ginkgo.Label("release-service", "release-neg", "negTenantPipelineInvalidGitRef")
	LabelFinal                                     = ginkgo.Label("release-service", "final")
)

// ManagednamespaceSecret contains the secrets required for the managed namespace.
var ManagednamespaceSecret = []corev1.ObjectReference{
	{Name: constants.RedhatAppstudioUserSecret},
	{Name: constants.ReleaseCatalogTAQuaySecret},
}

// Pipeline configuration variables (loaded from environment at runtime).
var (
	RelSvcCatalogURL       string = utils.GetEnv("RELEASE_SERVICE_CATALOG_URL", "https://github.com/konflux-ci/release-service-catalog")
	RelSvcCatalogRevision  string = utils.GetEnv("RELEASE_SERVICE_CATALOG_REVISION", "development")
	RelSvcOperatorURL      string = utils.GetEnv("RELEASE_SERVICE_OPERATOR_URL", "https://github.com/konflux-ci/release-service")
	RelSvcOperatorRevision string = utils.GetEnv("RELEASE_SERVICE_OPERATOR_REVISION", "main")
)
