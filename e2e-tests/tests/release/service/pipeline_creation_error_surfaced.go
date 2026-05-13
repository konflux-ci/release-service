package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ecp "github.com/conforma/crds/api/v1alpha1"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/framework"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/release-service/e2e-tests/tests/release"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const managedPipelineRunCreationFailedMsg = "Release processing failed on managed pipelineRun creation"

// When the API rejects managed release PipelineRun creation (e.g. ResourceQuota), the controller
// must mark the Release failed and surface a message instead of requeueing silently.
var _ = ginkgo.Describe("Managed PipelineRun creation denial is surfaced on Release status", releasecommon.LabelNegManagedPipelineRunCreationDenied, ginkgo.Ordered, func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	ginkgo.AfterEach(framework.ReportFailure(&fw))
	var err error
	var devNamespace = "managed-pr-quota-deny"
	var managedNamespace = "managed-pr-quota-deny-managed"
	var releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"
	var sampleImage = "quay.io/hacbs-release-tests/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var gitSourceURL = constants.GitSourceComponentUrl
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	var ecPolicyName = "managed-pr-quota-policy-" + utils.GenerateRandomString(4)

	var releaseCR *releaseApi.Release

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create framework: %v", err)
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create managed namespace %s: %v", managedNamespace, err)

		sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
		gomega.Expect(sourceAuthJson).ToNot(gomega.BeEmpty(), "QUAY_TOKEN env var is required (dockerconfigjson format)")

		releaseCatalogTrustedArtifactsQuayAuthJson := utils.GetEnv("RELEASE_CATALOG_TA_QUAY_TOKEN", "")
		gomega.Expect(releaseCatalogTrustedArtifactsQuayAuthJson).ToNot(gomega.BeEmpty(), "RELEASE_CATALOG_TA_QUAY_TOKEN env var is required (dockerconfigjson format)")

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(constants.ReleasePipelineServiceAccountDefault, managedNamespace, releasecommon.ManagednamespaceSecret, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create service account %s in %s: %v", constants.ReleasePipelineServiceAccountDefault, managedNamespace, err)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create rolebinding for service account: %v", err)

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(constants.RedhatAppstudioUserSecret, managedNamespace, sourceAuthJson)
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to create secret %s: %v", constants.RedhatAppstudioUserSecret, err)

		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(constants.ReleaseCatalogTAQuaySecret, managedNamespace, releaseCatalogTrustedArtifactsQuayAuthJson)
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to create secret %s: %v", constants.ReleaseCatalogTAQuaySecret, err)

		err = fw.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(managedNamespace, constants.RedhatAppstudioUserSecret, constants.ReleasePipelineServiceAccountDefault, true)
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to link secret to service account: %v", err)

		releasePublicKeyDecoded := []byte("-----BEGIN PUBLIC KEY-----\n" +
			"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEocSG/SnE0vQ20wRfPltlXrY4Ib9B\n" +
			"CRnFUCg/fndZsXdz0IX5sfzIyspizaTbu4rapV85KirmSBU6XUaLY347xg==\n" +
			"-----END PUBLIC KEY-----")
		err = fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(releasePublicKeyDecoded, constants.PublicSecretNameAuth, managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create signing secret %s: %v", constants.PublicSecretNameAuth, err)

		defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to get default EC policy: %v", err)

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
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create EC policy %s: %v", ecPolicyName, err)

		_, err = fw.AsKubeAdmin.KonfluxApiController.CreateApplication(constants.ApplicationNameDefault, devNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create Application %s: %v", constants.ApplicationNameDefault, err)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SourceReleasePlanName, devNamespace, constants.ApplicationNameDefault, managedNamespace, "", nil, nil, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlan %s: %v", constants.SourceReleasePlanName, err)

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
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to marshal RPA data: %v", err)

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
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlanAdmission %s: %v", constants.TargetReleasePlanAdmissionName, err)

		_, err = fw.AsKubeAdmin.CommonController.CreateRole("role-release-service-account", managedNamespace, map[string][]string{
			"apiGroupsList": {""},
			"roleResources": {"secrets"},
			"roleVerbs":     {"get", "list", "watch"},
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create Role: %v", err)

		_, err = fw.AsKubeAdmin.CommonController.CreateRoleBinding("role-release-service-account-binding", managedNamespace, "ServiceAccount", constants.ReleasePipelineServiceAccountDefault, managedNamespace, "Role", "role-release-service-account", "rbac.authorization.k8s.io")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create RoleBinding: %v", err)

		_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(constants.ReleasePvcName, managedNamespace, corev1.ReadWriteOnce)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create PVC %s: %v", constants.ReleasePvcName, err)

		quota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deny-managed-pipelineruns-e2e",
				Namespace: managedNamespace,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceName("count/pipelineruns.tekton.dev"): resource.MustParse("0"),
				},
			},
		}
		err = fw.AsKubeAdmin.CommonController.KubeRest().Create(context.Background(), quota)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ResourceQuota to deny PipelineRuns in %s: %v", managedNamespace, err)

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithImageSource(constants.ComponentName, constants.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to create Snapshot: %v", err)
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(gomega.Succeed())
		}
	})

	ginkgo.It("marks the Release failed with a clear message when managed PipelineRun creation is denied", func() {
		gomega.Eventually(func() bool {
			releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
			if err != nil {
				return false
			}
			if !releaseCR.HasReleaseFinished() || !releaseCR.IsFailed() {
				return false
			}
			cond := meta.FindStatusCondition(releaseCR.Status.Conditions, "Released")
			return cond != nil && strings.Contains(cond.Message, managedPipelineRunCreationFailedMsg)
		}, constants.ReleaseCreationTimeout, constants.DefaultInterval).Should(gomega.BeTrue(),
			"expected Release to finish failed with %q on Released condition after quota denied managed PipelineRun creation", managedPipelineRunCreationFailedMsg)
	})
})
