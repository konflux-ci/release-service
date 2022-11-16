/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package release

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/redhat-appstudio/release-service/gitops"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/operator-framework/operator-lib/handler"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"

	"github.com/redhat-appstudio/release-service/tekton"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Release Adapter", Ordered, func() {
	var (
		adapter *Adapter

		application              *applicationapiv1alpha1.Application
		snapshot                 *applicationapiv1alpha1.Snapshot
		component                *applicationapiv1alpha1.Component
		environment              *applicationapiv1alpha1.Environment
		release                  *appstudiov1alpha1.Release
		releaseStrategy          *appstudiov1alpha1.ReleaseStrategy
		releasePlan              *appstudiov1alpha1.ReleasePlan
		releasePlanAdmission     *appstudiov1alpha1.ReleasePlanAdmission
		enterpriseContractPolicy *ecapiv1alpha1.EnterpriseContractPolicy
	)

	BeforeAll(func() {
		ecPolicy := ecapiv1alpha1.PolicySource{
			GitRepository: &ecapiv1alpha1.GitPolicySource{
				Repository: "https://github.com/",
				Revision:   "main",
			},
		}

		enterpriseContractPolicy = &ecapiv1alpha1.EnterpriseContractPolicy{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-policy-",
				Namespace:    testNamespace,
			},
			Spec: ecapiv1alpha1.EnterpriseContractPolicySpec{
				Description: "test-policy-description",
				Sources:     []ecapiv1alpha1.PolicySource{ecPolicy},
			},
		}
		Expect(k8sClient.Create(ctx, enterpriseContractPolicy)).Should(Succeed())

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      enterpriseContractPolicy.Name,
			Namespace: enterpriseContractPolicy.Namespace,
		}, enterpriseContractPolicy)
		Expect(err).ShouldNot(HaveOccurred())

		releaseStrategy = &appstudiov1alpha1.ReleaseStrategy{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-releasestrategy-",
				Namespace:    testNamespace,
			},
			Spec: appstudiov1alpha1.ReleaseStrategySpec{
				Pipeline:              "release-pipeline",
				Bundle:                "test-bundle",
				Policy:                enterpriseContractPolicy.Name,
				PersistentVolumeClaim: "test-pvc",
				ServiceAccount:        "test-account",
			},
		}
		Expect(k8sClient.Create(ctx, releaseStrategy)).Should(Succeed())

		snapshot = &applicationapiv1alpha1.Snapshot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-snapshot-",
				Namespace:    "default",
			},
			Spec: applicationapiv1alpha1.SnapshotSpec{
				Application: "test-app",
				Components:  []applicationapiv1alpha1.SnapshotComponent{},
			},
		}
		Expect(k8sClient.Create(ctx, snapshot)).Should(Succeed())

		application = &applicationapiv1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
			Spec: applicationapiv1alpha1.ApplicationSpec{
				DisplayName: "app",
			},
		}
		Expect(k8sClient.Create(ctx, application)).Should(Succeed())

		component = &applicationapiv1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-component",
				Namespace: "default",
			},
			Spec: applicationapiv1alpha1.ComponentSpec{
				Application:   "test-app",
				ComponentName: "test-component",
				Source: applicationapiv1alpha1.ComponentSource{
					ComponentSourceUnion: applicationapiv1alpha1.ComponentSourceUnion{
						GitSource: &applicationapiv1alpha1.GitSource{
							URL: "https://foo",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, component)).Should(Succeed())

		environment = &applicationapiv1alpha1.Environment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-environment",
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

		releasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-releaseplanadmission-",
				Namespace:    testNamespace,
				Labels: map[string]string{
					appstudiov1alpha1.AutoReleaseLabel: "true",
				},
			},
			Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
				Application:     "test-app",
				Origin:          testNamespace,
				Environment:     "test-environment",
				ReleaseStrategy: releaseStrategy.GetName(),
			},
		}
		Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())

		releasePlan = &appstudiov1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-releaseplan-",
				Namespace:    testNamespace,
				Labels: map[string]string{
					appstudiov1alpha1.AutoReleaseLabel: "true",
				},
			},
			Spec: appstudiov1alpha1.ReleasePlanSpec{
				Application: "test-app",
				Target:      testNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, releasePlan)).Should(Succeed())

	})

	BeforeEach(func() {
		release = &appstudiov1alpha1.Release{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-release-",
				Namespace:    testNamespace,
			},
			Spec: appstudiov1alpha1.ReleaseSpec{
				Snapshot:    snapshot.GetName(),
				ReleasePlan: releasePlan.GetName(),
			},
		}
		Expect(k8sClient.Create(ctx, release)).Should(Succeed())

		Eventually(func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      release.Name,
				Namespace: testNamespace,
			}, release)
			return err
		}, time.Second*10).ShouldNot(HaveOccurred())

		adapter = NewAdapter(release, ctrl.Log, k8sClient, ctx)
		Expect(reflect.TypeOf(adapter)).To(Equal(reflect.TypeOf(&Adapter{})))
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, release)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	AfterAll(func() {
		err := k8sClient.Delete(ctx, snapshot)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())

		Expect(k8sClient.Delete(ctx, releaseStrategy)).Should(Succeed())

		err = k8sClient.Delete(ctx, releasePlan)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())

		err = k8sClient.Delete(ctx, releasePlanAdmission)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	It("can create a new Adapter instance", func() {
		Expect(reflect.TypeOf(NewAdapter(release, ctrl.Log, k8sClient, ctx))).To(Equal(reflect.TypeOf(&Adapter{})))
	})

	It("ensures the finalizer is added to the Release object", func() {
		result, err := adapter.EnsureFinalizerIsAdded()
		Expect(!result.CancelRequest && err == nil).To(BeTrue())
		Expect(release.ObjectMeta.GetFinalizers()).To(ContainElement(finalizerName))

		// Call should not fail if the finalizer already exists
		result, err = adapter.EnsureFinalizerIsAdded()
		Expect(!result.CancelRequest && err == nil).To(BeTrue())
	})

	It("ensures the ReleasePlanAdmission is checked to be enabled", func() {
		result, err := adapter.EnsureReleasePlanAdmissionEnabled()
		Expect(!result.RequeueRequest && !result.CancelRequest && err == nil).To(BeTrue())
	})

	It("ensures processing stops if the ReleasePlanAdmission is found to be disabled", func() {
		// Disable the ReleasePlanAdmission
		patch := client.MergeFrom(releasePlanAdmission.DeepCopy())
		releasePlanAdmission.SetLabels(map[string]string{
			appstudiov1alpha1.AutoReleaseLabel: "false",
		})
		Expect(k8sClient.Patch(ctx, releasePlanAdmission, patch)).Should(Succeed())

		Eventually(func() bool {
			result, _ := adapter.EnsureReleasePlanAdmissionEnabled()
			return !result.RequeueRequest && result.CancelRequest &&
				release.Status.Conditions[0].Reason == string(appstudiov1alpha1.ReleaseReasonTargetDisabledError)
		})

		// Reenable the ReleasePlanAdmission
		patch = client.MergeFrom(releasePlanAdmission.DeepCopy())
		releasePlanAdmission.SetLabels(map[string]string{
			appstudiov1alpha1.AutoReleaseLabel: "true",
		})
		Expect(k8sClient.Patch(ctx, releasePlanAdmission, patch)).Should(Succeed())
	})

	It("ensures a PipelineRun object exists", func() {
		Eventually(func() bool {
			result, err := adapter.EnsureReleasePipelineRunExists()
			return !result.CancelRequest && err == nil
		}, time.Second*10).Should(BeTrue())

		pipelineRun, err := adapter.getReleasePipelineRun()
		Expect(pipelineRun != nil && err == nil).To(BeTrue())
		Expect(k8sClient.Delete(ctx, pipelineRun)).Should(Succeed())
	})

	It("ensures the Release Pipeline status is tracked", func() {
		result, err := adapter.EnsureReleasePipelineRunExists()
		Expect(!result.CancelRequest && err == nil).Should(BeTrue())

		pipelineRun, err := adapter.getReleasePipelineRun()
		Expect(pipelineRun != nil && err == nil).Should(BeTrue())

		result, err = adapter.EnsureReleasePipelineStatusIsTracked()
		Expect(!result.CancelRequest && err == nil).To(BeTrue())

		// It should return results.ContinueProcessing() in case the release is done
		release.MarkSucceeded()
		result, err = adapter.EnsureReleasePipelineStatusIsTracked()
		Expect(!result.CancelRequest && err == nil).To(BeTrue())

		// The Release is running but has no pipelineRun matching the release
		release.MarkRunning()

		// avoiding "(SA5011)"
		if pipelineRun != nil {
			pipelineRun.ObjectMeta.Labels = make(map[string]string)
		}
		Expect(k8sClient.Update(ctx, pipelineRun)).Should(Succeed())

		release.Status.Conditions = []metav1.Condition{}
		result, err = adapter.EnsureReleasePipelineStatusIsTracked()
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			pipelineRun, err := adapter.getReleasePipelineRun()
			return pipelineRun == nil && err == nil

		}, time.Second*10).Should(BeTrue())
		Expect(k8sClient.Delete(ctx, pipelineRun)).Should(Succeed())
	})

	Context("When calling EnsureSnapshotEnvironmentBindingIsCreated", func() {
		It("skips the operation if the release has not succeeded yet", func() {
			result, err := adapter.EnsureSnapshotEnvironmentBindingIsCreated()
			Expect(result.RequeueRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if the release has already being deployed", func() {
			release.MarkRunning()
			release.MarkSucceeded()
			release.Status.SnapshotEnvironmentBinding = "foo"
			result, err := adapter.EnsureSnapshotEnvironmentBindingIsCreated()
			Expect(result.RequeueRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if no environment is set in the ReleasePlanAdmission", func() {
			release.MarkRunning()
			release.MarkSucceeded()

			environmentBeforeUpdate := releasePlanAdmission.Spec.Environment

			// Remove the Environment from the ReleasePlanAdmission
			patch := client.MergeFrom(releasePlanAdmission.DeepCopy())
			releasePlanAdmission.Spec.Environment = ""
			Expect(k8sClient.Patch(ctx, releasePlanAdmission, patch)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      releasePlanAdmission.Name,
					Namespace: releasePlanAdmission.Namespace,
				}, releasePlanAdmission)

				return err == nil && releasePlanAdmission.Spec.Environment == ""
			}, time.Second*10).Should(BeTrue())

			result, err := adapter.EnsureSnapshotEnvironmentBindingIsCreated()
			Expect(result.RequeueRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			// Blocking for 5 seconds to ensure that no binding is created
			Consistently(func() bool {
				binding, err := adapter.getSnapshotEnvironmentBinding(environment, releasePlanAdmission)

				return err == nil && binding == nil
			}, time.Second*5).Should(BeTrue())

			// Restore the Environment value
			patch = client.MergeFrom(releasePlanAdmission.DeepCopy())
			releasePlanAdmission.Spec.Environment = environmentBeforeUpdate
			Expect(k8sClient.Patch(ctx, releasePlanAdmission, patch)).Should(Succeed())
		})

		It("fails when the required resources are not present", func() {
			release.MarkRunning()
			release.MarkSucceeded()

			// Delete the application which is one of the required resources
			Expect(k8sClient.Delete(ctx, application)).Should(Succeed())
			Eventually(func() bool {
				_, err := adapter.getApplication(releasePlanAdmission)

				return err != nil
			}, time.Second*10).Should(BeTrue())

			result, err := adapter.EnsureSnapshotEnvironmentBindingIsCreated()
			Expect(result.RequeueRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			// Recreate the application
			application.ObjectMeta.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, application)).Should(Succeed())
		})

		It("creates or updates a binding and updates the release status", func() {
			release.MarkRunning()
			release.MarkSucceeded()
			result, err := adapter.EnsureSnapshotEnvironmentBindingIsCreated()
			Expect(result.RequeueRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			binding, err := adapter.getSnapshotEnvironmentBinding(environment, releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(binding).ToNot(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      release.Name,
					Namespace: release.Namespace,
				}, release)

				return err == nil && release.Status.SnapshotEnvironmentBinding != ""
			}, time.Second*10).Should(BeTrue())

			// Delete binding to clean up
			Expect(k8sClient.Delete(ctx, binding)).Should(Succeed())
		})
	})

	Context("When calling createOrUpdateSnapshotEnvironmentBinding", func() {
		It("fails when the required resources are not present", func() {
			// Delete the application which is one of the required resources
			Expect(k8sClient.Delete(ctx, application)).Should(Succeed())
			Eventually(func() bool {
				_, err := adapter.getApplication(releasePlanAdmission)

				return err != nil
			}, time.Second*10).Should(BeTrue())

			_, err := adapter.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			// Recreate the application
			application.ObjectMeta.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, application)).Should(Succeed())
		})

		It("creates a new binding if a previous one didn't exist", func() {
			Eventually(func() bool {
				binding, err := adapter.getSnapshotEnvironmentBinding(environment, releasePlanAdmission)

				return err == nil && binding == nil
			}, time.Second*10).Should(BeTrue())

			binding, err := adapter.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(binding).ToNot(BeNil())

			Eventually(func() bool {
				binding, err = adapter.getSnapshotEnvironmentBinding(environment, releasePlanAdmission)
				return err == nil && binding != nil
			}, time.Second*10).Should(BeTrue())

			// Owner reference should be set on creation
			Expect(len(binding.OwnerReferences)).To(Equal(1))

			// Delete binding to clean up
			Expect(k8sClient.Delete(ctx, binding)).Should(Succeed())
		})

		It("updates the binding if it exists", func() {
			binding, err := adapter.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(binding).ToNot(BeNil())

			Eventually(func() bool {
				binding, err = adapter.getSnapshotEnvironmentBinding(environment, releasePlanAdmission)
				return err == nil && binding != nil
			}, time.Second*10).Should(BeTrue())

			// Trigger an update by creating a new component that will produce a change in the binding spec
			componentsBeforeUpdate, err := adapter.getApplicationComponents(application)
			Expect(err).NotTo(HaveOccurred())

			newComponent := component.DeepCopy()
			newComponent.ObjectMeta = metav1.ObjectMeta{
				Name:      "test-component2",
				Namespace: "default",
			}
			Expect(k8sClient.Create(ctx, newComponent)).Should(Succeed())

			// Wait for the application to keep track of the new component
			Eventually(func() bool {
				components, err := adapter.getApplicationComponents(application)
				return err == nil && len(components) == len(componentsBeforeUpdate)+1
			}, time.Second*10).Should(BeTrue())

			versionBeforeUpdate := binding.ResourceVersion

			binding, err = adapter.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(binding).ToNot(BeNil())

			Eventually(func() bool {
				binding, err = adapter.getSnapshotEnvironmentBinding(environment, releasePlanAdmission)

				// Binding should exist and its version should have increased
				return err == nil && binding != nil && versionBeforeUpdate < binding.ResourceVersion
			}, time.Second*10).Should(BeTrue())

			// Owner reference should be kept on update
			Expect(len(binding.OwnerReferences)).To(Equal(1))

			// Delete binding and component to clean up
			Expect(k8sClient.Delete(ctx, binding)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, newComponent)).Should(Succeed())
		})
	})

	Context("When calling getSnapshotEnvironmentResources", func() {
		It("should return all the resources", func() {
			application, components, snapshot, environment, err := adapter.getSnapshotEnvironmentResources(releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(application).ToNot(BeNil())
			Expect(components).ToNot(BeNil())
			Expect(snapshot).ToNot(BeNil())
			Expect(environment).ToNot(BeNil())
		})

		It("should fail when any of the resources is not present", func() {
			Expect(k8sClient.Delete(ctx, application)).Should(Succeed())
			Eventually(func() bool {
				_, err := adapter.getApplication(releasePlanAdmission)

				return err != nil
			}, time.Second*10).Should(BeTrue())

			retrievedApplication, _, _, _, err := adapter.getSnapshotEnvironmentResources(releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
			Expect(retrievedApplication).To(BeNil())

			// Recreate the application
			application.ObjectMeta.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, application)).Should(Succeed())
		})
	})

	Context("When calling getApplication", func() {
		It("should return the Application if it exists", func() {
			application, err := adapter.getApplication(releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(application).ToNot(BeNil())
		})

		It("should fail if the Application doesn't exist", func() {
			Expect(k8sClient.Delete(ctx, application)).Should(Succeed())
			Eventually(func() bool {
				_, err := adapter.getApplication(releasePlanAdmission)

				return err != nil
			}, time.Second*10).Should(BeTrue())

			retrievedApplication, err := adapter.getApplication(releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(retrievedApplication).To(BeNil())

			// Recreate the application
			application.ObjectMeta.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, application)).Should(Succeed())
		})
	})

	Context("When calling getApplicationComponents", func() {
		It("should return the list of components", func() {
			components, err := adapter.getApplicationComponents(application)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(components) > 0).To(BeTrue())
		})
	})

	Context("When calling getEnvironment", func() {
		It("should return the Environment if it exists", func() {
			application, err := adapter.getEnvironment(releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(application).ToNot(BeNil())
		})

		It("should fail if the Environment doesn't exist", func() {
			Expect(k8sClient.Delete(ctx, environment)).Should(Succeed())
			Eventually(func() bool {
				_, err := adapter.getEnvironment(releasePlanAdmission)

				return err != nil
			}, time.Second*10).Should(BeTrue())

			retrievedEnvironment, err := adapter.getEnvironment(releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(retrievedEnvironment).To(BeNil())

			// Recreate the environment
			environment.ObjectMeta.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, environment)).Should(Succeed())
		})
	})

	Context("When calling getSnapshotEnvironmentBinding", func() {
		It("should return the SnapshotEnvironmentBinding if it exists", func() {
			_, components, snapshot, environment, err := adapter.getSnapshotEnvironmentResources(releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			binding := gitops.NewSnapshotEnvironmentBinding(components, snapshot, environment)
			Expect(k8sClient.Create(ctx, binding)).Should(Succeed())

			Eventually(func() bool {
				binding, err := adapter.getSnapshotEnvironmentBinding(environment, releasePlanAdmission)

				return err == nil && binding != nil
			}, time.Second*10).Should(BeTrue())

			// Delete binding to clean up
			Expect(k8sClient.Delete(ctx, binding)).Should(Succeed())
		})
	})

	Context("When calling syncResources", func() {
		It("should fail if there's no active ReleasePlanAdmission", func() {
			Expect(k8sClient.Delete(ctx, releasePlanAdmission)).Should(Succeed())
			Eventually(func() bool {
				_, err := adapter.getActiveReleasePlanAdmission()

				return err != nil
			}, time.Second*10).Should(BeTrue())

			Expect(adapter.syncResources()).ToNot(Succeed())

			// Recreate the ReleasePlanAdmission
			releasePlanAdmission.ObjectMeta.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())
		})

		It("should fail if there's no Snapshot", func() {
			Expect(k8sClient.Delete(ctx, snapshot)).Should(Succeed())
			Eventually(func() bool {
				_, err := adapter.getSnapshot()

				return err != nil
			}, time.Second*10).Should(BeTrue())

			Expect(adapter.syncResources()).ToNot(Succeed())

			// Recreate the Snapshot
			snapshot.ObjectMeta.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, snapshot)).Should(Succeed())
		})

		It("should sync resources properly", func() {
			Expect(adapter.syncResources()).To(Succeed())
		})
	})

	It("ensures finalizers are called when a release is deleted", func() {
		// Call to EnsureFinalizersAreCalled should succeed if the Release was not set to be deleted
		result, err := adapter.EnsureFinalizersAreCalled()
		Expect(!result.CancelRequest && !result.RequeueRequest).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())

		// Add finalizer
		result, err = adapter.EnsureFinalizerIsAdded()
		Expect(result.CancelRequest).To(BeFalse())
		Expect(err).NotTo(HaveOccurred())

		// Create a ReleasePipelineRun to ensure it gets finalized
		result, err = adapter.EnsureReleasePipelineRunExists()
		Expect(result.CancelRequest).To(BeFalse())
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, release)).ToNot(HaveOccurred())

		Eventually(func() bool {
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      release.Name,
				Namespace: release.Namespace,
			}, release)
			return err == nil && release.GetDeletionTimestamp() != nil && release.Status.ReleasePipelineRun != ""
		}, time.Second*10).Should(BeTrue())

		result, err = adapter.EnsureFinalizersAreCalled()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(err == nil || errors.IsNotFound(err)).Should(BeTrue())

		Expect(result.RequeueRequest).Should(BeTrue())

		Eventually(func() bool {
			pipelineRun, _ := adapter.getReleasePipelineRun()
			return pipelineRun == nil
		}, time.Second*10).Should(BeTrue())
	})

	It("can finalize a Release", func() {
		// Release should be properly finalized if there is no ReleasePipelineRun
		Expect(adapter.finalizeRelease()).To(Succeed())

		// It should finalize properly as well when there's a ReleasePipelineRun
		result, err := adapter.EnsureReleasePipelineRunExists()
		Expect(result.CancelRequest).To(BeFalse())
		Expect(err).NotTo(HaveOccurred())
		Expect(adapter.finalizeRelease()).To(Succeed())
		Eventually(func() bool {
			pipelineRun, _ := adapter.getReleasePipelineRun()
			return pipelineRun == nil
		}, time.Second*10).Should(BeTrue())
	})

	Context("When createReleasePipelineRun is called", func() {

		BeforeEach(func() {
			snapshot.TypeMeta.Kind = "snapshot"
			Expect(k8sClient.Update(ctx, snapshot)).Should(Succeed())
		})

		It("is a PipelineRun type as AsPipelineRun was implicitly called", func() {
			pipelineRun, err := adapter.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			Expect(err).NotTo(HaveOccurred())
			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1beta1.PipelineRun{})))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("has the owner annotation as WithOwner was implicitly called", func() {
			pipelineRun, err := adapter.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun.GetAnnotations()[handler.NamespacedNameAnnotation]).To(ContainSubstring(release.Name))
			Expect(pipelineRun.GetAnnotations()[handler.TypeAnnotation]).To(ContainSubstring("Release"))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("contains the release labels as WithReleaseLabels was implicitly called", func() {
			pipelineRun, err := adapter.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun.GetLabels()[tekton.PipelinesTypeLabel]).To(Equal("release"))
			Expect(pipelineRun.GetLabels()[tekton.ReleaseNameLabel]).To(Equal(release.Name))
			Expect(pipelineRun.GetLabels()[tekton.ReleaseNamespaceLabel]).To(Equal(testNamespace))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("contains the release strategy as WithReleaseStrategy was implicitly called", func() {
			pipelineRun, err := adapter.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun.Spec.PipelineRef.Name).To(Equal(releaseStrategy.Spec.Pipeline))
			Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(releaseStrategy.Spec.Bundle))
			Expect(pipelineRun.Spec.ServiceAccountName).Should(ContainSubstring("test-account"))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("contains a json representation of the enterprise contract policy as WithEnterpriseContractPolicy was called", func() {
			pipelineRun, err := adapter.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			Expect(err).NotTo(HaveOccurred())
			enterpriseContractPolicy, err := adapter.getEnterpriseContractPolicy(releaseStrategy)
			Expect(err).ShouldNot(HaveOccurred())

			enterpriseContractPolicyJsonSpec, _ := json.Marshal(enterpriseContractPolicy.Spec)
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(enterpriseContractPolicyJsonSpec)))))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("has the snapshot application name as WithSnapshot was implicitly called", func() {
			pipelineRun, err := adapter.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", ContainSubstring(snapshot.Spec.Application))))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})
	})

	It("can return the pipelineRun from the release", func() {
		result, err := adapter.EnsureReleasePipelineRunExists()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result.CancelRequest).To(BeFalse())

		pipelineRun, err := adapter.getReleasePipelineRun()
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1beta1.PipelineRun{})))
	})

	It("can return the releaseStrategy referenced in a given ReleasePlanAdmission", func() {
		strategy, err := adapter.getReleaseStrategy(releasePlanAdmission)
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(strategy)).To(Equal(reflect.TypeOf(&appstudiov1alpha1.ReleaseStrategy{})))
	})

	It("can return the ReleasePlanAdmission targeted from a given ReleasePlan", func() {
		link, _ := adapter.getActiveReleasePlanAdmission()
		Expect(reflect.TypeOf(link)).To(Equal(reflect.TypeOf(&appstudiov1alpha1.ReleasePlanAdmission{})))
		Expect(link.Spec.Application).To(Equal("test-app"))
		Expect(link.Spec.ReleaseStrategy).To(Equal(releaseStrategy.Name))
		Expect(link.Namespace).Should(Equal(releasePlan.Spec.Target))

		// It should return nil if the ReleasePlanAdmission has auto-release set to false
		patch := client.MergeFrom(link.DeepCopy())
		link.SetLabels(map[string]string{
			appstudiov1alpha1.AutoReleaseLabel: "false",
		})
		Expect(k8sClient.Patch(ctx, link, patch)).Should(Succeed())
		Eventually(func() bool {
			activeReleasePlanAdmission, err := adapter.getActiveReleasePlanAdmission()
			return activeReleasePlanAdmission == nil && err != nil &&
				strings.Contains(err.Error(), "with auto-release label set to false")
		}, time.Second*10).Should(BeTrue())
	})

	It("fails to find the target ReleasePlanAdmission when target does not match", func() {
		patch := client.MergeFrom(releasePlan.DeepCopy())
		releasePlan.Spec.Target = "foo"
		Expect(k8sClient.Patch(ctx, releasePlan, patch)).Should(Succeed())
		Eventually(func() bool {
			activeReleasePlanAdmission, err := adapter.getActiveReleasePlanAdmission()
			return activeReleasePlanAdmission == nil && err != nil &&
				strings.Contains(err.Error(), "no ReleasePlanAdmission found in the target")
		}, time.Second*10).Should(BeTrue())
	})

	It("can mark the status for a given Release according to the pipelineRun status", func() {
		pipelineRun := tekton.NewReleasePipelineRun("test-pipeline", testNamespace).AsPipelineRun()
		pipelineRun.Name = "foo" // PipelineRun needs a name before registering the Release status
		err := adapter.registerReleaseStatusData(pipelineRun, releaseStrategy)
		Expect(err).ToNot(HaveOccurred())
		Expect(adapter.release.Status.StartTime).ToNot(BeNil())
		pipelineRun.Status.MarkSucceeded("succeeded", "set it to succeeded")
		Expect(pipelineRun.IsDone()).To(BeTrue())

		err = adapter.registerReleasePipelineRunStatus(pipelineRun)
		Expect(err).ToNot(HaveOccurred())
		Expect(release.HasSucceeded()).To(BeTrue())

		// Double check that we still have a valid StartTime
		Expect(adapter.release.Status.StartTime).ToNot(BeNil())

		// Clear up previous condition so isDone returns false
		release.Status.Conditions = []metav1.Condition{}
		pipelineRun.Status.MarkFailed("failed", "set it to failed")
		err = adapter.registerReleasePipelineRunStatus(pipelineRun)
		Expect(err).NotTo(HaveOccurred())
		Expect(release.HasSucceeded()).To(BeFalse())
	})

	It("can add the release information to its status and mark it as Running", func() {
		// The call should succeed if releaseStrategy or pipelineRun are nil
		Expect(adapter.registerReleaseStatusData(nil, nil)).To(Succeed())

		pipelineRun := tekton.NewReleasePipelineRun("test-pipeline", testNamespace).AsPipelineRun()
		pipelineRun.Name = "new-name"
		err := adapter.registerReleaseStatusData(pipelineRun, releaseStrategy)
		Expect(err).NotTo(HaveOccurred())
		Expect(release.Status.ReleasePipelineRun).To(Equal(testNamespace + "/" + pipelineRun.Name))
		Expect(release.Status.Conditions[0].Status == metav1.ConditionUnknown)
		Expect(release.Status.Conditions[0].Reason == string(appstudiov1alpha1.ReleaseReasonRunning))
	})

	It("can update the Status of a release", func() {
		patch := client.MergeFrom(adapter.release.DeepCopy())
		adapter.release.Status.ReleasePipelineRun = "foo/test"
		Expect(k8sClient.Status().Patch(ctx, release, patch)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      release.Name,
				Namespace: testNamespace,
			}, release)
			return err == nil && release.Status.ReleasePipelineRun == "foo/test"
		}, time.Second*10).Should(BeTrue())
	})

	It("can get an existing Snapshot", func() {
		snapshot, err := adapter.getSnapshot()
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(snapshot)).To(Equal(reflect.TypeOf(&applicationapiv1alpha1.Snapshot{})))

		Expect(k8sClient.Delete(ctx, snapshot)).Should(Succeed())
		Eventually(func() bool {
			_, err = adapter.getSnapshot()
			return errors.IsNotFound(err)
		}, time.Second*10).Should(BeTrue())
	})

	It("can retrieve an existing EnterpriseContractPolicy from a release strategy", func() {
		ecpolicy, err := adapter.getEnterpriseContractPolicy(releaseStrategy)
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(ecpolicy)).To(Equal(reflect.TypeOf(&ecapiv1alpha1.EnterpriseContractPolicy{})))
	})

	It("returns an IsNotFound error if the EnterpriseContractPolicy does not exist", func() {
		Expect(k8sClient.Delete(ctx, enterpriseContractPolicy)).Should(Succeed())
		Eventually(func() bool {
			_, err := adapter.getEnterpriseContractPolicy(releaseStrategy)
			return errors.IsNotFound(err)
		}, time.Second*10).Should(BeTrue())

		enterpriseContractPolicy.ResourceVersion = ""
		Expect(k8sClient.Create(ctx, enterpriseContractPolicy)).Should(Succeed())
	})

	It("can return the ReleasePlan from the release", func() {
		link, err := adapter.getReleasePlan()
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(link)).To(Equal(reflect.TypeOf(&appstudiov1alpha1.ReleasePlan{})))
	})
})
