package service

import (
	"strings"

	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/framework"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/release-service/e2e-tests/tests/release"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	meta "k8s.io/apimachinery/pkg/api/meta"
)

const tenantPipelineRunCreationFailedMsg = "Release processing failed on tenant pipelineRun creation"

// When a ReleasePlan has an invalid git resolver configuration (empty URL/revision),
// the controller must mark the Release failed and surface a message instead of requeueing forever.
var _ = ginkgo.Describe("Invalid git config in tenant pipeline is surfaced on Release status", releasecommon.LabelNegTenantPipelineInvalidGitRef, ginkgo.Ordered, func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	ginkgo.AfterEach(framework.ReportFailure(&fw))
	var err error
	var devNamespace = "invalid-git-config"
	var sampleImage = "quay.io/hacbs-release-tests/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var gitSourceURL = constants.GitSourceComponentUrl
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"

	var releaseCR *releaseApi.Release

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create framework: %v", err)
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.KonfluxApiController.CreateApplication(constants.ApplicationNameDefault, devNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create Application %s: %v", constants.ApplicationNameDefault, err)

		invalidTenantPipeline := &tektonutils.ParameterizedPipeline{
			Pipeline: tektonutils.Pipeline{
				PipelineRef: tektonutils.PipelineRef{
					Resolver: "git",
					Params: []tektonutils.Param{
						{Name: "url", Value: ""},
						{Name: "revision", Value: ""},
						{Name: "pathInRepo", Value: ""},
					},
				},
			},
		}

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SourceReleasePlanName, devNamespace, constants.ApplicationNameDefault, "", "", nil, invalidTenantPipeline, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlan %s: %v", constants.SourceReleasePlanName, err)

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithImageSource(constants.ComponentName, constants.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to create Snapshot: %v", err)
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(gomega.Succeed())
		}
	})

	ginkgo.It("marks the Release failed with a clear message when tenant pipeline has invalid git config", func() {
		gomega.Eventually(func() bool {
			releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
			if err != nil {
				return false
			}
			if !releaseCR.HasReleaseFinished() || !releaseCR.IsFailed() {
				return false
			}
			cond := meta.FindStatusCondition(releaseCR.Status.Conditions, "Released")
			if cond == nil {
				return false
			}
			return strings.Contains(cond.Message, tenantPipelineRunCreationFailedMsg)
		}, constants.ReleaseCreationTimeout, constants.DefaultInterval).Should(gomega.BeTrue(),
			"expected Release to finish failed with %q on Released condition", tenantPipelineRunCreationFailedMsg)
	})
})
