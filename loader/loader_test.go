package loader

import (
	stderrors "errors"
	"fmt"
	"os"
	"strings"
	"time"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	ecapiv1alpha1 "github.com/conforma/crds/api/v1alpha1"
	applicationapiv1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Release Adapter", Ordered, func() {
	var (
		loader          ObjectLoader
		createResources func()
		deleteResources func()

		application                  *applicationapiv1alpha1.Application
		component                    *applicationapiv1alpha1.Component
		enterpriseContractConfigMap  *corev1.ConfigMap
		enterpriseContractPolicy     *ecapiv1alpha1.EnterpriseContractPolicy
		finalPipelineRun             *tektonv1.PipelineRun
		managedCollectorsPipelineRun *tektonv1.PipelineRun
		managedPipelineRun           *tektonv1.PipelineRun
		tenantCollectorsPipelineRun  *tektonv1.PipelineRun
		tenantPipelineRun            *tektonv1.PipelineRun
		release                      *v1alpha1.Release
		releasePlan                  *v1alpha1.ReleasePlan
		releasePlanAdmission         *v1alpha1.ReleasePlanAdmission
		releaseServiceConfig         *v1alpha1.ReleaseServiceConfig
		roleBinding                  *rbac.RoleBinding
		snapshot                     *applicationapiv1alpha1.Snapshot
	)

	AfterAll(func() {
		deleteResources()
	})

	BeforeAll(func() {
		createResources()

		loader = NewLoader()
	})

	When("calling GetActiveReleasePlanAdmission test", func() {
		It("returns an active release plan admission", func() {
			returnedObject, err := loader.GetActiveReleasePlanAdmission(ctx, k8sClient, releasePlan)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleasePlanAdmission{}))
			Expect(returnedObject.Name).To(Equal(releasePlanAdmission.Name))
		})

		It("fails to return an active release plan admission if the block releases label is set to true", func() {
			// Use a new application for this test so we don't have timing issues
			disabledReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			disabledReleasePlanAdmission.Labels[metadata.BlockReleasesLabel] = "true"
			disabledReleasePlanAdmission.Name = "disabled-release-plan-admission"
			disabledReleasePlanAdmission.Spec.Applications = []string{"block-releases-test"}
			disabledReleasePlanAdmission.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, disabledReleasePlanAdmission)).To(Succeed())
			releasePlan.Spec.Application = "block-releases-test"

			Eventually(func() bool {
				returnedObject, err := loader.GetActiveReleasePlanAdmission(ctx, k8sClient, releasePlan)
				return returnedObject == nil && err != nil && strings.Contains(err.Error(), "with block-releases label set to false")
			})

			releasePlan.Spec.Application = application.Name
			Expect(k8sClient.Delete(ctx, disabledReleasePlanAdmission)).To(Succeed())
		})
	})

	When("calling GetActiveReleasePlanAdmissionFromRelease", func() {
		It("returns an active release plan admission", func() {
			returnedObject, err := loader.GetActiveReleasePlanAdmissionFromRelease(ctx, k8sClient, release)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleasePlanAdmission{}))
			Expect(returnedObject.Name).To(Equal(releasePlanAdmission.Name))
		})

		It("fails to return an active release plan admission if the release plan does not match", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Spec.ReleasePlan = "non-existent-release-plan"

			returnedObject, err := loader.GetActiveReleasePlanAdmissionFromRelease(ctx, k8sClient, modifiedRelease)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(returnedObject).To(BeNil())
		})
	})

	When("calling GetApplication", func() {
		It("returns the requested application", func() {
			returnedObject, err := loader.GetApplication(ctx, k8sClient, releasePlan)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&applicationapiv1alpha1.Application{}))
			Expect(returnedObject.Name).To(Equal(application.Name))
		})
	})

	When("calling GetEnterpriseContractConfigMap", func() {
		It("returns nil when the ENTERPRISE_CONTRACT_CONFIG_MAP variable is not set", func() {
			os.Unsetenv("ENTERPRISE_CONTRACT_CONFIG_MAP")
			returnedObject, err := loader.GetEnterpriseContractConfigMap(ctx, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).To(BeNil())
		})

		It("returns the requested enterprise contract configmap", func() {
			os.Setenv("ENTERPRISE_CONTRACT_CONFIG_MAP", "default/ec-defaults")
			returnedObject, err := loader.GetEnterpriseContractConfigMap(ctx, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&corev1.ConfigMap{}))
			Expect(returnedObject.Name).To(Equal(enterpriseContractConfigMap.Name))
		})
	})

	When("calling GetEnterpriseContractPolicy", func() {
		It("returns the requested enterprise contract policy", func() {
			returnedObject, err := loader.GetEnterpriseContractPolicy(ctx, k8sClient, releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&ecapiv1alpha1.EnterpriseContractPolicy{}))
			Expect(returnedObject.Name).To(Equal(enterpriseContractPolicy.Name))
		})
	})

	When("calling GetMatchingReleasePlanAdmission", func() {
		It("returns a release plan admission", func() {
			returnedObject, err := loader.GetMatchingReleasePlanAdmission(ctx, k8sClient, releasePlan)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleasePlanAdmission{}))
			Expect(returnedObject.Name).To(Equal(releasePlanAdmission.Name))
		})

		It("returns ReleasePlanAdmission from ReleasePlan label even when multiple matching RPAs exist", func() {
			modifiedReleasePlan := releasePlan.DeepCopy()
			modifiedReleasePlan.Labels = map[string]string{
				metadata.ReleasePlanAdmissionLabel: "new-release-plan-admission",
			}

			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Name = "new-release-plan-admission"
			newReleasePlanAdmission.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, newReleasePlanAdmission)).To(Succeed())

			Eventually(func() bool {
				returnedObject, err := loader.GetMatchingReleasePlanAdmission(ctx, k8sClient, modifiedReleasePlan)
				return err == nil && returnedObject.Name == newReleasePlanAdmission.Name
			})

			Expect(k8sClient.Delete(ctx, newReleasePlanAdmission)).To(Succeed())
		})

		It("fails to return a ReleasePlanAdmission from ReleasePlan label when targeted RPA doesn't exist", func() {
			modifiedReleasePlan := releasePlan.DeepCopy()
			modifiedReleasePlan.Labels = map[string]string{
				metadata.ReleasePlanAdmissionLabel: "foo",
			}

			returnedObject, err := loader.GetMatchingReleasePlanAdmission(ctx, k8sClient, modifiedReleasePlan)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(returnedObject).To(BeNil())
		})

		It("fails to return a ReleasePlanAdmission from ReleasePlan label when targeted RPA has incorrect origin", func() {
			modifiedReleasePlan := releasePlan.DeepCopy()
			modifiedReleasePlan.Labels = map[string]string{
				metadata.ReleasePlanAdmissionLabel: "new-release-plan-admission",
			}

			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Name = "new-release-plan-admission"
			newReleasePlanAdmission.ResourceVersion = ""
			newReleasePlanAdmission.Spec.Origin = "non-existent-origin"
			Expect(k8sClient.Create(ctx, newReleasePlanAdmission)).To(Succeed())
			// Wait until the new releasePlanAdmission is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: newReleasePlanAdmission.Name, Namespace: newReleasePlanAdmission.Namespace}, newReleasePlanAdmission)
			}).Should(Succeed())

			returnedObject, err := loader.GetMatchingReleasePlanAdmission(ctx, k8sClient, modifiedReleasePlan)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not match the namespace"))
			Expect(returnedObject).To(BeNil())

			Expect(k8sClient.Delete(ctx, newReleasePlanAdmission)).To(Succeed())
		})

		It("fails to return a release plan admission if the target does not match", func() {
			modifiedReleasePlan := releasePlan.DeepCopy()
			modifiedReleasePlan.Spec.Target = "non-existent-target"

			returnedObject, err := loader.GetMatchingReleasePlanAdmission(ctx, k8sClient, modifiedReleasePlan)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no ReleasePlanAdmission found in namespace"))
			Expect(returnedObject).To(BeNil())
		})

		It("fails to return a release plan admission if the target is nil", func() {
			modifiedReleasePlan := releasePlan.DeepCopy()
			modifiedReleasePlan.Spec.Target = ""

			returnedObject, err := loader.GetMatchingReleasePlanAdmission(ctx, k8sClient, modifiedReleasePlan)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has no target"))
			Expect(returnedObject).To(BeNil())
		})

		It("fails to return a release plan admission if multiple matches are found", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Name = "new-release-plan-admission"
			newReleasePlanAdmission.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, newReleasePlanAdmission)).To(Succeed())

			Eventually(func() bool {
				returnedObject, err := loader.GetMatchingReleasePlanAdmission(ctx, k8sClient, releasePlan)
				return returnedObject == nil && err != nil && strings.Contains(err.Error(), "multiple ReleasePlanAdmissions")
			})

			Expect(k8sClient.Delete(ctx, newReleasePlanAdmission)).To(Succeed())
		})
	})

	When("calling GetMatchingReleasePlans", func() {
		var releasePlanTwo, releasePlanDiffApp, releasePlanWithLabel *v1alpha1.ReleasePlan

		BeforeEach(func() {
			releasePlanTwo = releasePlan.DeepCopy()
			releasePlanTwo.Name = "rp-two"
			releasePlanTwo.ResourceVersion = ""
			releasePlanDiffApp = releasePlan.DeepCopy()
			releasePlanDiffApp.Name = "rp-diff"
			releasePlanDiffApp.Spec.Application = "some-other-app"
			releasePlanDiffApp.ResourceVersion = ""
			releasePlanWithLabel = releasePlan.DeepCopy()
			releasePlanWithLabel.Name = "rp-with-label"
			releasePlanWithLabel.Labels = map[string]string{
				metadata.ReleasePlanAdmissionLabel: releasePlanAdmission.Name,
			}
			releasePlanWithLabel.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, releasePlanTwo)).To(Succeed())
			Expect(k8sClient.Create(ctx, releasePlanDiffApp)).To(Succeed())
			Expect(k8sClient.Create(ctx, releasePlanWithLabel)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, releasePlanTwo)).To(Succeed())
			Expect(k8sClient.Delete(ctx, releasePlanDiffApp)).To(Succeed())
			Expect(k8sClient.Delete(ctx, releasePlanWithLabel)).To(Succeed())
		})

		It("returns only ReleasePlans with matching label when label exists", func() {
			Eventually(func() bool {
				returnedObject, err := loader.GetMatchingReleasePlans(ctx, k8sClient, releasePlanAdmission)
				return returnedObject != &v1alpha1.ReleasePlanList{} && err == nil && len(returnedObject.Items) == 1
			}).Should(BeTrue())
		})

		It("returns ReleasePlan with matching label even when other ReleasePlans exist", func() {
			Eventually(func() bool {
				returnedObject, err := loader.GetMatchingReleasePlans(ctx, k8sClient, releasePlanAdmission)
				return returnedObject.Items[0].Name == releasePlanWithLabel.Name && err == nil && len(returnedObject.Items) == 1
			}).Should(BeTrue())
		})

		It("falls back to all ReleasePlans when no label matches", func() {
			unmatchedReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			unmatchedReleasePlanAdmission.Name = "other-rpa"
			unmatchedReleasePlanAdmission.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, unmatchedReleasePlanAdmission)).To(Succeed())

			Eventually(func() bool {
				returnedObject, err := loader.GetMatchingReleasePlans(ctx, k8sClient, unmatchedReleasePlanAdmission)
				return returnedObject != &v1alpha1.ReleasePlanList{} && err == nil && len(returnedObject.Items) == 2
			}).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, unmatchedReleasePlanAdmission)).To(Succeed())
		})

		It("does not return a ReleasePlan with a different application", func() {
			Eventually(func() bool {
				returnedObject, err := loader.GetMatchingReleasePlans(ctx, k8sClient, releasePlanAdmission)
				contains := false
				for _, releasePlan := range returnedObject.Items {
					if releasePlan.Spec.Application == "some-other-app" {
						contains = true
					}
				}
				return returnedObject != &v1alpha1.ReleasePlanList{} && err == nil && contains == false
			}).Should(BeTrue())
		})

		It("fails to return release plans if origin is empty", func() {
			modifiedReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			modifiedReleasePlanAdmission.Spec.Origin = ""

			returnedObject, err := loader.GetMatchingReleasePlans(ctx, k8sClient, modifiedReleasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("releasePlanAdmission has no origin, so no ReleasePlans can be found"))
			Expect(returnedObject).To(BeNil())
		})
	})

	When("calling GetPreviousRelease", func() {
		var newerRelease, mostRecentRelease *v1alpha1.Release

		AfterEach(func() {
			k8sClient.Delete(ctx, newerRelease)
			k8sClient.Delete(ctx, mostRecentRelease)

			// Wait until the releases are gone
			Eventually(func() bool {
				releases := &v1alpha1.ReleaseList{}
				err := k8sClient.List(ctx, releases,
					client.InNamespace(release.Namespace),
					client.MatchingFields{"spec.releasePlan": release.Spec.ReleasePlan})
				return err == nil && len(releases.Items) == 1
			}).Should(BeTrue())
		})

		It("returns a NotFound error if no previous release is found", func() {
			returnedObject, err := loader.GetPreviousRelease(ctx, k8sClient, release)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(returnedObject).To(BeNil())
		})

		It("returns the previous release if found", func() {
			// We need a new release with a more recent creation timestamp
			time.Sleep(1 * time.Second)

			newerRelease = release.DeepCopy()
			newerRelease.Name = "newer-release"
			newerRelease.ResourceVersion = ""
			newerRelease.Spec.Snapshot = "new-snapshot"
			Expect(k8sClient.Create(ctx, newerRelease)).To(Succeed())

			// Wait until the new release is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: newerRelease.Name, Namespace: newerRelease.Namespace}, newerRelease)
			}).Should(Succeed())

			returnedObject, err := loader.GetPreviousRelease(ctx, k8sClient, newerRelease)
			Expect(err).ToNot(HaveOccurred())
			Expect(returnedObject).ToNot(BeNil())
			Expect(returnedObject.Name).To(Equal(release.Name))
		})

		It("returns the previous release if multiple releases are found", func() {
			// We need two new releases with a more recent creation timestamp
			time.Sleep(1 * time.Second)

			newerRelease = release.DeepCopy()
			newerRelease.Name = "newer-release"
			newerRelease.ResourceVersion = ""
			newerRelease.Spec.Snapshot = "new-snapshot"
			Expect(k8sClient.Create(ctx, newerRelease)).To(Succeed())

			// Wait until the new release is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: newerRelease.Name, Namespace: newerRelease.Namespace}, newerRelease)
			}).Should(Succeed())

			time.Sleep(1 * time.Second)

			mostRecentRelease = release.DeepCopy()
			mostRecentRelease.Name = "most-recent-release"
			mostRecentRelease.ResourceVersion = ""
			mostRecentRelease.Spec.Snapshot = "most-recent-snapshot"
			Expect(k8sClient.Create(ctx, mostRecentRelease)).To(Succeed())

			// Wait until the new release is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: mostRecentRelease.Name, Namespace: mostRecentRelease.Namespace}, mostRecentRelease)
			}).Should(Succeed())

			returnedObject, err := loader.GetPreviousRelease(ctx, k8sClient, mostRecentRelease)
			Expect(err).ToNot(HaveOccurred())
			Expect(returnedObject).ToNot(BeNil())
			Expect(returnedObject.Name).To(Equal(newerRelease.Name))
		})

		It("returns the previous release with a different snapshot than the current release", func() {
			// We need two new releases with the same snapshot
			time.Sleep(1 * time.Second)

			newerRelease = release.DeepCopy()
			newerRelease.Name = "newer-release"
			newerRelease.ResourceVersion = ""
			newerRelease.Spec.Snapshot = "new-snapshot"
			Expect(k8sClient.Create(ctx, newerRelease)).To(Succeed())

			// Wait until the new release is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: newerRelease.Name, Namespace: newerRelease.Namespace}, newerRelease)
			}).Should(Succeed())

			time.Sleep(1 * time.Second)

			mostRecentRelease = release.DeepCopy()
			mostRecentRelease.Name = "newer-release-retry"
			mostRecentRelease.ResourceVersion = ""
			mostRecentRelease.Spec.Snapshot = "new-snapshot"
			Expect(k8sClient.Create(ctx, mostRecentRelease)).To(Succeed())

			// Wait until the new release is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: mostRecentRelease.Name, Namespace: mostRecentRelease.Namespace}, mostRecentRelease)
			}).Should(Succeed())

			time.Sleep(1 * time.Second)

			returnedObject, err := loader.GetPreviousRelease(ctx, k8sClient, mostRecentRelease)
			Expect(err).ToNot(HaveOccurred())
			Expect(returnedObject).ToNot(BeNil())
			Expect(returnedObject.Name).To(Equal(release.Name))
			Expect(returnedObject.Spec.Snapshot).To(Not(Equal(mostRecentRelease.Spec.Snapshot)))
		})

		It("returns the previous release that was successful before the current release", func() {
			// We need two new releases one that failed and one that was successful
			time.Sleep(1 * time.Second)

			newerRelease = release.DeepCopy()
			newerRelease.Name = "newer-release"
			newerRelease.ResourceVersion = ""
			newerRelease.Spec.Snapshot = "new-snapshot"
			Expect(k8sClient.Create(ctx, newerRelease)).To(Succeed())

			// Wait until the new release is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: newerRelease.Name, Namespace: newerRelease.Namespace}, newerRelease)
			}).Should(Succeed())

			// Mark as release failed
			newerRelease.MarkReleasing("")
			newerRelease.MarkReleaseFailed("")
			Expect(k8sClient.Status().Update(ctx, newerRelease)).To(Succeed())

			mostRecentRelease = release.DeepCopy()
			mostRecentRelease.Name = "most-recent-release"
			mostRecentRelease.ResourceVersion = ""
			mostRecentRelease.Spec.Snapshot = "most-recent-snapshot"
			Expect(k8sClient.Create(ctx, mostRecentRelease)).To(Succeed())

			// Wait until the new release is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: mostRecentRelease.Name, Namespace: mostRecentRelease.Namespace}, mostRecentRelease)
			}).Should(Succeed())

			time.Sleep(1 * time.Second)

			returnedObject, err := loader.GetPreviousRelease(ctx, k8sClient, mostRecentRelease)
			Expect(err).ToNot(HaveOccurred())
			Expect(returnedObject).ToNot(BeNil())
			Expect(returnedObject.Name).To(Equal(release.Name))
		})
	})

	When("calling GetRelease", func() {
		It("returns the requested release", func() {
			returnedObject, err := loader.GetRelease(ctx, k8sClient, release.Name, release.Namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.Release{}))
			Expect(returnedObject.Name).To(Equal(release.Name))
		})
	})

	When("calling GetRoleBindingFromReleaseStatusPipelineInfo", func() {
		It("fails to return a tenant RoleBinding if the reference is not in the release", func() {
			returnedObject, err := loader.GetRoleBindingFromReleaseStatusPipelineInfo(ctx, k8sClient, &release.Status.ManagedProcessing, "tenant")
			Expect(returnedObject).To(BeNil())
			Expect(stderrors.Is(err, ErrInvalidRoleBindingRef)).To(BeTrue())
		})

		It("fails to return a tenant RoleBinding if the roleBinding does not exist", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Status.ManagedProcessing.RoleBindings.TenantRoleBinding = "foo/bar"

			returnedObject, err := loader.GetRoleBindingFromReleaseStatusPipelineInfo(ctx, k8sClient, &modifiedRelease.Status.ManagedProcessing, "tenant")
			Expect(returnedObject).To(BeNil())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("fails to return a RoleBinding for an invalid type", func() {
			modifiedRelease := release.DeepCopy()
			returnedObject, err := loader.GetRoleBindingFromReleaseStatusPipelineInfo(ctx, k8sClient, &modifiedRelease.Status.ManagedProcessing, "foo")
			Expect(returnedObject).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid role binding type"))
		})

		It("returns the requested resource", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Status.ManagedProcessing.RoleBindings.TenantRoleBinding = fmt.Sprintf("%s%c%s", roleBinding.Namespace,
				types.Separator, roleBinding.Name)

			returnedObject, err := loader.GetRoleBindingFromReleaseStatusPipelineInfo(ctx, k8sClient, &modifiedRelease.Status.ManagedProcessing, "tenant")
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&rbac.RoleBinding{}))
			Expect(returnedObject.Name).To(Equal(roleBinding.Name))
		})
	})

	When("calling GetReleasePipelineRun", func() {
		It("returns an error when called with an unexpected Pipeline type", func() {
			invalidPipelineType := metadata.PipelineType("invalid-type")
			returnedObject, err := loader.GetReleasePipelineRun(ctx, k8sClient, release, invalidPipelineType)
			Expect(returnedObject).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid type"))
		})

		It("returns a Final PipelineRun if the labels match with the release data", func() {
			returnedObject, err := loader.GetReleasePipelineRun(ctx, k8sClient, release, metadata.FinalPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&tektonv1.PipelineRun{}))
			Expect(returnedObject.Name).To(Equal(finalPipelineRun.Name))
		})

		It("returns a Managed Collectors PipelineRun if the labels match with the release data", func() {
			returnedObject, err := loader.GetReleasePipelineRun(ctx, k8sClient, release, metadata.ManagedCollectorsPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&tektonv1.PipelineRun{}))
			Expect(returnedObject.Name).To(Equal(managedCollectorsPipelineRun.Name))
		})

		It("returns a Managed PipelineRun if the labels match with the release data", func() {
			returnedObject, err := loader.GetReleasePipelineRun(ctx, k8sClient, release, metadata.ManagedPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&tektonv1.PipelineRun{}))
			Expect(returnedObject.Name).To(Equal(managedPipelineRun.Name))
		})

		It("returns a Tenant Collecotrs PipelineRun if the labels match with the release data", func() {
			returnedObject, err := loader.GetReleasePipelineRun(ctx, k8sClient, release, metadata.TenantCollectorsPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&tektonv1.PipelineRun{}))
			Expect(returnedObject.Name).To(Equal(tenantCollectorsPipelineRun.Name))
		})

		It("returns a Tenant PipelineRun if the labels match with the release data", func() {
			returnedObject, err := loader.GetReleasePipelineRun(ctx, k8sClient, release, metadata.TenantPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&tektonv1.PipelineRun{}))
			Expect(returnedObject.Name).To(Equal(tenantPipelineRun.Name))
		})

		It("fails to return a PipelineRun if the labels don't match with the release data", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Name = "non-existing-release"

			returnedObject, err := loader.GetReleasePipelineRun(ctx, k8sClient, modifiedRelease, metadata.ManagedPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).To(BeNil())
		})
	})

	When("calling GetReleasePlan", func() {
		It("returns the requested release plan", func() {
			returnedObject, err := loader.GetReleasePlan(ctx, k8sClient, release)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleasePlan{}))
			Expect(returnedObject.Name).To(Equal(releasePlan.Name))
		})
	})

	When("calling GetReleaseServiceConfig", func() {
		It("returns the requested ReleaseServiceConfig", func() {
			returnedObject, err := loader.GetReleaseServiceConfig(ctx, k8sClient, releaseServiceConfig.Name, releaseServiceConfig.Namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleaseServiceConfig{}))
			Expect(returnedObject.Name).To(Equal(releaseServiceConfig.Name))
		})
	})

	When("calling GetSnapshot", func() {
		It("returns the requested snapshot", func() {
			returnedObject, err := loader.GetSnapshot(ctx, k8sClient, release)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&applicationapiv1alpha1.Snapshot{}))
			Expect(returnedObject.Name).To(Equal(snapshot.Name))
		})
	})

	// Composite functions

	When("calling GetProcessingResources", func() {
		It("returns all the relevant resources", func() {
			os.Setenv("ENTERPRISE_CONTRACT_CONFIG_MAP", "default/ec-defaults")
			resources, err := loader.GetProcessingResources(ctx, k8sClient, release)
			Expect(err).NotTo(HaveOccurred())
			Expect(*resources).To(MatchFields(IgnoreExtras, Fields{
				"EnterpriseContractConfigMap": Not(BeNil()),
				"EnterpriseContractPolicy":    Not(BeNil()),
				"ReleasePlan":                 Not(BeNil()),
				"ReleasePlanAdmission":        Not(BeNil()),
				"Snapshot":                    Not(BeNil()),
			}))
		})

		It("fails if any resource fails to be fetched", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Spec.Snapshot = "non-existent-snapshot"

			_, err := loader.GetProcessingResources(ctx, k8sClient, modifiedRelease)
			Expect(err).To(HaveOccurred())
		})
	})

	createResources = func() {
		application = &applicationapiv1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "application",
				Namespace: "default",
			},
			Spec: applicationapiv1alpha1.ApplicationSpec{
				DisplayName: "application",
			},
		}
		Expect(k8sClient.Create(ctx, application)).To(Succeed())

		component = &applicationapiv1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "component",
				Namespace: "default",
			},
			Spec: applicationapiv1alpha1.ComponentSpec{
				Application:   application.Name,
				ComponentName: "component",
			},
		}
		Expect(k8sClient.Create(ctx, component)).Should(Succeed())

		enterpriseContractConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ec-defaults",
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, enterpriseContractConfigMap)).Should(Succeed())

		enterpriseContractPolicy = &ecapiv1alpha1.EnterpriseContractPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "enterprise-contract-policy",
				Namespace: "default",
			},
			Spec: ecapiv1alpha1.EnterpriseContractPolicySpec{
				Sources: []ecapiv1alpha1.Source{
					{Name: "foo"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, enterpriseContractPolicy)).Should(Succeed())

		releasePlan = &v1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "release-plan",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleasePlanSpec{
				Application: application.Name,
				Target:      "default",
			},
		}
		Expect(k8sClient.Create(ctx, releasePlan)).To(Succeed())

		releaseServiceConfig = &v1alpha1.ReleaseServiceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1alpha1.ReleaseServiceConfigResourceName,
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, releaseServiceConfig)).To(Succeed())

		releasePlanAdmission = &v1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "release-plan-admission",
				Namespace: "default",
				Labels: map[string]string{
					metadata.BlockReleasesLabel: "false",
				},
			},
			Spec: v1alpha1.ReleasePlanAdmissionSpec{
				Applications: []string{application.Name},
				Origin:       "default",
				Pipeline: &tektonutils.Pipeline{
					PipelineRef: tektonutils.PipelineRef{
						Resolver: "bundles",
						Params: []tektonutils.Param{
							{Name: "bundle", Value: "testbundle"},
							{Name: "name", Value: "release-pipeline"},
							{Name: "kind", Value: "pipeline"},
						},
					},
				},
				Policy: enterpriseContractPolicy.Name,
			},
		}
		Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())

		roleBinding = &rbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rolebinding",
				Namespace: "default",
			},
			RoleRef: rbac.RoleRef{
				APIGroup: rbac.GroupName,
				Kind:     "ClusterRole",
				Name:     "clusterrole",
			},
		}
		Expect(k8sClient.Create(ctx, roleBinding)).To(Succeed())

		snapshot = &applicationapiv1alpha1.Snapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "snapshot",
				Namespace: "default",
			},
			Spec: applicationapiv1alpha1.SnapshotSpec{
				Application: application.Name,
			},
		}
		Expect(k8sClient.Create(ctx, snapshot)).To(Succeed())

		release = &v1alpha1.Release{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "release",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleaseSpec{
				Snapshot:    snapshot.Name,
				ReleasePlan: releasePlan.Name,
			},
		}
		Expect(k8sClient.Create(ctx, release)).To(Succeed())

		finalPipelineRun = &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					metadata.ReleaseNameLabel:      release.Name,
					metadata.ReleaseNamespaceLabel: release.Namespace,
					metadata.PipelinesTypeLabel:    metadata.FinalPipelineType.String(),
				},
				Name:      "final-pipeline-run",
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, finalPipelineRun)).To(Succeed())

		managedCollectorsPipelineRun = &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					metadata.ReleaseNameLabel:      release.Name,
					metadata.ReleaseNamespaceLabel: release.Namespace,
					metadata.PipelinesTypeLabel:    metadata.ManagedCollectorsPipelineType.String(),
				},
				Name:      "managed-collectors-pipeline-run",
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, managedCollectorsPipelineRun)).To(Succeed())

		managedPipelineRun = &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					metadata.ReleaseNameLabel:      release.Name,
					metadata.ReleaseNamespaceLabel: release.Namespace,
					metadata.PipelinesTypeLabel:    metadata.ManagedPipelineType.String(),
				},
				Name:      "managed-pipeline-run",
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, managedPipelineRun)).To(Succeed())

		tenantCollectorsPipelineRun = &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					metadata.ReleaseNameLabel:      release.Name,
					metadata.ReleaseNamespaceLabel: release.Namespace,
					metadata.PipelinesTypeLabel:    metadata.TenantCollectorsPipelineType.String(),
				},
				Name:      "tenant-collectors-pipeline-run",
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, tenantCollectorsPipelineRun)).To(Succeed())

		tenantPipelineRun = &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					metadata.ReleaseNameLabel:      release.Name,
					metadata.ReleaseNamespaceLabel: release.Namespace,
					metadata.PipelinesTypeLabel:    metadata.TenantPipelineType.String(),
				},
				Name:      "tenant-pipeline-run",
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, tenantPipelineRun)).To(Succeed())
	}

	deleteResources = func() {
		Expect(k8sClient.Delete(ctx, application)).To(Succeed())
		Expect(k8sClient.Delete(ctx, component)).To(Succeed())
		Expect(k8sClient.Delete(ctx, enterpriseContractPolicy)).To(Succeed())
		Expect(k8sClient.Delete(ctx, finalPipelineRun)).To(Succeed())
		Expect(k8sClient.Delete(ctx, managedPipelineRun)).To(Succeed())
		Expect(k8sClient.Delete(ctx, tenantPipelineRun)).To(Succeed())
		Expect(k8sClient.Delete(ctx, release)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlan)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlanAdmission)).To(Succeed())
		Expect(k8sClient.Delete(ctx, roleBinding)).To(Succeed())
		Expect(k8sClient.Delete(ctx, snapshot)).To(Succeed())
	}

})
