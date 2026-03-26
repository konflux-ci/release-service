// Package common provides test data and helper functions for release e2e tests.
package common

import (
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
)

// Test suite labels for release-service e2e tests.
var (
	LabelReleaseService   = ginkgo.Label("release-service")
	LabelHappyPath        = ginkgo.Label("release-service", "happy-path")
	LabelTenant           = ginkgo.Label("release-service", "tenant")
	LabelReleasePlanAdm   = ginkgo.Label("release-service", "release_plan_and_admission")
	LabelNegative         = ginkgo.Label("release-service", "release-neg", "negMissingReleasePlan")
	LabelNegBlockReleases = ginkgo.Label("release-service", "release-neg", "negBlockReleases")
)

// ManagednamespaceSecret contains the secrets required for the managed namespace.
var ManagednamespaceSecret = []corev1.ObjectReference{
	{Name: constants.RedhatAppstudioUserSecret},
	{Name: constants.ReleaseCatalogTAQuaySecret},
}

// Pipeline configuration variables (loaded from environment at runtime).
var (
	RelSvcCatalogURL      string = utils.GetEnv("RELEASE_SERVICE_CATALOG_URL", "https://github.com/konflux-ci/release-service-catalog")
	RelSvcCatalogRevision string = utils.GetEnv("RELEASE_SERVICE_CATALOG_REVISION", "development")
)
