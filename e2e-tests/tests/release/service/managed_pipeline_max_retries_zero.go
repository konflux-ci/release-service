package service

import (
	"encoding/json"
	"fmt"
	"time"

	ecp "github.com/conforma/crds/api/v1alpha1"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/framework"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/release-service/e2e-tests/tests/release"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
)

var _ = ginkgo.Describe("Managed pipeline OOM is not retried when RPA MaxRetries is zero", releasecommon.LabelManagedPipelineMaxRetriesZero, ginkgo.Ordered, func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	ginkgo.AfterEach(framework.ReportFailure(&fw))
	var err error
	var devNamespace = "retry-maxzero"
	var managedNamespace = "retry-maxzero-managed"
	var _ *appservice.Snapshot
	var sampleImage = "quay.io/hacbs-release-tests/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	var gitSourceRevision = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	var ecPolicyName = "retry-maxzero-policy-" + utils.GenerateRandomString(4)

	var releaseCR *releaseApi.Release

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create framework: %v", err)
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create managed namespace %s: %v", managedNamespace, err)

		managedServiceAccount, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(constants.ReleasePipelineServiceAccountDefault, managedNamespace, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create service account %s in %s: %v", constants.ReleasePipelineServiceAccountDefault, managedNamespace, err)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(managedNamespace, managedServiceAccount)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create RoleBinding: %v", err)

		releasePublicKeyDecoded := []byte("-----BEGIN PUBLIC KEY-----\n" +
			"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEocSG/SnE0vQ20wRfPltlXrY4Ib9B\n" +
			"CRnFUCg/fndZsXdz0IX5sfzIyspizaTbu4rapV85KirmSBU6XUaLY347xg==\n" +
			"-----END PUBLIC KEY-----")
		err = fw.AsKubeAdmin.TektonController.CreateOrUpdateSigningSecret(releasePublicKeyDecoded, constants.PublicSecretNameAuth, managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create signing secret %s: %v", constants.PublicSecretNameAuth, err)

		defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to get default EC policy: %v", err)
		gomega.Expect(defaultEcPolicy.Spec.Sources).ToNot(gomega.BeEmpty(), "default EC policy has no sources")

		source := defaultEcPolicy.Spec.Sources[0]
		source.Config = &ecp.SourceConfig{
			Include: []string{"@slsa3"},
			Exclude: []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
		}

		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   fmt.Sprintf("k8s://%s/%s", managedNamespace, constants.PublicSecretNameAuth),
			Sources:     []ecp.Source{source},
		}
		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(ecPolicyName, managedNamespace, defaultEcPolicySpec)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create EC policy %s: %v", ecPolicyName, err)

		// Delete any existing ReleaseServiceConfig and create a new one with the desired RetryablePipeline configuration.
		_ = fw.AsKubeAdmin.ReleaseController.DeleteReleaseServiceConfig(releaseApi.ReleaseServiceConfigResourceName, "release-service")
		err = fw.AsKubeAdmin.ReleaseController.CreateReleaseServiceConfig(
			releaseApi.ReleaseServiceConfigResourceName, "release-service", []releaseApi.RetryablePipeline{
				{
					Url:        releasecommon.RelSvcOperatorURL,
					Revision:   releasecommon.RelSvcOperatorRevision,
					PathInRepo: "e2e-tests/pipelines/retry-e2e-managed.yaml",
					RetryPolicy: releaseApi.RetryPolicy{
						MaxRetries: 3,
						Mitigations: &releaseApi.Mitigations{
							OOMKill: &releaseApi.MemoryMitigation{
								Multiplier: "2.0",
								MaxComputeResources: &corev1.ResourceRequirements{
									Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
									Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
								},
							},
							Timeout: &releaseApi.TimeoutMitigation{
								Task: &releaseApi.TimeoutIncrement{
									Increment:  metav1.Duration{Duration: 5 * time.Minute},
									MaxTimeout: &metav1.Duration{Duration: 10 * time.Minute},
								},
								Pipeline: &releaseApi.TimeoutIncrement{
									Increment:  metav1.Duration{Duration: 5 * time.Minute},
									MaxTimeout: &metav1.Duration{Duration: 20 * time.Minute},
								},
							},
						},
					},
				},
			})
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleaseServiceConfig: %v", err)

		_, err = fw.AsKubeAdmin.KonfluxApiController.CreateApplication(constants.ApplicationNameDefault, devNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create Application %s: %v", constants.ApplicationNameDefault, err)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SourceReleasePlanName, devNamespace, constants.ApplicationNameDefault, managedNamespace, "", nil, nil, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlan %s: %v", constants.SourceReleasePlanName, err)

		data, err := json.Marshal(map[string]interface{}{
			"failMode":   "oom",
			"failOnTask": "task-01",
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to marshal RPA data: %v", err)

		maxRetries := 0
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace, "", devNamespace, ecPolicyName, constants.ReleasePipelineServiceAccountDefault, []string{constants.ApplicationNameDefault}, false, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: releasecommon.RelSvcOperatorURL},
				{Name: "revision", Value: releasecommon.RelSvcOperatorRevision},
				{Name: "pathInRepo", Value: "e2e-tests/pipelines/retry-e2e-managed.yaml"},
			},
		}, &runtime.RawExtension{
			Raw: data,
		}, &maxRetries, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlanAdmission %s: %v", constants.TargetReleasePlanAdmissionName, err)

		_, err = fw.AsKubeAdmin.IntegrationController.CreateSnapshotWithImageSource(constants.ComponentName, constants.ApplicationNameDefault, devNamespace, sampleImage, constants.GitSourceComponentUrl, gitSourceRevision, "", "", "", "")
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "failed to create Snapshot: %v", err)
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(gomega.Succeed())
		}
	})

	var _ = ginkgo.Describe("Post-release verification", func() {

		ginkgo.It("verifies that a Release CR should have been created in the dev namespace", func() {
			gomega.Eventually(func() error {
				releaseCR, err = fw.AsKubeAdmin.ReleaseController.GetFirstReleaseInNamespace(devNamespace)
				return err
			}, constants.ReleaseCreationTimeout, constants.DefaultInterval).Should(gomega.Succeed())
		})

		ginkgo.It("verifies that a managed PipelineRun is triggered", func() {
			gomega.Expect(fw.AsKubeAdmin.TektonController.WaitForPipelineRunToStart(releaseCR, managedNamespace)).To(gomega.Succeed())
		})

		ginkgo.It("verifies the Release is marked as failed", func() {
			releaseCR, err = fw.AsKubeAdmin.ReleaseController.WaitForRelease(releaseCR)
			gomega.Expect(err).To(gomega.HaveOccurred(), "expected release %s/%s to fail", releaseCR.GetNamespace(), releaseCR.GetName())
		})

		ginkgo.It("verifies the Release failed with an OOM attempt and with no retries", func() {
			gomega.Expect(framework.GetManagedPipelineRetryCount(releaseCR)).To(gomega.Equal(0), "expected no retries as MaxRetries=0 on RPA overrides the RSC")

			attempts := releaseCR.Status.ManagedPipelineAttempts
			gomega.Expect(attempts).NotTo(gomega.BeEmpty(), "expected a single managed pipeline attempt to be present")
			gomega.Expect(attempts[0].FailureReason).To(gomega.Equal("OOMKill"), "expected OOMKill failure reason, got %s", attempts[0].FailureReason)
		})
	})
})
