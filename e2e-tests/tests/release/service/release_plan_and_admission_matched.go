package service

import (
	"fmt"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/api/meta"

	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/framework"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/release-service/e2e-tests/tests/release"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = ginkgo.Describe("ReleasePlan and ReleasePlanAdmission match", releasecommon.LabelReleasePlanAdm, ginkgo.Ordered, func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	var err error
	var devNamespace = "rel-plan-admis"
	var managedNamespace = "plan-and-admission-managed"

	var releasePlanCR, secondReleasePlanCR *releaseApi.ReleasePlan
	var releasePlanAdmissionCR *releaseApi.ReleasePlanAdmission

	ginkgo.AfterEach(framework.ReportFailure(&fw))

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create framework")
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create managed namespace %s", managedNamespace)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SourceReleasePlanName, devNamespace, constants.ApplicationNameDefault, managedNamespace, "true", nil, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlan %s", constants.SourceReleasePlanName)
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(gomega.HaveOccurred())
		}
	})

	var _ = ginkgo.Describe("RP and PRA status change verification", func() {
		ginkgo.It("verifies that the ReleasePlan CR is unmatched in the beginning", func() {
			var condition *metav1.Condition
			gomega.Eventually(func() error {
				releasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(constants.SourceReleasePlanName, devNamespace)
				if err != nil {
					return fmt.Errorf("failed to get ReleasePlan: %w", err)
				}
				condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
				if condition == nil {
					return fmt.Errorf("matched condition not set yet on %s", releasePlanCR.Name)
				}
				return nil
			}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(gomega.Succeed(), "timed out waiting for ReleasePlan %s to have Matched condition", constants.SourceReleasePlanName)
			condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
			gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionFalse), "expected ReleasePlan to be unmatched initially")
		})

		ginkgo.It("Creates ReleasePlanAdmission CR in corresponding managed namespace", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace, "", devNamespace, constants.ReleaseStrategyPolicyDefault, constants.ReleasePipelineServiceAccountDefault, []string{constants.ApplicationNameDefault}, false, &tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: releasecommon.RelSvcCatalogURL},
					{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
					{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
				},
			}, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create ReleasePlanAdmission %s in %s", constants.TargetReleasePlanAdmissionName, managedNamespace)
		})

		ginkgo.When("ReleasePlanAdmission CR is created in managed namespace", func() {
			ginkgo.It("verifies that the ReleasePlan CR is set to matched", func() {
				var condition *metav1.Condition
				gomega.Eventually(func() error {
					releasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(constants.SourceReleasePlanName, devNamespace)
					if err != nil {
						return fmt.Errorf("failed to get ReleasePlan: %w", err)
					}
					condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("matched condition not set yet on %s", releasePlanCR.Name)
					}
					if condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("release plan %s not matched yet", releasePlanCR.Name)
					}
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(gomega.Succeed(), "timed out waiting for ReleasePlan %s to be matched", constants.SourceReleasePlanName)
				gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue), "expected ReleasePlan to be matched")
				gomega.Expect(releasePlanCR.Status.ReleasePlanAdmission.Name).To(gomega.Equal(managedNamespace+"/"+constants.TargetReleasePlanAdmissionName), "expected ReleasePlan to reference the correct RPA")
				gomega.Expect(releasePlanCR.Status.ReleasePlanAdmission.Active).To(gomega.BeTrue(), "expected ReleasePlanAdmission reference to be active")
			})

			ginkgo.It("verifies that the ReleasePlanAdmission CR is set to matched", func() {
				var condition *metav1.Condition
				gomega.Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace)
					if err != nil {
						return fmt.Errorf("failed to get ReleasePlanAdmission: %w", err)
					}
					condition = meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil || condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("release plan admission %s not matched yet", releasePlanAdmissionCR.Name)
					}
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(gomega.Succeed(), "timed out waiting for ReleasePlanAdmission %s to be matched", constants.TargetReleasePlanAdmissionName)
				gomega.Expect(condition).NotTo(gomega.BeNil(), "expected Matched condition to exist")
				gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue), "expected ReleasePlanAdmission to be matched")
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.HaveLen(1), "expected exactly 1 matched ReleasePlan")
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + constants.SourceReleasePlanName, Active: true}}))
			})
		})

		ginkgo.It("Creates a manual release ReleasePlan CR in devNamespace", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SecondReleasePlanName, devNamespace, constants.ApplicationNameDefault, managedNamespace, "false", nil, nil, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create second ReleasePlan %s", constants.SecondReleasePlanName)
		})

		ginkgo.When("the second ReleasePlan CR is created", func() {
			ginkgo.It("verifies that the second ReleasePlan CR is set to matched", func() {
				var condition *metav1.Condition
				gomega.Eventually(func() error {
					secondReleasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(constants.SecondReleasePlanName, devNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition = meta.FindStatusCondition(secondReleasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())

					if condition == nil {
						return fmt.Errorf("the matched condition of %s is still not set", secondReleasePlanCR.Name)
					}
					if condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("the matched condition of %s has not reconciled yet", secondReleasePlanCR.Name)
					}
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(gomega.Succeed())
				gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue))
				gomega.Expect(secondReleasePlanCR.Status.ReleasePlanAdmission.Name).To(gomega.Equal(managedNamespace + "/" + constants.TargetReleasePlanAdmissionName))
				gomega.Expect(secondReleasePlanCR.Status.ReleasePlanAdmission.Active).To(gomega.BeTrue())
			})

			ginkgo.It("verifies that the ReleasePlanAdmission CR has two matched ReleasePlan CRs", func() {
				gomega.Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to get the matched condition of RPA %s", releasePlanAdmissionCR.Name)
					}

					if len(releasePlanAdmissionCR.Status.ReleasePlans) < 2 {
						return fmt.Errorf("the second ReleasePlan CR has not been added to %s", releasePlanAdmissionCR.Name)
					}
					gomega.Expect(condition).NotTo(gomega.BeNil())
					gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue))
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out waiting for ReleasePlanAdmission %s to be reconciled to matched", releasePlanAdmissionCR.Name))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.HaveLen(2))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + constants.SourceReleasePlanName, Active: true}, {Name: devNamespace + "/" + constants.SecondReleasePlanName, Active: false}}))
			})
		})

		ginkgo.It("deletes one ReleasePlan CR", func() {
			err = fw.AsKubeAdmin.ReleaseController.DeleteReleasePlan(constants.SourceReleasePlanName, devNamespace, true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to delete ReleasePlan %s", constants.SourceReleasePlanName)
		})

		ginkgo.When("One ReleasePlan CR is deleted in managed namespace", func() {
			ginkgo.It("verifies that the ReleasePlanAdmission CR has only one matching ReleasePlan", func() {
				gomega.Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to find the matched condition of %s", releasePlanAdmissionCR.Name)
					}

					if len(releasePlanAdmissionCR.Status.ReleasePlans) > 1 {
						return fmt.Errorf("release plan CR is deleted, but ReleasePlanAdmission CR %s has not been reconciled", releasePlanAdmissionCR.Name)
					}
					gomega.Expect(condition).NotTo(gomega.BeNil())
					gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue))
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(gomega.Succeed(), fmt.Sprintf("timed out waiting for ReleasePlanAdmission %s to be reconciled after one ReleasePlan is deleted", releasePlanAdmissionCR.Name))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.HaveLen(1))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + constants.SecondReleasePlanName, Active: false}}))
			})
		})

		ginkgo.It("deletes the ReleasePlanAdmission CR", func() {
			err = fw.AsKubeAdmin.ReleaseController.DeleteReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace, true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to delete ReleasePlanAdmission %s", constants.TargetReleasePlanAdmissionName)
		})

		ginkgo.When("ReleasePlanAdmission CR is deleted in managed namespace", func() {
			ginkgo.It("verifies that the ReleasePlan CR has no matched ReleasePlanAdmission", func() {
				gomega.Eventually(func() error {
					secondReleasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(constants.SecondReleasePlanName, devNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition := meta.FindStatusCondition(secondReleasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to get the matched condition of %s", secondReleasePlanCR.Name)
					}

					if condition.Status == metav1.ConditionTrue {
						return fmt.Errorf("the matched condition of %s has not reconciled yet", secondReleasePlanCR.Name)
					}
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(gomega.Succeed())
				gomega.Expect(secondReleasePlanCR.Status.ReleasePlanAdmission).To(gomega.Equal(releaseApi.MatchedReleasePlanAdmission{}))
			})
		})
	})
})
