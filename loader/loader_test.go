package loader

import (
	"fmt"
	"os"
	"strings"
	"time"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
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

		application                 *applicationapiv1alpha1.Application
		component                   *applicationapiv1alpha1.Component
		enterpriseContractConfigMap *corev1.ConfigMap
		enterpriseContractPolicy    *ecapiv1alpha1.EnterpriseContractPolicy
		managedPipelineRun          *tektonv1.PipelineRun
		tenantPipelineRun           *tektonv1.PipelineRun
		release                     *v1alpha1.Release
		releasePlan                 *v1alpha1.ReleasePlan
		releasePlanAdmission        *v1alpha1.ReleasePlanAdmission
		releaseServiceConfig        *v1alpha1.ReleaseServiceConfig
		roleBinding                 *rbac.RoleBinding
		snapshot                    *applicationapiv1alpha1.Snapshot
	)

	AfterAll(func() {
		deleteResources()
	})

	BeforeAll(func() {
		createResources()

		loader = NewLoader()
	})

	When("calling GetActiveReleasePlanAdmission", func() {
		It("returns an active release plan admission", func() {
			returnedObject, err := loader.GetActiveReleasePlanAdmission(ctx, k8sClient, releasePlan)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleasePlanAdmission{}))
			Expect(returnedObject.Name).To(Equal(releasePlanAdmission.Name))
		})

		It("fails to return an active release plan admission if the auto release label is set to false", func() {
			// Use a new application for this test so we don't have timing issues
			disabledReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			disabledReleasePlanAdmission.Labels[metadata.AutoReleaseLabel] = "false"
			disabledReleasePlanAdmission.Name = "disabled-release-plan-admission"
			disabledReleasePlanAdmission.Spec.Applications = []string{"auto-release-test"}
			disabledReleasePlanAdmission.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, disabledReleasePlanAdmission)).To(Succeed())
			releasePlan.Spec.Application = "auto-release-test"

			Eventually(func() bool {
				returnedObject, err := loader.GetActiveReleasePlanAdmission(ctx, k8sClient, releasePlan)
				return returnedObject == nil && err != nil && strings.Contains(err.Error(), "with auto-release label set to false")
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
			Expect(returnedObject).To(Equal(&v1alpha1.ReleasePlanAdmission{}))
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
		var releasePlanTwo, releasePlanDiffApp *v1alpha1.ReleasePlan

		BeforeEach(func() {
			releasePlanTwo = releasePlan.DeepCopy()
			releasePlanTwo.Name = "rp-two"
			releasePlanTwo.ResourceVersion = ""
			releasePlanDiffApp = releasePlan.DeepCopy()
			releasePlanDiffApp.Name = "rp-diff"
			releasePlanDiffApp.Spec.Application = "some-other-app"
			releasePlanDiffApp.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, releasePlanTwo)).To(Succeed())
			Expect(k8sClient.Create(ctx, releasePlanDiffApp)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, releasePlanTwo)).To(Succeed())
			Expect(k8sClient.Delete(ctx, releasePlanDiffApp)).To(Succeed())
		})

		It("returns the requested list of release plans", func() {
			Eventually(func() bool {
				returnedObject, err := loader.GetMatchingReleasePlans(ctx, k8sClient, releasePlanAdmission)
				return returnedObject != &v1alpha1.ReleasePlanList{} && err == nil && len(returnedObject.Items) == 2
			})
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
			})
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
			Expect(k8sClient.Create(ctx, newerRelease)).To(Succeed())

			// Wait until the new release is cached
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: newerRelease.Name, Namespace: newerRelease.Namespace}, newerRelease)
			}).Should(Succeed())

			time.Sleep(1 * time.Second)

			mostRecentRelease = release.DeepCopy()
			mostRecentRelease.Name = "most-recent-release"
			mostRecentRelease.ResourceVersion = ""
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
	})

	When("calling GetRelease", func() {
		It("returns the requested release", func() {
			returnedObject, err := loader.GetRelease(ctx, k8sClient, release.Name, release.Namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.Release{}))
			Expect(returnedObject.Name).To(Equal(release.Name))
		})
	})

	When("calling GetRoleBindingFromReleaseStatus", func() {
		It("fails to return a RoleBinding if the reference is not in the release", func() {
			returnedObject, err := loader.GetRoleBindingFromReleaseStatus(ctx, k8sClient, release)
			Expect(returnedObject).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("release doesn't contain a valid reference to a RoleBinding"))
		})

		It("fails to return a RoleBinding if the roleBinding does not exist", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Status.ManagedProcessing.RoleBinding = "foo/bar"

			returnedObject, err := loader.GetRoleBindingFromReleaseStatus(ctx, k8sClient, modifiedRelease)
			Expect(returnedObject).To(BeNil())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("returns the requested resource", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Status.ManagedProcessing.RoleBinding = fmt.Sprintf("%s%c%s", roleBinding.Namespace,
				types.Separator, roleBinding.Name)

			returnedObject, err := loader.GetRoleBindingFromReleaseStatus(ctx, k8sClient, modifiedRelease)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&rbac.RoleBinding{}))
			Expect(returnedObject.Name).To(Equal(roleBinding.Name))
		})
	})

	When("calling GetReleasePipelineRun", func() {
		It("returns a Managed PipelineRun if the labels match with the release data", func() {
			returnedObject, err := loader.GetReleasePipelineRun(ctx, k8sClient, release, metadata.ManagedPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&tektonv1.PipelineRun{}))
			Expect(returnedObject.Name).To(Equal(managedPipelineRun.Name))
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
					metadata.AutoReleaseLabel: "true",
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

		managedPipelineRun = &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					metadata.ReleaseNameLabel:      release.Name,
					metadata.ReleaseNamespaceLabel: release.Namespace,
					metadata.PipelinesTypeLabel:    metadata.ManagedPipelineType,
				},
				Name:      "pipeline-run",
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, managedPipelineRun)).To(Succeed())

		tenantPipelineRun = &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					metadata.ReleaseNameLabel:      release.Name,
					metadata.ReleaseNamespaceLabel: release.Namespace,
					metadata.PipelinesTypeLabel:    metadata.TenantPipelineType,
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
		Expect(k8sClient.Delete(ctx, managedPipelineRun)).To(Succeed())
		Expect(k8sClient.Delete(ctx, tenantPipelineRun)).To(Succeed())
		Expect(k8sClient.Delete(ctx, release)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlan)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlanAdmission)).To(Succeed())
		Expect(k8sClient.Delete(ctx, roleBinding)).To(Succeed())
		Expect(k8sClient.Delete(ctx, snapshot)).To(Succeed())
	}

})
