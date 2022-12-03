package loader

import (
	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

var (
	application                *applicationapiv1alpha1.Application
	component                  *applicationapiv1alpha1.Component
	enterpriseContractPolicy   *ecapiv1alpha1.EnterpriseContractPolicy
	environment                *applicationapiv1alpha1.Environment
	pipelineRun                *v1beta1.PipelineRun
	release                    *v1alpha1.Release
	releasePlan                *v1alpha1.ReleasePlan
	releasePlanAdmission       *v1alpha1.ReleasePlanAdmission
	releaseStrategy            *v1alpha1.ReleaseStrategy
	snapshot                   *applicationapiv1alpha1.Snapshot
	snapshotEnvironmentBinding *applicationapiv1alpha1.SnapshotEnvironmentBinding
)

var _ = Describe("Release Adapter", Ordered, func() {
	BeforeAll(func() {
		createResources()
	})

	Context("When calling convertUnstructuredObject", func() {
		It("returns an actual implementation of the crd passed as an argument", func() {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
				},
			}
			unstructuredObject, err := getUnstructuredObject(pod)
			Expect(err).NotTo(HaveOccurred())

			podFromUnstructuredObject := &v1.Pod{}
			Expect(convertUnstructuredObject(unstructuredObject, podFromUnstructuredObject)).To(Succeed())

			Expect(pod.ObjectMeta.Name).To(Equal(podFromUnstructuredObject.Name))
			Expect(pod.Spec.RestartPolicy).To(Equal(podFromUnstructuredObject.Spec.RestartPolicy))
		})
	})

	Context("When calling getObject", func() {
		It("returns the requested resource if it exists", func() {
			returnedApplication := &applicationapiv1alpha1.Application{}
			returnedApplication, err := getObject(application.Name, application.Namespace, k8sClient, ctx, returnedApplication)
			Expect(err).NotTo(HaveOccurred())
			Expect(application.Spec).To(Equal(returnedApplication.Spec))
		})

		It("returns and error if the requested resource doesn't exist", func() {
			returnedObject := &applicationapiv1alpha1.Application{}
			returnedObject, err := getObject("non-existent-app", "non-existent-app", k8sClient, ctx, returnedObject)
			Expect(err).To(HaveOccurred())
			Expect(returnedObject).To(BeNil())
		})
	})

	Context("When calling getUnstructuredObject", func() {
		It("returns an unstructured instance with the values of the original object", func() {
			unstructuredObject, err := getUnstructuredObject(application)
			Expect(err).NotTo(HaveOccurred())
			spec, exists := unstructuredObject.Object["spec"]
			Expect(exists).To(BeTrue())
			specMap, ok := spec.(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(specMap["displayName"]).To(Equal(application.Spec.DisplayName))
		})
	})

	Context("When calling GetActiveReleasePlanAdmission", func() {
		It("returns an active release plan admission", func() {
			returnedObject, err := GetActiveReleasePlanAdmission(releasePlan, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleasePlanAdmission{}))
			Expect(returnedObject.Name).To(Equal(releasePlanAdmission.Name))
		})

		It("fails to return an active release plan admission if the target does not match", func() {
			modifiedReleasePlan := releasePlan.DeepCopy()
			modifiedReleasePlan.Spec.Target = "non-existent-target"

			returnedObject, err := GetActiveReleasePlanAdmission(modifiedReleasePlan, k8sClient, ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no ReleasePlanAdmission found in the target"))
			Expect(returnedObject).To(BeNil())
		})

		It("fails to return an active release plan admission if the auto release label is set to false", func() {
			disabledReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			disabledReleasePlanAdmission.Labels[v1alpha1.AutoReleaseLabel] = "false"
			disabledReleasePlanAdmission.Name = "disabled-release-plan-admission"
			disabledReleasePlanAdmission.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, disabledReleasePlanAdmission)).To(Succeed())

			Eventually(func() bool {
				returnedObject, err := GetActiveReleasePlanAdmission(releasePlan, k8sClient, ctx)
				return returnedObject == nil && err != nil && strings.Contains(err.Error(), "with auto-release label set to false")
			})

			Expect(k8sClient.Delete(ctx, disabledReleasePlanAdmission)).To(Succeed())
		})
	})

	Context("When calling GetActiveReleasePlanAdmissionFromRelease", func() {
		It("returns an active release plan admission", func() {
			returnedObject, err := GetActiveReleasePlanAdmissionFromRelease(release, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleasePlanAdmission{}))
			Expect(returnedObject.Name).To(Equal(releasePlanAdmission.Name))
		})

		It("fails to return an active release plan admission if the release plan does not match", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Spec.ReleasePlan = "non-existent-release-plan"

			returnedObject, err := GetActiveReleasePlanAdmissionFromRelease(modifiedRelease, k8sClient, ctx)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
			Expect(returnedObject).To(BeNil())
		})
	})

	Context("When calling GetApplication", func() {
		It("returns the requested application", func() {
			returnedObject, err := GetApplication(releasePlanAdmission, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&applicationapiv1alpha1.Application{}))
			Expect(returnedObject.Name).To(Equal(application.Name))
		})
	})

	Context("When calling GetApplicationComponents", func() {
		It("returns the requested list of components", func() {
			returnedObjects, err := GetApplicationComponents(*application, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(returnedObjects)).To(Equal(1))
			Expect(returnedObjects[0].Name).To(Equal(component.Name))
		})
	})

	Context("When calling GetEnterpriseContractPolicy", func() {
		It("returns the requested enterprise contract policy", func() {
			returnedObject, err := GetEnterpriseContractPolicy(releaseStrategy, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&ecapiv1alpha1.EnterpriseContractPolicy{}))
			Expect(returnedObject.Name).To(Equal(enterpriseContractPolicy.Name))
		})
	})

	Context("When calling GetEnvironment", func() {
		It("returns the requested environment", func() {
			returnedObject, err := GetEnvironment(releasePlanAdmission, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&applicationapiv1alpha1.Environment{}))
			Expect(returnedObject.Name).To(Equal(environment.Name))
		})
	})

	Context("When calling GetReleasePipelineRun", func() {
		It("returns a PipelineRun if the labels match with the release data", func() {
			returnedObject, err := GetReleasePipelineRun(release, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1beta1.PipelineRun{}))
			Expect(returnedObject.Name).To(Equal(pipelineRun.Name))
		})

		It("fails to return a PipelineRun if the labels don't match with the release data", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Name = "non-existing-release"

			returnedObject, err := GetReleasePipelineRun(modifiedRelease, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).To(BeNil())
		})
	})

	Context("When calling GetReleasePlan", func() {
		It("returns the requested release plan", func() {
			returnedObject, err := GetReleasePlan(release, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleasePlan{}))
			Expect(returnedObject.Name).To(Equal(releasePlan.Name))
		})
	})

	Context("When calling GetReleaseStrategy", func() {
		It("returns the requested release strategy", func() {
			returnedObject, err := GetReleaseStrategy(releasePlanAdmission, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&v1alpha1.ReleaseStrategy{}))
			Expect(returnedObject.Name).To(Equal(releaseStrategy.Name))
		})
	})

	Context("When calling GetSnapshot", func() {
		It("returns the requested snapshot", func() {
			returnedObject, err := GetSnapshot(snapshot.Name, snapshot.Namespace, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&applicationapiv1alpha1.Snapshot{}))
			Expect(returnedObject.Name).To(Equal(snapshot.Name))
		})
	})

	Context("When calling GetSnapshotEnvironmentBinding", func() {
		It("returns a snapshot environment binding if the environment field value matches the release plan admission one", func() {
			returnedObject, err := GetSnapshotEnvironmentBinding(releasePlanAdmission, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).NotTo(Equal(&applicationapiv1alpha1.SnapshotEnvironmentBinding{}))
			Expect(returnedObject.Name).To(Equal(snapshotEnvironmentBinding.Name))
		})

		It("fails to return a snapshot environment binding if the environment field value doesn't match the release plan admission one", func() {
			modifiedReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			modifiedReleasePlanAdmission.Spec.Environment = "non-existing-environment"

			returnedObject, err := GetSnapshotEnvironmentBinding(modifiedReleasePlanAdmission, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedObject).To(BeNil())
		})
	})

	Context("When calling GetReleasePipelineRunResources", func() {
		It("returns all the relevant resources", func() {
			resources, err := GetReleasePipelineRunResources(release, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).To(MatchFields(IgnoreExtras, Fields{
				"EnterpriseContractPolicy": Not(BeNil()),
				"ReleasePlanAdmission":     Not(BeNil()),
				"ReleaseStrategy":          Not(BeNil()),
				"Snapshot":                 Not(BeNil()),
			}))
		})

		It("fails if any resource fails to be fetched", func() {
			modifiedRelease := release.DeepCopy()
			modifiedRelease.Spec.ReleasePlan = "non-existent-release-plan"

			_, err := GetReleasePipelineRunResources(modifiedRelease, k8sClient, ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When calling GetSnapshotEnvironmentBindingResources", func() {
		It("returns all the relevant resources", func() {
			resources, err := GetSnapshotEnvironmentBindingResources(release, releasePlanAdmission, k8sClient, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).To(MatchFields(IgnoreExtras, Fields{
				"Application":           Not(BeNil()),
				"ApplicationComponents": Not(BeNil()),
				"Environment":           Not(BeNil()),
				"Snapshot":              Not(BeNil()),
			}))
		})

		It("fails if any resource fails to be fetched", func() {
			modifiedReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			modifiedReleasePlanAdmission.Spec.Application = "non-existent-application"

			_, err := GetSnapshotEnvironmentBindingResources(release, modifiedReleasePlanAdmission, k8sClient, ctx)
			Expect(err).To(HaveOccurred())
		})
	})
})

// createResources creates all the required resources for the tests in this file.
func createResources() {
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

	enterpriseContractPolicy = &ecapiv1alpha1.EnterpriseContractPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "enterprise-contract-policy",
			Namespace: "default",
		},
		Spec: ecapiv1alpha1.EnterpriseContractPolicySpec{
			Sources: []string{"foo"},
		},
	}
	Expect(k8sClient.Create(ctx, enterpriseContractPolicy)).Should(Succeed())

	environment = &applicationapiv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "environment",
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.EnvironmentSpec{
			DeploymentStrategy: applicationapiv1alpha1.DeploymentStrategy_Manual,
			DisplayName:        "production",
			Type:               applicationapiv1alpha1.EnvironmentType_POC,
			Configuration: applicationapiv1alpha1.EnvironmentConfiguration{
				Env: []applicationapiv1alpha1.EnvVarPair{},
			},
		},
	}
	Expect(k8sClient.Create(ctx, environment)).Should(Succeed())

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

	releaseStrategy = &v1alpha1.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release-strategy",
			Namespace: "default",
		},
		Spec: v1alpha1.ReleaseStrategySpec{
			Pipeline: "release-pipeline",
			Policy:   enterpriseContractPolicy.Name,
		},
	}
	Expect(k8sClient.Create(ctx, releaseStrategy)).Should(Succeed())

	releasePlanAdmission = &v1alpha1.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release-plan-admission",
			Namespace: "default",
			Labels: map[string]string{
				v1alpha1.AutoReleaseLabel: "true",
			},
		},
		Spec: v1alpha1.ReleasePlanAdmissionSpec{
			Application:     application.Name,
			Origin:          "default",
			Environment:     environment.Name,
			ReleaseStrategy: releaseStrategy.Name,
		},
	}
	Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())

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

	snapshotEnvironmentBinding = &applicationapiv1alpha1.SnapshotEnvironmentBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snapshot-environment-binding",
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.SnapshotEnvironmentBindingSpec{
			Application: application.Name,
			Environment: environment.Name,
			Snapshot:    snapshot.Name,
			Components:  []applicationapiv1alpha1.BindingComponent{},
		},
	}
	Expect(k8sClient.Create(ctx, snapshotEnvironmentBinding)).To(Succeed())

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

	pipelineRun = &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				tekton.ReleaseNameLabel:      release.Name,
				tekton.ReleaseNamespaceLabel: release.Namespace,
			},
			Name:      "pipeline-run",
			Namespace: "default",
		},
	}
	Expect(k8sClient.Create(ctx, pipelineRun)).To(Succeed())
}
