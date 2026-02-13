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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Release CR fails when missing ReleasePlan and ReleasePlanAdmission", releasecommon.LabelNegative, Ordered, func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var devNamespace = "neg-rp-dev"
	var managedNamespace = "neg-rp-managed"

	var releaseCR *releaseApi.Release
	var snapshotName = "snapshot"
	var destinationReleasePlanAdmissionName = "sre-production"
	var releaseName = "release"

	AfterEach(framework.ReportFailure(&fw))

	BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		Expect(err).NotTo(HaveOccurred(), "failed to create framework")
		devNamespace = fw.UserNamespace
		GinkgoWriter.Printf("Created dev namespace: %s\n", devNamespace)

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create managed namespace %s", managedNamespace)

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
		Expect(err).NotTo(HaveOccurred(), "failed to create Snapshot %s", snapshotName)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, "", devNamespace, constants.ReleaseStrategyPolicy, constants.ReleasePipelineServiceAccountDefault, []string{constants.ApplicationName}, false, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
			},
		}, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to create ReleasePlanAdmission %s", destinationReleasePlanAdmissionName)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, constants.SourceReleasePlanName)
		Expect(err).NotTo(HaveOccurred(), "failed to create Release %s", releaseName)
		GinkgoWriter.Printf("Created Release %s (without matching ReleasePlan)\n", releaseName)
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(Succeed())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(Succeed())
		}
	})

	var _ = Describe("post-release verification.", func() {
		It("missing ReleasePlan makes a Release CR set as failed in both IsReleased and IsValid with a proper message to user.", func() {
			Eventually(func() bool {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, devNamespace)
				if err != nil {
					GinkgoWriter.Printf("Failed to get Release: %v\n", err)
					return false
				}
				if releaseCR.HasReleaseFinished() {
					return !(releaseCR.IsValid() && releaseCR.IsReleased()) &&
						strings.Contains(releaseCR.Status.Conditions[0].Message, "Release validation failed")
				}
				return false
			}, constants.ReleaseCreationTimeout, constants.DefaultInterval).Should(BeTrue(), "expected Release to fail validation due to missing ReleasePlan")
			GinkgoWriter.Printf("Release %s correctly failed: %s\n", releaseName, releaseCR.Status.Conditions[0].Message)
		})
		It("missing ReleasePlanAdmission makes a Release CR set as failed in both IsReleased and IsValid with a proper message to user.", func() {
			Expect(fw.AsKubeAdmin.ReleaseController.DeleteReleasePlanAdmission(destinationReleasePlanAdmissionName, managedNamespace, false)).NotTo(HaveOccurred(), "failed to delete ReleasePlanAdmission %s", destinationReleasePlanAdmissionName)
			GinkgoWriter.Printf("Deleted ReleasePlanAdmission %s\n", destinationReleasePlanAdmissionName)

			Eventually(func() bool {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, devNamespace)
				if err != nil {
					GinkgoWriter.Printf("Failed to get Release: %v\n", err)
					return false
				}
				if releaseCR.HasReleaseFinished() {
					return !(releaseCR.IsValid() && releaseCR.IsReleased()) &&
						strings.Contains(releaseCR.Status.Conditions[0].Message, "Release validation failed")
				}
				return false
			}, constants.ReleaseCreationTimeout, constants.DefaultInterval).Should(BeTrue(), "expected Release to fail validation due to missing ReleasePlanAdmission")
			GinkgoWriter.Printf("Release %s correctly failed: %s\n", releaseName, releaseCR.Status.Conditions[0].Message)
		})
	})
})
