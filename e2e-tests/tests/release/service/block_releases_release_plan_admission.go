package service

import (
	"context"
	"strings"

	"github.com/konflux-ci/application-api/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/framework"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/release-service/e2e-tests/tests/release"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = ginkgo.Describe("Release CR fails when block-releases true in ReleasePlanAdmission", releasecommon.LabelNegBlockReleases, ginkgo.Ordered, func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	var err error

	var devNamespace = "block-rp-dev"
	var managedNamespace = "block-rp-managed"

	var releaseCR *releaseApi.Release
	var snapshotName = "snapshot"
	var destinationReleasePlanAdmissionName = "sre-production"
	var releaseName = "release"

	ginkgo.AfterEach(framework.ReportFailure(&fw))

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create framework")
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create managed namespace %s", managedNamespace)

		snapshot := &v1alpha1.Snapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshotName,
				Namespace: devNamespace,
			},
			Spec: v1alpha1.SnapshotSpec{
				Application: constants.ApplicationName,
				Components:  []v1alpha1.SnapshotComponent{},
			},
		}
		err = fw.AsKubeAdmin.CommonController.KubeRest().Create(context.Background(), snapshot)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create Snapshot %s", snapshotName)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SourceReleasePlanName, devNamespace, constants.ApplicationNameDefault, managedNamespace, "", nil, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlan %s", constants.SourceReleasePlanName)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, "", devNamespace, constants.ReleaseStrategyPolicy, constants.ReleasePipelineServiceAccountDefault, []string{constants.ApplicationName}, true, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
			},
		}, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlanAdmission %s with block-releases=true", destinationReleasePlanAdmissionName)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, constants.SourceReleasePlanName)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create Release %s", releaseName)
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(gomega.Succeed())
		}
	})

	var _ = ginkgo.Describe("post-release verification.", func() {
		ginkgo.It("block-releases true in ReleasePlanAdmission makes a Release CR set as failed in both IsReleased and IsValid with a proper message to user.", func() {
			gomega.Eventually(func() bool {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, devNamespace)
				if err != nil {
					return false
				}
				if releaseCR.HasReleaseFinished() {
					return !(releaseCR.IsValid() && releaseCR.IsReleased()) &&
						strings.Contains(releaseCR.Status.Conditions[0].Message, "Release validation failed")
				}
				return false
			}, constants.ReleaseCreationTimeout, constants.DefaultInterval).Should(gomega.BeTrue(), "expected Release to fail validation due to block-releases=true in RPA")
		})
	})
})
