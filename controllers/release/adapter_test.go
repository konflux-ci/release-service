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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/operator-framework/operator-lib/handler"
	"github.com/redhat-appstudio/operator-toolkit/controller"
	toolkit "github.com/redhat-appstudio/operator-toolkit/loader"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"github.com/redhat-appstudio/release-service/metadata"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Release adapter", Ordered, func() {
	var (
		createReleaseAndAdapter func() *adapter
		createResources         func()
		deleteResources         func()

		application                 *applicationapiv1alpha1.Application
		component                   *applicationapiv1alpha1.Component
		enterpriseContractConfigMap *corev1.ConfigMap
		enterpriseContractPolicy    *ecapiv1alpha1.EnterpriseContractPolicy
		environment                 *applicationapiv1alpha1.Environment
		releasePlan                 *v1alpha1.ReleasePlan
		releasePlanAdmission        *v1alpha1.ReleasePlanAdmission
		releaseServiceConfig        *v1alpha1.ReleaseServiceConfig
		snapshot                    *applicationapiv1alpha1.Snapshot
		snapshotEnvironmentBinding  *applicationapiv1alpha1.SnapshotEnvironmentBinding
	)

	AfterAll(func() {
		deleteResources()
	})

	BeforeAll(func() {
		createResources()
	})

	When("newAdapter is called", func() {
		It("creates and return a new adapter", func() {
			Expect(reflect.TypeOf(newAdapter(ctx, k8sClient, nil, loader.NewLoader(), &ctrl.Log))).To(Equal(reflect.TypeOf(&adapter{})))
		})
	})

	Context("When calling EnsureConfigIsLoaded", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("returns an error if SERVICE_NAMESPACE is not set", func() {
			adapter.release.MarkReleasing("")
			result, err := adapter.EnsureConfigIsLoaded()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
			Expect(adapter.release.HasReleaseFinished()).To(BeTrue())
			Expect(adapter.releaseServiceConfig).To(BeNil())
		})

		It("loads the ReleaseServiceConfig and assigns it to the adapter", func() {
			os.Setenv("SERVICE_NAMESPACE", "default")
			result, err := adapter.EnsureConfigIsLoaded()
			Expect(!result.CancelRequest && !result.RequeueRequest).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(adapter.releaseServiceConfig).NotTo(BeNil())
			Expect(adapter.releaseServiceConfig.Namespace).To(Equal("default"))
		})

		It("creates and assigns an empty ReleaseServiceConfig if none is found", func() {
			os.Setenv("SERVICE_NAMESPACE", "test")
			result, err := adapter.EnsureConfigIsLoaded()
			Expect(!result.CancelRequest && !result.RequeueRequest).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(adapter.releaseServiceConfig).NotTo(BeNil())
			Expect(adapter.releaseServiceConfig.Namespace).To(Equal("test"))
		})
	})

	When("EnsureFinalizersAreCalled is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should do nothing if the Release is not set to be deleted", func() {
			result, err := adapter.EnsureFinalizersAreCalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(!result.CancelRequest && !result.RequeueRequest).To(BeTrue())
		})

		It("should finalize the Release if it's set to be deleted and it has a finalizer", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						EnterpriseContractConfigMap: enterpriseContractConfigMap,
						EnterpriseContractPolicy:    enterpriseContractPolicy,
						ReleasePlan:                 releasePlan,
						ReleasePlanAdmission:        releasePlanAdmission,
						Snapshot:                    snapshot,
					},
				},
			})
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Finalizers).To(HaveLen(1))

			result, err = adapter.EnsureReleaseIsProcessed()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.client.Delete(adapter.ctx, adapter.release)).To(Succeed())
			adapter.release, err = adapter.loader.GetRelease(adapter.ctx, adapter.client, adapter.release.Name, adapter.release.Namespace)
			Expect(adapter.release).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.DeletionTimestamp).NotTo(BeNil())

			result, err = adapter.EnsureFinalizersAreCalled()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
			Expect(pipelineRun).To(Or(BeNil(), HaveField("DeletionTimestamp", Not(BeNil()))))
			Expect(err).NotTo(HaveOccurred())

			_, err = adapter.loader.GetRelease(adapter.ctx, adapter.client, adapter.release.Name, adapter.release.Namespace)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	When("EnsureFinalizerIsAdded is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should add a finalizer to the Release", func() {
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Finalizers).To(ContainElement(metadata.ReleaseFinalizer))
		})

		It("shouldn't fail if the Release already has the finalizer added", func() {
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Finalizers).To(ContainElement(metadata.ReleaseFinalizer))

			result, err = adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("EnsureReleaseIsCompleted is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.release.MarkReleasing("")
		})

		It("should not change the release status if it's set already", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()
			adapter.release.MarkDeploying("")
			adapter.release.MarkDeployed()
			adapter.release.MarkReleaseFailed("")
			result, err := adapter.EnsureReleaseIsCompleted()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasReleaseFinished()).To(BeTrue())
			Expect(adapter.release.IsReleased()).To(BeFalse())
		})

		It("should do nothing if the processing has not completed", func() {
			result, err := adapter.EnsureReleaseIsCompleted()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasReleaseFinished()).To(BeFalse())
		})

		It("should do nothing if a deployment is required and it's not complete", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()
			result, err := adapter.EnsureReleaseIsCompleted()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasReleaseFinished()).To(BeFalse())
		})

		It("should complete the release if all the required phases have completed", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()
			adapter.release.MarkDeploying("")
			adapter.release.MarkDeployed()
			result, err := adapter.EnsureReleaseIsCompleted()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasReleaseFinished()).To(BeTrue())
		})
	})

	When("EnsureReleaseIsDeployed is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("skips the operation if the Release processing has not succeeded yet", func() {
			result, err := adapter.EnsureReleaseIsDeployed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if the Release has already being deployed", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()
			adapter.release.MarkDeploying("")
			adapter.release.MarkDeployed()

			result, err := adapter.EnsureReleaseIsDeployed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if no environment is set in the ReleasePlanAdmission", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()

			newReleasePlanAdmission := &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan-admission",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Applications: []string{"app"},
					Origin:       "default",
					PipelineRef: &tektonutils.PipelineRef{
						Resolver: "bundles",
						Params: []tektonutils.Param{
							{Name: "bundle", Value: "quay.io/some/bundle"},
							{Name: "name", Value: "release-pipeline"},
							{Name: "kind", Value: "pipeline"},
						},
					},
					Policy: enterpriseContractPolicy.Name,
				},
			}
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
			})

			result, err := adapter.EnsureReleaseIsDeployed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Deployment.SnapshotEnvironmentBinding).To(BeEmpty())
		})

		It("fails when the ReleasePlanAdmission is not present", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			result, err := adapter.EnsureReleaseIsDeployed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("updates the binding if one already exists", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
					Resource:   snapshotEnvironmentBinding,
				},
				{
					ContextKey: loader.DeploymentResourcesContextKey,
					Resource: &loader.DeploymentResources{
						Application:           application,
						ApplicationComponents: []applicationapiv1alpha1.Component{*component},
						Environment:           environment,
						Snapshot:              snapshot,
					},
				},
			})

			result, err := adapter.EnsureReleaseIsDeployed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Deployment.SnapshotEnvironmentBinding).NotTo(BeEmpty())

			binding, _ := adapter.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(adapter.ctx, adapter.client, adapter.release)
			Expect(binding).NotTo(BeNil())
			Expect(binding.Annotations).To(HaveLen(2))
			Expect(binding.Annotations[handler.NamespacedNameAnnotation]).To(
				Equal(adapter.release.Namespace + "/" + adapter.release.Name),
			)
			Expect(binding.Annotations[handler.TypeAnnotation]).To(Equal(adapter.release.Kind))
		})

		It("creates a binding and updates the release status", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
				},
				{
					ContextKey: loader.DeploymentResourcesContextKey,
					Resource: &loader.DeploymentResources{
						Application:           application,
						ApplicationComponents: []applicationapiv1alpha1.Component{*component},
						Environment:           environment,
						Snapshot:              snapshot,
					},
				},
			})

			result, err := adapter.EnsureReleaseIsDeployed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Deployment.SnapshotEnvironmentBinding).NotTo(BeEmpty())

			// Restore the context to get the actual binding
			adapter.ctx = context.TODO()

			binding, err := adapter.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(adapter.ctx, adapter.client, adapter.release)
			Expect(binding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, binding)).Should(Succeed())
		})
	})

	When("EnsureReleaseDeploymentIsTracked is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("skips the operation if the deployment has not started yet", func() {
			result, err := adapter.EnsureReleaseDeploymentIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if the deployment has finished", func() {
			adapter.release.MarkDeploying("")
			adapter.release.MarkDeployed()

			result, err := adapter.EnsureReleaseDeploymentIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails if the binding is not found", func() {
			adapter.release.MarkDeploying("")
			adapter.release.Status.Deployment.SnapshotEnvironmentBinding = "not/found"

			result, err := adapter.EnsureReleaseDeploymentIsTracked()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("skips the operation if the binding isn't owned by the release", func() {
			adapter.release.MarkDeploying("")

			newSnapshotEnvironmentBinding := snapshotEnvironmentBinding.DeepCopy()
			newSnapshotEnvironmentBinding.Status.ComponentDeploymentConditions = []metav1.Condition{
				{
					Type:   applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
					Status: metav1.ConditionTrue,
					Reason: "Deployed",
				},
			}
			newSnapshotEnvironmentBinding.Annotations = map[string]string{
				handler.TypeAnnotation:           adapter.release.Kind,
				handler.NamespacedNameAnnotation: "other-release",
			}
			adapter.release.Status.Deployment.SnapshotEnvironmentBinding = newSnapshotEnvironmentBinding.Namespace + "/" + newSnapshotEnvironmentBinding.Name

			result, err := adapter.EnsureReleaseDeploymentIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("tracks the binding if one is found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
					Resource:   snapshotEnvironmentBinding,
				},
			})
			adapter.release.MarkDeploying("")
			adapter.release.Status.Deployment.SnapshotEnvironmentBinding = snapshotEnvironmentBinding.Namespace + "/" + snapshotEnvironmentBinding.Name

			result, err := adapter.EnsureReleaseDeploymentIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("EnsureReleaseIsRunning is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should stop processing if the release has finished", func() {
			adapter.release.MarkReleasing("")
			adapter.release.MarkReleased()

			result, err := adapter.EnsureReleaseIsRunning()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should mark the Release as releasing if it is missing the status", func() {
			result, err := adapter.EnsureReleaseIsRunning()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsReleasing()).To(BeTrue())
		})

		It("should do nothing if the release is already running", func() {
			adapter.release.MarkReleasing("")

			result, err := adapter.EnsureReleaseIsRunning()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsReleasing()).To(BeTrue())
		})
	})

	When("EnsureReleaseIsProcessed is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should do nothing if the Release is already processed", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()

			result, err := adapter.EnsureReleaseIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsProcessing()).To(BeFalse())
		})

		It("should continue if the PipelineRun exists and the release processing has started", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource: &tektonv1.PipelineRun{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-run",
							Namespace: "default",
						},
					},
				},
			})
			adapter.release.MarkProcessing("")

			result, err := adapter.EnsureReleaseIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should register the processing data if the PipelineRun already exists", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource: &tektonv1.PipelineRun{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-run",
							Namespace: "default",
						},
					},
				},
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						EnterpriseContractConfigMap: enterpriseContractConfigMap,
						EnterpriseContractPolicy:    enterpriseContractPolicy,
						ReleasePlanAdmission:        releasePlanAdmission,
						Snapshot:                    snapshot,
					},
				},
			})

			result, err := adapter.EnsureReleaseIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsProcessing()).To(BeTrue())
		})

		It("should requeue the Release if any of the resources is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			result, err := adapter.EnsureReleaseIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should create a pipelineRun and register the processing data if all the required resources are present", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						EnterpriseContractConfigMap: enterpriseContractConfigMap,
						EnterpriseContractPolicy:    enterpriseContractPolicy,
						ReleasePlan:                 releasePlan,
						ReleasePlanAdmission:        releasePlanAdmission,
						Snapshot:                    snapshot,
					},
				},
			})

			result, err := adapter.EnsureReleaseIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsProcessing()).To(BeTrue())

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})
	})

	When("EnsureReleaseIsValid is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.release.MarkReleasing("")
			Expect(adapter.client.Status().Update(adapter.ctx, adapter.release)).To(Succeed())
		})

		It("should mark the release as validated if all checks pass", func() {
			adapter.validations = []controller.ValidationFunction{}

			result, err := adapter.EnsureReleaseIsValid()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeTrue())
			Expect(adapter.release.HasReleaseFinished()).To(BeFalse())
		})

		It("should mark the release as failed if a validation fails", func() {
			adapter.validations = []controller.ValidationFunction{
				func() *controller.ValidationResult {
					return &controller.ValidationResult{Valid: false}
				},
			}

			result, err := adapter.EnsureReleaseIsValid()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
			Expect(adapter.release.HasReleaseFinished()).To(BeTrue())
		})

		It("should requeue the release if a validation fails with an error", func() {
			adapter.validations = []controller.ValidationFunction{
				func() *controller.ValidationResult {
					return &controller.ValidationResult{Err: fmt.Errorf("internal error")}
				},
			}

			result, err := adapter.EnsureReleaseIsValid()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.release.HasReleaseFinished()).To(BeFalse())
		})

		It("does not clear the release status", func() {
			adapter.validations = []controller.ValidationFunction{}

			result, err := adapter.EnsureReleaseIsValid()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.StartTime).NotTo(BeNil())
		})
	})

	When("EnsureReleaseProcessingIsTracked is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the Release processing has not started", func() {
			result, err := adapter.EnsureReleaseProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should continue if the Release processing has finished", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()

			result, err := adapter.EnsureReleaseProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should track the status if the PipelineRun exists", func() {
			adapter.release.MarkProcessing("")

			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource:   pipelineRun,
				},
			})

			result, err := adapter.EnsureReleaseProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasProcessingFinished()).To(BeTrue())
		})

		It("removes the finalizer if present", func() {
			adapter.release.MarkProcessing("")

			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pipeline-run",
					Namespace:  "default",
					Finalizers: []string{metadata.ReleaseFinalizer},
				},
			}
			// The resource needs to be created as it will get patched
			Expect(adapter.client.Create(adapter.ctx, pipelineRun)).To(Succeed())

			pipelineRun.Status.MarkSucceeded("", "")

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource:   pipelineRun,
				},
			})

			result, err := adapter.EnsureReleaseProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun.Finalizers).To(BeEmpty())

			// Clean up at the end
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should continue if the PipelineRun doesn't exist", func() {
			adapter.release.MarkProcessing("")

			result, err := adapter.EnsureReleaseProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("createReleasePipelineRun is called", func() {
		var (
			adapter     *adapter
			pipelineRun *tektonv1.PipelineRun
		)

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)

			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			resources := &loader.ProcessingResources{
				ReleasePlan:                 releasePlan,
				ReleasePlanAdmission:        releasePlanAdmission,
				EnterpriseContractConfigMap: enterpriseContractConfigMap,
				EnterpriseContractPolicy:    enterpriseContractPolicy,
				Snapshot:                    snapshot,
			}

			var err error
			pipelineRun, err = adapter.createReleasePipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a PipelineRun", func() {
			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1.PipelineRun{})))
		})

		It("has the release reference", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(adapter.release.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", adapter.release.Namespace, types.Separator, adapter.release.Name))))
		})

		It("has the releasePlan reference", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(releasePlan.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releasePlan.Namespace, types.Separator, releasePlan.Name))))
		})

		It("has the releasePlanAdmission reference", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(releasePlanAdmission.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releasePlanAdmission.Namespace, types.Separator, releasePlanAdmission.Name))))
		})

		It("has the snapshot reference", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(snapshot.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", snapshot.Namespace, types.Separator, snapshot.Name))))
		})

		It("has owner annotations", func() {
			Expect(pipelineRun.GetAnnotations()[handler.NamespacedNameAnnotation]).To(ContainSubstring(adapter.release.Name))
			Expect(pipelineRun.GetAnnotations()[handler.TypeAnnotation]).To(ContainSubstring("Release"))
		})

		It("has release labels", func() {
			Expect(pipelineRun.GetLabels()[metadata.PipelinesTypeLabel]).To(Equal("release"))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNameLabel]).To(Equal(adapter.release.Name))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNamespaceLabel]).To(Equal(testNamespace))
		})

		It("references the pipeline specified in the ReleasePlanAdmission", func() {
			var pipelineName string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "name" {
					pipelineName = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineName).To(Equal(releasePlanAdmission.Spec.PipelineRef.Params[1].Value))
		})

		It("contains the proper timeout value", func() {
			timeout := releasePlanAdmission.Spec.PipelineRef.Timeout
			Expect(pipelineRun.Spec.Timeouts.Pipeline.Duration.String()).To(Equal(string(timeout)))
		})

		It("contains a parameter with the verify ec task bundle", func() {
			bundle := enterpriseContractConfigMap.Data["verify_ec_task_bundle"]
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(bundle)))))
		})

		It("contains a parameter with the json representation of the EnterpriseContractPolicy", func() {
			jsonSpec, _ := json.Marshal(enterpriseContractPolicy.Spec)
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(jsonSpec)))))
		})
	})

	When("createOrUpdateSnapshotEnvironmentBinding is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("fails when the required resources are not present", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.DeploymentResourcesContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			_, err := adapter.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("creates a new binding owned by the release if the required resources are present", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
				},
				{
					ContextKey: loader.DeploymentResourcesContextKey,
					Resource: &loader.DeploymentResources{
						Application:           application,
						ApplicationComponents: []applicationapiv1alpha1.Component{*component},
						Environment:           environment,
						Snapshot:              snapshot,
					},
				},
			})

			binding, err := adapter.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(binding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(binding.OwnerReferences).To(HaveLen(1))
			Expect(binding.Annotations).To(HaveLen(2))
			Expect(binding.Annotations[handler.NamespacedNameAnnotation]).To(
				Equal(adapter.release.Namespace + "/" + adapter.release.Name),
			)
			Expect(binding.Annotations[handler.TypeAnnotation]).To(Equal(adapter.release.Kind))

			Expect(k8sClient.Delete(ctx, binding)).Should(Succeed())
		})

		It("updates a binding and marks it as owned by the release if a binding is already present", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
					Resource:   snapshotEnvironmentBinding,
				},
				{
					ContextKey: loader.DeploymentResourcesContextKey,
					Resource: &loader.DeploymentResources{
						Application:           application,
						ApplicationComponents: []applicationapiv1alpha1.Component{*component},
						Environment:           environment,
						Snapshot:              snapshot,
					},
				},
			})

			binding, err := adapter.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(binding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(binding.Annotations).To(HaveLen(2))
			Expect(binding.Annotations[handler.NamespacedNameAnnotation]).To(
				Equal(adapter.release.Namespace + "/" + adapter.release.Name),
			)
			Expect(binding.Annotations[handler.TypeAnnotation]).To(Equal(adapter.release.Kind))
		})
	})

	When("finalizeRelease is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("finalizes the Release successfully", func() {
			Expect(adapter.finalizeRelease()).To(Succeed())
		})

		It("finalizes the Release and deletes the PipelineRun", func() {
			resources := &loader.ProcessingResources{
				ReleasePlan:                 releasePlan,
				ReleasePlanAdmission:        releasePlanAdmission,
				EnterpriseContractConfigMap: enterpriseContractConfigMap,
				EnterpriseContractPolicy:    enterpriseContractPolicy,
				Snapshot:                    snapshot,
			}
			pipelineRun, err := adapter.createReleasePipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease()).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).To(BeNil())
		})
	})

	When("getEmptyReleaseServiceConfig is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should return a ReleaseServiceConfig without Spec and with the right ObjectMeta", func() {
			releaseServiceConfig := adapter.getEmptyReleaseServiceConfig("namespace")
			Expect(releaseServiceConfig).NotTo(BeNil())
			Expect(releaseServiceConfig.Name).To(Equal(v1alpha1.ReleaseServiceConfigResourceName))
			Expect(releaseServiceConfig.Namespace).To(Equal("namespace"))
		})
	})

	When("registerDeploymentData is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no binding", func() {
			Expect(adapter.registerDeploymentData(nil, releasePlanAdmission)).To(Succeed())
			Expect(adapter.release.Status.Deployment.SnapshotEnvironmentBinding).To(BeEmpty())
		})

		It("does nothing if there is no ReleasePlanAdmission", func() {
			Expect(adapter.registerDeploymentData(snapshotEnvironmentBinding, nil)).To(Succeed())
			Expect(adapter.release.Status.Deployment.SnapshotEnvironmentBinding).To(BeEmpty())
		})

		It("registers the Release deployment data", func() {
			Expect(adapter.registerDeploymentData(snapshotEnvironmentBinding, releasePlanAdmission)).To(Succeed())
			Expect(adapter.release.Status.Deployment.Environment).To(Equal(fmt.Sprintf("%s%c%s",
				releasePlanAdmission.Namespace, types.Separator, releasePlanAdmission.Spec.Environment)))
			Expect(adapter.release.Status.Deployment.SnapshotEnvironmentBinding).To(Equal(fmt.Sprintf("%s%c%s",
				snapshotEnvironmentBinding.Namespace, types.Separator, snapshotEnvironmentBinding.Name)))
			Expect(adapter.release.IsDeploying()).To(BeTrue())
		})

		It("should not register the environment if the ReleasePlanAdmission does not reference any", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Environment = ""

			Expect(adapter.registerDeploymentData(snapshotEnvironmentBinding, newReleasePlanAdmission)).To(Succeed())
			Expect(adapter.release.Status.Deployment.Environment).To(Equal(""))
			Expect(adapter.release.Status.Deployment.SnapshotEnvironmentBinding).To(Equal(fmt.Sprintf("%s%c%s",
				snapshotEnvironmentBinding.Namespace, types.Separator, snapshotEnvironmentBinding.Name)))
			Expect(adapter.release.IsDeploying()).To(BeTrue())
		})
	})

	When("registerDeploymentStatus is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no binding", func() {
			Expect(adapter.registerDeploymentStatus(nil)).To(Succeed())
			Expect(adapter.release.IsDeploying()).To(BeFalse())
		})

		It("does nothing if the binding doesn't have the expected condition", func() {
			Expect(adapter.registerDeploymentStatus(snapshotEnvironmentBinding)).To(Succeed())
			Expect(adapter.release.IsDeploying()).To(BeFalse())
		})

		It("registers the deployment status when the the deployment succeeded deployed", func() {
			adapter.release.MarkDeploying("")
			newSnapshotEnvironmentBinding := snapshotEnvironmentBinding.DeepCopy()
			newSnapshotEnvironmentBinding.Status.ComponentDeploymentConditions = []metav1.Condition{
				{
					Type:   applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
					Status: metav1.ConditionTrue,
					Reason: "Deployed",
				},
			}
			Expect(adapter.registerDeploymentStatus(newSnapshotEnvironmentBinding)).To(Succeed())
			Expect(adapter.release.HasDeploymentFinished()).To(BeTrue())
			Expect(adapter.release.IsDeployed()).To(BeTrue())
		})

		It("registers the deployment status when the deployment failed", func() {
			adapter.release.MarkDeploying("")
			newSnapshotEnvironmentBinding := snapshotEnvironmentBinding.DeepCopy()
			newSnapshotEnvironmentBinding.Status.ComponentDeploymentConditions = []metav1.Condition{
				{
					Type:   applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
					Status: metav1.ConditionFalse,
					Reason: applicationapiv1alpha1.ComponentDeploymentConditionErrorOccurred,
				},
			}
			Expect(adapter.registerDeploymentStatus(newSnapshotEnvironmentBinding)).To(Succeed())
			Expect(adapter.release.HasDeploymentFinished()).To(BeTrue())
			Expect(adapter.release.IsDeployed()).To(BeFalse())
			Expect(adapter.release.IsReleased()).To(BeFalse())
		})

		It("registers the deployment status when the deployment is progressing", func() {
			adapter.release.MarkDeploying("")
			newSnapshotEnvironmentBinding := snapshotEnvironmentBinding.DeepCopy()
			newSnapshotEnvironmentBinding.Status.ComponentDeploymentConditions = []metav1.Condition{
				{
					Type:   applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
					Status: metav1.ConditionFalse,
					Reason: "Deploying",
				},
			}
			Expect(adapter.registerDeploymentStatus(newSnapshotEnvironmentBinding)).To(Succeed())
			Expect(adapter.release.HasDeploymentFinished()).To(BeFalse())
		})
	})

	When("registerProcessingData is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerProcessingData(nil)).To(Succeed())
			Expect(adapter.release.Status.Processing.PipelineRun).To(BeEmpty())
		})

		It("registers the Release processing data", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			Expect(adapter.registerProcessingData(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.Processing.PipelineRun).To(Equal(fmt.Sprintf("%s%c%s",
				pipelineRun.Namespace, types.Separator, pipelineRun.Name)))
			Expect(adapter.release.Status.Target).To(Equal(pipelineRun.Namespace))
			Expect(adapter.release.IsProcessing()).To(BeTrue())
		})
	})

	When("registerProcessingStatus is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerProcessingStatus(nil)).To(Succeed())
			Expect(adapter.release.Status.Processing.CompletionTime).To(BeNil())
		})

		It("does nothing if the PipelineRun is not done", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			Expect(adapter.registerProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.Processing.CompletionTime).To(BeNil())
		})

		It("sets the Release as succeeded if the PipelineRun succeeded", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.release.MarkProcessing("")

			Expect(adapter.registerProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.IsProcessed()).To(BeTrue())
		})

		It("sets the Release as failed if the PipelineRun didn't succeed", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkFailed("", "")
			adapter.release.MarkProcessing("")

			Expect(adapter.registerProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.HasProcessingFinished()).To(BeTrue())
			Expect(adapter.release.IsProcessed()).To(BeFalse())
		})
	})

	When("calling syncResources", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("fails if there's no active ReleasePlanAdmission", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			Expect(adapter.syncResources()).NotTo(Succeed())
		})

		It("fails if there's no Snapshot", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
				},
				{
					ContextKey: loader.SnapshotContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			Expect(adapter.syncResources()).NotTo(Succeed())
		})

		It("should sync resources properly", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			Expect(adapter.syncResources()).To(Succeed())
		})
	})

	When("calling validateAuthor", func() {
		var adapter *adapter
		var conditionMsg string

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
			})
		})

		It("returns valid and no error if the release is already attributed", func() {
			adapter.release.Status.Attribution.Author = "user"
			result := adapter.validateAuthor()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return invalid and no error if the ReleasePlan is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result := adapter.validateAuthor()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		When("the release has the automated label", func() {
			BeforeEach(func() {
				adapter.release.Labels = map[string]string{
					metadata.AutomatedLabel: "true",
				}
			})

			It("returns invalid and an error if automated label is present but is not set in release status", func() {
				result := adapter.validateAuthor()
				Expect(result.Valid).To(BeFalse())
				Expect(result.Err).To(HaveOccurred())
				for i := range adapter.release.Status.Conditions {
					if adapter.release.Status.Conditions[i].Type == "Validated" {
						conditionMsg = adapter.release.Status.Conditions[i].Message
					}
				}
				Expect(conditionMsg).To(Equal("automated not set in status for automated release"))
			})

			It("returns invalid and an error if the ReleasePlan has no author", func() {
				adapter.release.Status.Automated = true
				result := adapter.validateAuthor()
				Expect(result.Valid).To(BeFalse())
				Expect(result.Err).NotTo(HaveOccurred())
				for i := range adapter.release.Status.Conditions {
					if adapter.release.Status.Conditions[i].Type == "Validated" {
						conditionMsg = adapter.release.Status.Conditions[i].Message
					}
				}
				Expect(conditionMsg).To(Equal("no author in the ReleasePlan found for automated release"))
			})

			It("properly sets the Attribution data in the release status", func() {
				adapter.release.Status.Automated = true
				releasePlan.Labels = map[string]string{
					metadata.AuthorLabel: "user",
				}
				result := adapter.validateAuthor()
				Expect(result.Valid).To(BeTrue())
				Expect(result.Err).NotTo(HaveOccurred())
				Expect(adapter.release.Status.Attribution.StandingAuthorization).To(BeTrue())
				Expect(adapter.release.Status.Attribution.Author).To(Equal("user"))
			})
		})

		It("returns invalid and an error if the Release has the automated label and no author", func() {
			adapter.release.Labels = map[string]string{
				metadata.AutomatedLabel: "false",
				metadata.AuthorLabel:    "",
			}
			result := adapter.validateAuthor()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			for i := range adapter.release.Status.Conditions {
				if adapter.release.Status.Conditions[i].Type == "Validated" {
					conditionMsg = adapter.release.Status.Conditions[i].Message
				}
			}
			Expect(conditionMsg).To(Equal("no author found for manual release"))
		})

		It("properly sets the Attribution author in the manual release status", func() {
			adapter.release.Labels = map[string]string{
				metadata.AuthorLabel: "user",
			}
			result := adapter.validateAuthor()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Attribution.StandingAuthorization).To(BeFalse())
			Expect(adapter.release.Status.Attribution.Author).To(Equal("user"))
		})
	})

	When("validateProcessingResources is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.release.MarkReleasing("")
		})

		It("should return valid and no error if all the resources are found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						EnterpriseContractConfigMap: enterpriseContractConfigMap,
						EnterpriseContractPolicy:    enterpriseContractPolicy,
						ReleasePlanAdmission:        releasePlanAdmission,
						Snapshot:                    snapshot,
					},
				},
			})

			result := adapter.validateProcessingResources()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return invalid and no error if any of the resources are not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result := adapter.validateProcessingResources()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("should return invalid and no error if the ReleasePlanAdmission is found to be disabled", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("auto-release label set to false"),
				},
			})

			result := adapter.validateProcessingResources()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("should return invalid and no error if multiple ReleasePlanAdmissions exist", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("multiple ReleasePlanAdmissions found"),
				},
			})

			result := adapter.validateProcessingResources()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("should return invalid and an error if some other type of error occurs", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Err:        fmt.Errorf("internal error"),
					Resource: &loader.ProcessingResources{
						ReleasePlanAdmission: releasePlanAdmission,
					},
				},
			})

			result := adapter.validateProcessingResources()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).To(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})
	})

	When("validatePipelineRef is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			releaseServiceConfig.Spec = v1alpha1.ReleaseServiceConfigSpec{}
			adapter.releaseServiceConfig = releaseServiceConfig
		})

		It("should return invalid and no error if the ReleasePlanAdmission is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result := adapter.validatePipelineRef()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("should return invalid and an error if some other type of error occurs when retrieving the ReleasePlanAdmission", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("internal error"),
					Resource:   releasePlanAdmission,
				},
			})

			result := adapter.validatePipelineRef()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).To(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("returns invalid and no error if debug is false and the PipelineRef uses a cluster resolver", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource: &v1alpha1.ReleasePlanAdmission{
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
							Environment:  environment.Name,
							PipelineRef: &tektonutils.PipelineRef{
								Resolver: "cluster",
								Params: []tektonutils.Param{
									{Name: "name", Value: "release-pipeline"},
									{Name: "namespace", Value: "default"},
									{Name: "kind", Value: "pipeline"},
								},
							},
							Policy: enterpriseContractPolicy.Name,
						},
					},
				},
			})
			adapter.releaseServiceConfig.Spec.Debug = false

			result := adapter.validatePipelineRef()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).To(BeNil())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("returns valid and no error if debug mode is enabled in the ReleaseServiceConfig", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			adapter.releaseServiceConfig.Spec.Debug = true

			result := adapter.validatePipelineRef()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).To(BeNil())
		})

		It("returns valid and no error if debug mode is disabled and the PipelineRef uses a bundle resolver", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			result := adapter.validatePipelineRef()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).To(BeNil())
		})
	})

	createReleaseAndAdapter = func() *adapter {
		release := &v1alpha1.Release{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "release-",
				Namespace:    "default",
			},
			Spec: v1alpha1.ReleaseSpec{
				Snapshot:    snapshot.Name,
				ReleasePlan: releasePlan.Name,
			},
		}
		Expect(k8sClient.Create(ctx, release)).To(Succeed())
		release.Kind = "Release"

		return newAdapter(ctx, k8sClient, release, loader.NewMockLoader(), &ctrl.Log)
	}

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
				Name:      "enterprise-contract-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"verify_ec_task_bundle": "test-bundle",
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
					{
						Name: "foo",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, enterpriseContractPolicy)).Should(Succeed())
		enterpriseContractPolicy.Kind = "EnterpriseContractPolicy"

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
				Environment:  environment.Name,
				PipelineRef: &tektonutils.PipelineRef{
					Resolver: "bundles",
					Params: []tektonutils.Param{
						{Name: "bundle", Value: "quay.io/some/bundle"},
						{Name: "name", Value: "release-pipeline"},
						{Name: "kind", Value: "pipeline"},
					},
					Timeout: "2h0m0s",
				},
				Policy: enterpriseContractPolicy.Name,
			},
		}
		Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())
		releasePlanAdmission.Kind = "ReleasePlanAdmission"

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
		snapshot.Kind = "Snapshot"

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
	}

	deleteResources = func() {
		Expect(k8sClient.Delete(ctx, application)).To(Succeed())
		Expect(k8sClient.Delete(ctx, component)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, enterpriseContractConfigMap)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, enterpriseContractPolicy)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, environment)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlan)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlanAdmission)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, releaseServiceConfig)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, snapshot)).To(Succeed())
		Expect(k8sClient.Delete(ctx, snapshotEnvironmentBinding)).To(Succeed())
	}

})
