package service

import (
	"encoding/json"
	"fmt"

	ecp "github.com/conforma/crds/api/v1alpha1"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/framework"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/release-service/e2e-tests/tests/release"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Release service happy path", releasecommon.LabelHappyPath, Ordered, func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	AfterEach(framework.ReportFailure(&fw))
	var err error
	var compName string
	var devNamespace = "happy-path"
	var managedNamespace = "happy-path-managed"
	var _ *appservice.Snapshot
	var verifyConformaTaskName = "verify-conforma"
	var releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"
	var sampleImage = "quay.io/hacbs-release-tests/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var gitSourceURL = constants.GitSourceComponentUrl
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	var ecPolicyName = "hpath-policy-" + utils.GenerateRandomString(4)

	var releaseCR *releaseApi.Release

	BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		Expect(err).NotTo(HaveOccurred(), "failed to create framework: %v", err)
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create managed namespace %s: %v", managedNamespace, err)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		Expect(sourceAuthJson).ToNot(BeEmpty(), "QUAY_TOKEN env var is required (dockerconfigjson format)")

		releaseCatalogTrustedArtifactsQuayAuthJson := utils.GetEnv("RELEASE_CATALOG_TA_QUAY_TOKEN", "")
		Expect(releaseCatalogTrustedArtifactsQuayAuthJson).ToNot(BeEmpty(), "RELEASE_CATALOG_TA_QUAY_TOKEN env var is required (dockerconfigjson format)")

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(constants.ReleasePipelineServiceAccountDefault, managedNamespace, releasecommon.ManagednamespaceSecret, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to create service account %s in %s: %v", constants.ReleasePipelineServiceAccountDefault, managedNamespace, err)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		Expect(err).NotTo(HaveOccurred(), "failed to create rolebinding for service account: %v", err)

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(constants.RedhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		Expect(err).ToNot(HaveOccurred(), "failed to create secret %s: %v", constants.RedhatAppstudioUserSecret, err)

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(constants.ReleaseCatalogTAQuaySecret, managedNamespace, releaseCatalogTrustedArtifactsQuayAuthJson)
		Expect(err).ToNot(HaveOccurred(), "failed to create secret %s: %v", constants.ReleaseCatalogTAQuaySecret, err)

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, constants.RedhatAppstudioUserSecret, constants.ReleasePipelineServiceAccountDefault, true)
		Expect(err).ToNot(HaveOccurred(), "failed to link secret to service account: %v", err)

		releasePublicKeyDecoded := []byte("-----BEGIN PUBLIC KEY-----\n" +
			"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEocSG/SnE0vQ20wRfPltlXrY4Ib9B\n" +
			"CRnFUCg/fndZsXdz0IX5sfzIyspizaTbu4rapV85KirmSBU6XUaLY347xg==\n" +
			"-----END PUBLIC KEY-----")
		err = fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(releasePublicKeyDecoded, constants.PublicSecretNameAuth, managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create signing secret %s: %v", constants.PublicSecretNameAuth, err)

		defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		Expect(err).NotTo(HaveOccurred(), "failed to get default EC policy: %v", err)

		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   fmt.Sprintf("k8s://%s/%s", managedNamespace, constants.PublicSecretNameAuth),
			Sources:     defaultEcPolicy.Spec.Sources,
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"@slsa3"},
				Exclude:     []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			},
		}
		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(ecPolicyName, managedNamespace, defaultEcPolicySpec)
		Expect(err).NotTo(HaveOccurred(), "failed to create EC policy %s: %v", ecPolicyName, err)

		_, err = fw.AsKubeAdmin.KonfluxApiController.CreateApplication(constants.ApplicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create Application %s: %v", constants.ApplicationNameDefault, err)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SourceReleasePlanName, devNamespace, constants.ApplicationNameDefault, managedNamespace, "", nil, nil, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to create ReleasePlan %s: %v", constants.SourceReleasePlanName, err)

		data, err := json.Marshal(map[string]interface{}{
			"mapping": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"component":  compName,
						"repository": releasedImagePushRepo,
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred(), "failed to marshal RPA data: %v", err)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace, "", devNamespace, ecPolicyName, constants.ReleasePipelineServiceAccountDefault, []string{constants.ApplicationNameDefault}, false, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcCatalogURL},
				{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
				{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
			},
		}, &runtime.RawExtension{
			Raw: data,
		})
		Expect(err).NotTo(HaveOccurred(), "failed to create ReleasePlanAdmission %s: %v", constants.TargetReleasePlanAdmissionName, err)

		_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		Expect(err).NotTo(HaveOccurred(), "failed to create Role: %v", err)

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", constants.ReleasePipelineServiceAccountDefault, managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		Expect(err).NotTo(HaveOccurred(), "failed to create RoleBinding: %v", err)

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(constants.ReleasePvcName, managedNamespace, corev1.ReadWriteOnce)
		Expect(err).NotTo(HaveOccurred(), "failed to create PVC %s: %v", constants.ReleasePvcName, err)

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithImageSource(constants.ComponentName, constants.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
		Expect(err).ShouldNot(HaveOccurred(), "failed to create Snapshot: %v", err)
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(Succeed())
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

		It("verifies that Release PipelineRun is triggered", func() {
			Expect(fw.AsKubeAdmin.TektonController.WaitForReleasePipelineToBeFinished(releaseCR, managedNamespace)).To(Succeed(), "pipelinerun for release %s/%s failed", releaseCR.GetNamespace(), releaseCR.GetName())
		})

		It("verifies that Enterprise Contract Task has succeeded in the Release PipelineRun", func() {
			Eventually(func() error {
				pr, err := fw.AsKubeAdmin.TektonController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
				if err != nil {
					return fmt.Errorf("failed to get PipelineRun: %w", err)
				}
				ecTaskRunStatus, err := fw.AsKubeAdmin.TektonController.GetTaskRunStatus(fw.AsKubeAdmin.CommonController.KubeRest(), pr, verifyConformaTaskName)
				if err != nil {
					return fmt.Errorf("failed to get TaskRun status for %s: %w", verifyConformaTaskName, err)
				}
				if !framework.DidTaskSucceed(ecTaskRunStatus) {
					return fmt.Errorf("task %s has not succeeded yet", verifyConformaTaskName)
				}
				return nil
			}, constants.ReleasePipelineRunCompletionTimeout, constants.DefaultInterval).Should(Succeed())
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
