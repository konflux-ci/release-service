package service

import (
	"encoding/json"
	"fmt"
	"time"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"

	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/framework"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/release-service/e2e-tests/tests/release"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Release service tenant pipeline", releasecommon.LabelTenant, Ordered, func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
	var err error
	var devNamespace = "tenant-dev"
	var releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"
	var sampleImage = "quay.io/redhat-appstudio-qe/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var gitSourceURL = "https://github.com/redhat-appstudio-qe/dc-metro-map-release"
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	var tenantServiceAccountName = "tenant-service-account"
	var tenantPullSecretName = "tenant-pull-secret"

	var releaseCR *releaseApi.Release

	BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		Expect(err).NotTo(HaveOccurred(), "failed to create framework: %v", err)
		devNamespace = fw.UserNamespace

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty(), "QUAY_TOKEN env var is required (dockerconfigjson format)")

		_, err := fw.AsKubeAdmin.CommonController.GetSecret(devNamespace, tenantPullSecretName)
		if errors.IsNotFound(err) {
			_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(tenantPullSecretName, devNamespace, sourceAuthJson)
			Expect(err).ToNot(HaveOccurred(), "failed to create pull secret %s: %v", tenantPullSecretName, err)
		} else {
			Expect(err).ToNot(HaveOccurred(), "failed to get pull secret %s: %v", tenantPullSecretName, err)
		}

		_, err = fw.AsKubeAdmin.CommonController.CreateServiceAccount(tenantServiceAccountName, devNamespace, []corev1.ObjectReference{{Name: tenantPullSecretName}}, nil)
		Expect(err).ToNot(HaveOccurred(), "failed to create service account %s: %v", tenantServiceAccountName, err)

		_, err = fw.AsKubeAdmin.KonfluxApiController.CreateApplication(constants.ApplicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create Application %s: %v", constants.ApplicationNameDefault, err)

		data, err := json.Marshal(map[string]interface{}{
			"mapping": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"component":  constants.ComponentName,
						"repository": releasedImagePushRepo,
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred(), "failed to marshal ReleasePlan data: %v", err)

		tenantPipeline := &tektonutils.ParameterizedPipeline{}
		tenantPipeline.ServiceAccountName = tenantServiceAccountName
		tenantPipeline.Timeouts = tektonv1.TimeoutFields{
			Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
		}
		tenantPipeline.PipelineRef = tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: "https://github.com/redhat-appstudio-qe/pipeline_examples"},
				{Name: "revision", Value: "main"},
				{Name: "pathInRepo", Value: "pipelines/simple_pipeline.yaml"},
			},
		}

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SourceReleasePlanName, devNamespace, constants.ApplicationNameDefault, "", "", &runtime.RawExtension{
			Raw: data,
		}, tenantPipeline, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to create ReleasePlan %s: %v", constants.SourceReleasePlanName, err)

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(constants.ReleasePvcName, devNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred(), "failed to create PVC %s: %v", constants.ReleasePvcName, err)

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithImageSource(constants.ComponentName, constants.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
		Expect(err).ShouldNot(HaveOccurred(), "failed to create Snapshot: %v", err)
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(Succeed())
		}
	})

	var _ = Describe("Post-release verification", func() {

		It("verifies that a Release CR should have been created in the dev namespace", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				return err
			}, constants.ReleaseCreationTimeout, constants.DefaultInterval).Should(Succeed())
		})

		It("verifies that Tenant PipelineRun is triggered", func() {
			Expect(fw.AsKubeAdmin.TektonController.WaitForReleasePipelineToBeFinished(releaseCR, devNamespace)).To(Succeed(), "tenant pipelinerun for %s/%s failed", releaseCR.GetNamespace(), releaseCR.GetName())
		})

		It("verifies that a Release is marked as succeeded.", func() {
			Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				if err != nil {
					return fmt.Errorf("failed to get Release: %w", err)
				}
				if !releaseCR.IsReleased() {
					return fmt.Errorf("release %s is not marked as released yet", releaseCR.Name)
				}
				return nil
			}, constants.ReleaseCreationTimeout, constants.DefaultInterval).Should(Succeed())
		})
	})
})
