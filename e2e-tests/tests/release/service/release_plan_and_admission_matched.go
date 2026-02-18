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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ReleasePlan and ReleasePlanAdmission match", releasecommon.LabelReleasePlanAdm, Ordered, func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var devNamespace = "rel-plan-admis"
	var managedNamespace = "plan-and-admission-managed"

	var releasePlanCR, secondReleasePlanCR *releaseApi.ReleasePlan
	var releasePlanAdmissionCR *releaseApi.ReleasePlanAdmission

	AfterEach(framework.ReportFailure(&fw))

	BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		Expect(err).NotTo(HaveOccurred(), "failed to create framework")
		devNamespace = fw.UserNamespace
		GinkgoWriter.Printf("Created dev namespace: %s\n", devNamespace)

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create managed namespace %s", managedNamespace)

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SourceReleasePlanName, devNamespace, constants.ApplicationNameDefault, managedNamespace, "true", nil, nil, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to create ReleasePlan %s", constants.SourceReleasePlanName)
		GinkgoWriter.Printf("Created ReleasePlan: %s\n", constants.SourceReleasePlanName)
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		}
	})

	var _ = Describe("RP and PRA status change verification", func() {
		It("verifies that the ReleasePlan CR is unmatched in the beginning", func() {
			var condition *metav1.Condition
			Eventually(func() error {
				releasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(constants.SourceReleasePlanName, devNamespace)
				if err != nil {
					return fmt.Errorf("failed to get ReleasePlan: %w", err)
				}
				condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
				if condition == nil {
					return fmt.Errorf("Matched condition not set yet on %s", releasePlanCR.Name)
				}
				return nil
			}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(Succeed(), "timed out waiting for ReleasePlan %s to have Matched condition", constants.SourceReleasePlanName)
			condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse), "expected ReleasePlan to be unmatched initially")
			GinkgoWriter.Printf("ReleasePlan %s is unmatched (as expected)\n", releasePlanCR.Name)
		})

		It("Creates ReleasePlanAdmission CR in corresponding managed namespace", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace, "", devNamespace, constants.ReleaseStrategyPolicyDefault, constants.ReleasePipelineServiceAccountDefault, []string{constants.ApplicationNameDefault}, false, &tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: releasecommon.RelSvcCatalogURL},
					{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
					{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
				},
			}, nil)
			Expect(err).NotTo(HaveOccurred(), "failed to create ReleasePlanAdmission %s in %s", constants.TargetReleasePlanAdmissionName, managedNamespace)
			GinkgoWriter.Printf("Created ReleasePlanAdmission: %s/%s\n", managedNamespace, constants.TargetReleasePlanAdmissionName)
		})

		When("ReleasePlanAdmission CR is created in managed namespace", func() {
			It("verifies that the ReleasePlan CR is set to matched", func() {
				var condition *metav1.Condition
				Eventually(func() error {
					releasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(constants.SourceReleasePlanName, devNamespace)
					if err != nil {
						return fmt.Errorf("failed to get ReleasePlan: %w", err)
					}
					condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("Matched condition not set yet on %s", releasePlanCR.Name)
					}
					if condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("ReleasePlan %s not matched yet", releasePlanCR.Name)
					}
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(Succeed(), "timed out waiting for ReleasePlan %s to be matched", constants.SourceReleasePlanName)
				Expect(condition.Status).To(Equal(metav1.ConditionTrue), "expected ReleasePlan to be matched")
				Expect(releasePlanCR.Status.ReleasePlanAdmission.Name).To(Equal(managedNamespace+"/"+constants.TargetReleasePlanAdmissionName), "expected ReleasePlan to reference the correct RPA")
				Expect(releasePlanCR.Status.ReleasePlanAdmission.Active).To(BeTrue(), "expected ReleasePlanAdmission reference to be active")
				GinkgoWriter.Printf("ReleasePlan %s matched to %s\n", releasePlanCR.Name, releasePlanCR.Status.ReleasePlanAdmission.Name)
			})

			It("verifies that the ReleasePlanAdmission CR is set to matched", func() {
				var condition *metav1.Condition
				Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace)
					if err != nil {
						return fmt.Errorf("failed to get ReleasePlanAdmission: %w", err)
					}
					condition = meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil || condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("ReleasePlanAdmission %s not matched yet", releasePlanAdmissionCR.Name)
					}
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(Succeed(), "timed out waiting for ReleasePlanAdmission %s to be matched", constants.TargetReleasePlanAdmissionName)
				Expect(condition).NotTo(BeNil(), "expected Matched condition to exist")
				Expect(condition.Status).To(Equal(metav1.ConditionTrue), "expected ReleasePlanAdmission to be matched")
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(HaveLen(1), "expected exactly 1 matched ReleasePlan")
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + constants.SourceReleasePlanName, Active: true}}))
				GinkgoWriter.Printf("ReleasePlanAdmission %s matched with 1 ReleasePlan\n", releasePlanAdmissionCR.Name)
			})
		})

		It("Creates a manual release ReleasePlan CR in devNamespace", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(constants.SecondReleasePlanName, devNamespace, constants.ApplicationNameDefault, managedNamespace, "false", nil, nil, nil)
			Expect(err).NotTo(HaveOccurred(), "failed to create second ReleasePlan %s", constants.SecondReleasePlanName)
			GinkgoWriter.Printf("Created second ReleasePlan: %s\n", constants.SecondReleasePlanName)
		})

		When("the second ReleasePlan CR is created", func() {
			It("verifies that the second ReleasePlan CR is set to matched", func() {
				var condition *metav1.Condition
				Eventually(func() error {
					secondReleasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(constants.SecondReleasePlanName, devNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition = meta.FindStatusCondition(secondReleasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())

					if condition == nil {
						return fmt.Errorf("the MatchedConditon of %s is still not set", secondReleasePlanCR.Name)
					}
					if condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("the MatchedConditon of %s has not reconciled yet", secondReleasePlanCR.Name)
					}
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(Succeed())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
				Expect(secondReleasePlanCR.Status.ReleasePlanAdmission.Name).To(Equal(managedNamespace + "/" + constants.TargetReleasePlanAdmissionName))
				Expect(secondReleasePlanCR.Status.ReleasePlanAdmission.Active).To(BeTrue())
			})

			It("verifies that the ReleasePlanAdmission CR has two matched ReleasePlan CRs", func() {
				Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to get the MatchedConditon of RPA %s ", releasePlanAdmissionCR.Name)
					}

					if len(releasePlanAdmissionCR.Status.ReleasePlans) < 2 {
						return fmt.Errorf("the second ReleasePlan CR has not being added to %s", releasePlanAdmissionCR.Name)
					}
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(Succeed(), fmt.Sprintf("time out when waiting for ReleasePlanAdmission %s being reconciled to matched", releasePlanAdmissionCR.Name))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(HaveLen(2))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + constants.SourceReleasePlanName, Active: true}, {Name: devNamespace + "/" + constants.SecondReleasePlanName, Active: false}}))
			})
		})

		It("deletes one ReleasePlan CR", func() {
			err = fw.AsKubeAdmin.ReleaseController.DeleteReleasePlan(constants.SourceReleasePlanName, devNamespace, true)
			Expect(err).NotTo(HaveOccurred(), "failed to delete ReleasePlan %s", constants.SourceReleasePlanName)
			GinkgoWriter.Printf("Deleted ReleasePlan: %s\n", constants.SourceReleasePlanName)
		})

		When("One ReleasePlan CR is deleted in managed namespace", func() {
			It("verifies that the ReleasePlanAdmission CR has only one matching ReleasePlan", func() {
				Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to find the MatchedConditon of %s", releasePlanAdmissionCR.Name)
					}

					if len(releasePlanAdmissionCR.Status.ReleasePlans) > 1 {
						return fmt.Errorf("ReleasePlan CR is deleted, but ReleasePlanAdmission CR %s has not been reconciled", releasePlanAdmissionCR.Name)
					}
					Expect(condition).NotTo(BeNil())
					Expect(condition.Status).To(Equal(metav1.ConditionTrue))
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(Succeed(), fmt.Sprintf("time out when waiting for ReleasePlanAdmission %s being reconciled after one ReleasePlan is deleted", releasePlanAdmissionCR.Name))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(HaveLen(1))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + constants.SecondReleasePlanName, Active: false}}))
			})
		})

		It("deletes the ReleasePlanAdmission CR", func() {
			err = fw.AsKubeAdmin.ReleaseController.DeleteReleasePlanAdmission(constants.TargetReleasePlanAdmissionName, managedNamespace, true)
			Expect(err).NotTo(HaveOccurred(), "failed to delete ReleasePlanAdmission %s", constants.TargetReleasePlanAdmissionName)
			GinkgoWriter.Printf("Deleted ReleasePlanAdmission: %s\n", constants.TargetReleasePlanAdmissionName)
		})

		When("ReleasePlanAdmission CR is deleted in managed namespace", func() {
			It("verifies that the ReleasePlan CR has no matched ReleasePlanAdmission", func() {
				Eventually(func() error {
					secondReleasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(constants.SecondReleasePlanName, devNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(secondReleasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to get the MatchedConditon of %s", secondReleasePlanCR.Name)
					}

					if condition.Status == metav1.ConditionTrue {
						return fmt.Errorf("the MatchedConditon of %s has not reconciled yet", secondReleasePlanCR.Name)
					}
					return nil
				}, constants.ReleasePlanStatusUpdateTimeout, constants.DefaultInterval).Should(Succeed())
				Expect(secondReleasePlanCR.Status.ReleasePlanAdmission).To(Equal(releaseApi.MatchedReleasePlanAdmission{}))
			})
		})
	})
})
