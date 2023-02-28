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
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"github.com/redhat-appstudio/release-service/tekton"

	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"github.com/operator-framework/operator-lib/handler"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Release Adapter", Ordered, func() {
	var (
		createReleaseAndAdapter func() *Adapter
		createResources         func()
		deleteResources         func()

		application                *applicationapiv1alpha1.Application
		component                  *applicationapiv1alpha1.Component
		enterpriseContractPolicy   *ecapiv1alpha1.EnterpriseContractPolicy
		environment                *applicationapiv1alpha1.Environment
		releasePlan                *v1alpha1.ReleasePlan
		releasePlanAdmission       *v1alpha1.ReleasePlanAdmission
		releaseStrategy            *v1alpha1.ReleaseStrategy
		snapshot                   *applicationapiv1alpha1.Snapshot
		snapshotEnvironmentBinding *applicationapiv1alpha1.SnapshotEnvironmentBinding
	)

	AfterAll(func() {
		deleteResources()
	})

	BeforeAll(func() {
		createResources()
	})

	Context("When NewAdapter is called", func() {
		It("creates and return a new adapter", func() {
			Expect(reflect.TypeOf(NewAdapter(ctx, k8sClient, nil, loader.NewLoader(), ctrl.Log))).To(Equal(reflect.TypeOf(&Adapter{})))
		})
	})

	Context("When EnsureFinalizersAreCalled is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should do nothing if the release is not set to be deleted", func() {
			result, err := adapter.EnsureFinalizersAreCalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(!result.CancelRequest && !result.RequeueRequest).To(BeTrue())
		})

		It("should finalize the release if it's set to be deleted and it has a finalizer", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
				{
					ContextKey: loader.EnterpriseContractPolicyContextKey,
					Resource:   enterpriseContractPolicy,
				},
				{
					ContextKey: loader.SnapshotContextKey,
					Resource:   snapshot,
				},
			})
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Finalizers).To(HaveLen(1))

			result, err = adapter.EnsureReleasePipelineRunExists()
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
			Expect(pipelineRun).To(BeNil())
			Expect(err).NotTo(HaveOccurred())

			_, err = adapter.loader.GetRelease(adapter.ctx, adapter.client, adapter.release.Name, adapter.release.Namespace)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("When EnsureFinalizerIsAdded is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should add a finalizer to the release", func() {
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Finalizers).To(ContainElement(finalizerName))
		})

		It("shouldn't fail if the release already has the finalizer added", func() {
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Finalizers).To(ContainElement(finalizerName))

			result, err = adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When EnsureReleasePlanAdmissionEnabled is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should succeed if the ReleasePlanAdmission is enabled", func() {
			result, err := adapter.EnsureReleasePlanAdmissionEnabled()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should stop reconcile if the ReleasePlanAdmission is found to be disabled", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("auto-release label set to false"),
				},
			})
			result, err := adapter.EnsureReleasePlanAdmissionEnabled()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Conditions).To(HaveLen(1))
			Expect(adapter.release.Status.Conditions[0].Reason).To(Equal(string(v1alpha1.ReleaseReasonTargetDisabledError)))
		})

		It("should stop reconcile if multiple ReleasePlanAdmissions exist", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("multiple ReleasePlanAdmissions found"),
				},
			})
			result, err := adapter.EnsureReleasePlanAdmissionEnabled()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Conditions).To(HaveLen(1))
			Expect(adapter.release.Status.Conditions[0].Reason).To(Equal(string(v1alpha1.ReleaseReasonValidationError)))
		})
	})

	Context("When EnsureReleasePipelineRunExists is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the pipelineRun exists and the release has started", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource: &v1beta1.PipelineRun{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-run",
							Namespace: "default",
						},
					},
				},
			})

			adapter.release.MarkRunning()

			result, err := adapter.EnsureReleasePipelineRunExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should track the status data if the pipelineRun already exists", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource: &v1beta1.PipelineRun{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pipeline-run",
							Namespace: "default",
						},
					},
				},
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			result, err := adapter.EnsureReleasePipelineRunExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasStarted()).To(BeTrue())
		})

		It("should create a pipelineRun and track the status data if all the required resources are present", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
				{
					ContextKey: loader.EnterpriseContractPolicyContextKey,
					Resource:   enterpriseContractPolicy,
				},
				{
					ContextKey: loader.SnapshotContextKey,
					Resource:   snapshot,
				},
			})

			result, err := adapter.EnsureReleasePipelineRunExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasStarted()).To(BeTrue())

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should fail if the ReleasePlanAdmission is not found", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			result, err := adapter.EnsureReleasePipelineRunExists()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Conditions).To(HaveLen(1))
			Expect(adapter.release.Status.Conditions[0].Reason).To(Equal(string(v1alpha1.ReleaseReasonReleasePlanValidationError)))
		})

		It("should fail if the ReleaseStrategy is not found", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
				},
				{
					ContextKey: loader.ReleaseStrategyContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			result, err := adapter.EnsureReleasePipelineRunExists()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Conditions).To(HaveLen(1))
			Expect(adapter.release.Status.Conditions[0].Reason).To(Equal(string(v1alpha1.ReleaseReasonValidationError)))
		})

		It("should fail if the EnterpriseContractPolicy is not found", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
				},
				{
					ContextKey: loader.ReleaseStrategyContextKey,
					Resource:   releaseStrategy,
				},
				{
					ContextKey: loader.EnterpriseContractPolicyContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			result, err := adapter.EnsureReleasePipelineRunExists()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Conditions).To(HaveLen(1))
			Expect(adapter.release.Status.Conditions[0].Reason).To(Equal(string(v1alpha1.ReleaseReasonValidationError)))
		})

		It("should fail if the Snapshot is not found", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
				},
				{
					ContextKey: loader.ReleaseStrategyContextKey,
					Resource:   releaseStrategy,
				},
				{
					ContextKey: loader.EnterpriseContractPolicyContextKey,
				},
				{
					ContextKey: loader.SnapshotContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			result, err := adapter.EnsureReleasePipelineRunExists()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.Conditions).To(HaveLen(1))
			Expect(adapter.release.Status.Conditions[0].Reason).To(Equal(string(v1alpha1.ReleaseReasonValidationError)))
		})
	})

	Context("When EnsureReleasePipelineStatusIsTracked is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the release hasn't started or it's done", func() {
			result, err := adapter.EnsureReleasePipelineStatusIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should track the status if the pipelineRun exists", func() {
			adapter.release.MarkRunning()

			pipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource:   pipelineRun,
				},
			})

			result, err := adapter.EnsureReleasePipelineStatusIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsDone()).To(BeTrue())
		})

		It("should continue if the pipelineRun doesn't exist", func() {
			adapter.release.MarkRunning()

			result, err := adapter.EnsureReleasePipelineStatusIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When EnsureSnapshotEnvironmentBindingExists is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("skips the operation if the release has not succeeded yet", func() {
			result, err := adapter.EnsureSnapshotEnvironmentBindingExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if the release has already being deployed", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()
			adapter.release.MarkDeploying(metav1.ConditionFalse, "", "")
			adapter.release.MarkDeployed("", "")

			result, err := adapter.EnsureSnapshotEnvironmentBindingExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if no environment is set in the ReleasePlanAdmission", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()

			newReleasePlanAdmission := &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan-admission",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Application:     "app",
					Origin:          "default",
					ReleaseStrategy: "strategy",
				},
			}
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
			})

			result, err := adapter.EnsureSnapshotEnvironmentBindingExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.SnapshotEnvironmentBinding).To(BeEmpty())
		})

		It("fails when the ReleasePlanAdmission is not present", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()

			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			result, err := adapter.EnsureSnapshotEnvironmentBindingExists()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("updates the binding if one already exists", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()

			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
					Resource:   snapshotEnvironmentBinding,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingResourcesContextKey,
					Resource: &loader.SnapshotEnvironmentBindingResources{
						Application:           application,
						ApplicationComponents: []applicationapiv1alpha1.Component{*component},
						Environment:           environment,
						Snapshot:              snapshot,
					},
				},
			})

			result, err := adapter.EnsureSnapshotEnvironmentBindingExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.SnapshotEnvironmentBinding).NotTo(BeEmpty())

			binding, _ := adapter.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(adapter.ctx, adapter.client, adapter.release)
			Expect(binding).NotTo(BeNil())
			Expect(binding.Annotations).To(HaveLen(2))
			Expect(binding.Annotations[handler.NamespacedNameAnnotation]).To(
				Equal(adapter.release.Namespace + "/" + adapter.release.Name),
			)
			Expect(binding.Annotations[handler.TypeAnnotation]).To(Equal(adapter.release.Kind))
		})

		It("creates a binding and updates the release status", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()

			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingResourcesContextKey,
					Resource: &loader.SnapshotEnvironmentBindingResources{
						Application:           application,
						ApplicationComponents: []applicationapiv1alpha1.Component{*component},
						Environment:           environment,
						Snapshot:              snapshot,
					},
				},
			})

			result, err := adapter.EnsureSnapshotEnvironmentBindingExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.SnapshotEnvironmentBinding).NotTo(BeEmpty())

			// Restore the context to get the actual binding
			adapter.ctx = context.TODO()

			binding, err := adapter.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(adapter.ctx, adapter.client, adapter.release)
			Expect(binding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, binding)).Should(Succeed())
		})
	})

	Context("When EnsureSnapshotEnvironmentBindingIsTracked is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("skips the operation if the release has not succeeded yet", func() {
			result, err := adapter.EnsureSnapshotEnvironmentBindingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if the release doesn't have an associated binding", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()

			result, err := adapter.EnsureSnapshotEnvironmentBindingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if the release has been deployed already", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()
			adapter.release.Status.SnapshotEnvironmentBinding = snapshotEnvironmentBinding.Namespace + "/" + snapshotEnvironmentBinding.Name
			adapter.release.MarkDeploying(metav1.ConditionFalse, "", "")
			adapter.release.MarkDeployed("", "")

			result, err := adapter.EnsureSnapshotEnvironmentBindingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails if the binding is not found", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()
			adapter.release.Status.SnapshotEnvironmentBinding = "not/found"

			result, err := adapter.EnsureSnapshotEnvironmentBindingIsTracked()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("skips the operation if the binding isn't owned by the release", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()

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
			adapter.release.Status.SnapshotEnvironmentBinding = newSnapshotEnvironmentBinding.Namespace + "/" + newSnapshotEnvironmentBinding.Name

			result, err := adapter.EnsureSnapshotEnvironmentBindingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("tracks the binding if one is found", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
					Resource:   snapshotEnvironmentBinding,
				},
			})
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()
			adapter.release.Status.SnapshotEnvironmentBinding = snapshotEnvironmentBinding.Namespace + "/" + snapshotEnvironmentBinding.Name

			result, err := adapter.EnsureSnapshotEnvironmentBindingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When createReleasePipelineRun is called", func() {
		var (
			adapter     *Adapter
			pipelineRun *v1beta1.PipelineRun
		)

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)

			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()

			var err error
			pipelineRun, err = adapter.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a PipelineRun", func() {
			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&v1beta1.PipelineRun{})))
		})

		It("has owner annotations", func() {
			Expect(pipelineRun.GetAnnotations()[handler.NamespacedNameAnnotation]).To(ContainSubstring(adapter.release.Name))
			Expect(pipelineRun.GetAnnotations()[handler.TypeAnnotation]).To(ContainSubstring("Release"))
		})

		It("has release labels", func() {
			Expect(pipelineRun.GetLabels()[tekton.PipelinesTypeLabel]).To(Equal("release"))
			Expect(pipelineRun.GetLabels()[tekton.ReleaseNameLabel]).To(Equal(adapter.release.Name))
			Expect(pipelineRun.GetLabels()[tekton.ReleaseNamespaceLabel]).To(Equal(testNamespace))
		})

		It("references the pipeline specified in the ReleaseStrategy", func() {
			Expect(pipelineRun.Spec.PipelineRef.Name).To(Equal(releaseStrategy.Spec.Pipeline))
		})

		It("contains a parameter with the json representation of the EnterpriseContractPolicy", func() {
			jsonSpec, _ := json.Marshal(enterpriseContractPolicy.Spec)
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(jsonSpec)))))
		})

		It("contains a parameter with the json representation of the Snapshot", func() {
			jsonSpec, _ := json.Marshal(snapshot.Spec)
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(jsonSpec)))))
		})
	})

	Context("When registerGitOpsDeploymentStatus is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no binding", func() {
			Expect(adapter.registerGitOpsDeploymentStatus(nil)).To(Succeed())
			Expect(adapter.release.IsDeploying()).To(BeFalse())
		})

		It("does nothing if the binding doesn't have the expected condition", func() {
			Expect(adapter.registerGitOpsDeploymentStatus(snapshotEnvironmentBinding)).To(Succeed())
			Expect(adapter.release.IsDeploying()).To(BeFalse())
		})

		It("registers the deployment status when the binding has been deployed", func() {
			adapter.release.MarkDeploying(metav1.ConditionFalse, "", "")
			newSnapshotEnvironmentBinding := snapshotEnvironmentBinding.DeepCopy()
			newSnapshotEnvironmentBinding.Status.ComponentDeploymentConditions = []metav1.Condition{
				{
					Type:   applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
					Status: metav1.ConditionTrue,
					Reason: "Deployed",
				},
			}
			Expect(adapter.registerGitOpsDeploymentStatus(newSnapshotEnvironmentBinding)).To(Succeed())
			Expect(adapter.release.IsDeployed()).To(BeTrue())
		})
	})

	Context("When registerReleasePipelineRunStatus is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerReleasePipelineRunStatus(nil)).To(Succeed())
			Expect(adapter.release.Status.CompletionTime).To(BeNil())
		})

		It("does nothing if the PipelineRun is not done", func() {
			pipelineRun := &v1beta1.PipelineRun{}
			Expect(adapter.registerReleasePipelineRunStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.CompletionTime).To(BeNil())
		})

		It("sets the Release completion time", func() {
			pipelineRun := &v1beta1.PipelineRun{}
			pipelineRun.Status.MarkSucceeded("", "")
			Expect(adapter.registerReleasePipelineRunStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.CompletionTime).NotTo(BeNil())
		})

		It("sets the Release as succeeded if the PipelineRun succeeded", func() {
			pipelineRun := &v1beta1.PipelineRun{}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.release.MarkRunning()
			Expect(adapter.registerReleasePipelineRunStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.HasSucceeded()).To(BeTrue())
		})

		It("sets the Release as failed if the PipelineRun didn't succeed", func() {
			pipelineRun := &v1beta1.PipelineRun{}
			pipelineRun.Status.MarkFailed("", "")
			adapter.release.MarkRunning()
			Expect(adapter.registerReleasePipelineRunStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.HasSucceeded()).To(BeFalse())
		})
	})

	Context("When registerReleaseStatusData is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerReleaseStatusData(nil, nil)).To(Succeed())
			Expect(adapter.release.Status.ReleasePipelineRun).To(BeEmpty())
		})

		It("does nothing if there is no ReleaseStrategy", func() {
			pipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			Expect(adapter.registerReleaseStatusData(pipelineRun, nil)).To(Succeed())
			Expect(adapter.release.Status.ReleasePipelineRun).To(BeEmpty())
		})

		It("registers the Release data", func() {
			pipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			Expect(adapter.registerReleaseStatusData(pipelineRun, releaseStrategy)).To(Succeed())
			Expect(adapter.release.Status.ReleasePipelineRun).To(Equal(fmt.Sprintf("%s%c%s",
				pipelineRun.Namespace, types.Separator, pipelineRun.Name)))
			Expect(adapter.release.Status.ReleaseStrategy).To(Equal(fmt.Sprintf("%s%c%s",
				releaseStrategy.Namespace, types.Separator, releaseStrategy.Name)))
			Expect(adapter.release.Status.Target).To(Equal(pipelineRun.Namespace))
		})
	})

	Context("When createOrUpdateSnapshotEnvironmentBinding is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("fails when the required resources are not present", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.SnapshotEnvironmentBindingResourcesContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			_, err := adapter.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("creates a new binding owned by the release if the required resources are present", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingResourcesContextKey,
					Resource: &loader.SnapshotEnvironmentBindingResources{
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
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
					Resource:   snapshotEnvironmentBinding,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingResourcesContextKey,
					Resource: &loader.SnapshotEnvironmentBindingResources{
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

	Context("When finalizeRelease is called", func() {
		var adapter *Adapter

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
			pipelineRun, err := adapter.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease()).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
			Expect(pipelineRun).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When calling syncResources", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("fails if there's no active ReleasePlanAdmission", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			Expect(adapter.syncResources()).NotTo(Succeed())
		})

		It("fails if there's no Snapshot", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
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
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			Expect(adapter.syncResources()).To(Succeed())
		})
	})

	createReleaseAndAdapter = func() *Adapter {
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

		return NewAdapter(ctx, k8sClient, release, loader.NewMockLoader(), ctrl.Log)
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
		Expect(k8sClient.Delete(ctx, enterpriseContractPolicy)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, environment)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlan)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlanAdmission)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, releaseStrategy)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, snapshot)).To(Succeed())
		Expect(k8sClient.Delete(ctx, snapshotEnvironmentBinding)).To(Succeed())
	}

})
