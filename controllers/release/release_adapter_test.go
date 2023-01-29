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
	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/operator-framework/operator-lib/handler"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"github.com/redhat-appstudio/release-service/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Release Adapter", Ordered, func() {
	var (
		createAdapter   func() *Adapter
		createResources func()

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

	BeforeAll(func() {
		createResources()
	})

	Context("When EnsureFinalizersAreCalled is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createAdapter()
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
			adapter = createAdapter()
		})

		It("should a finalizer is added to the release", func() {
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
			adapter = createAdapter()
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
			adapter = createAdapter()
		})

		It("should continue if the pipelineRun exists and the release has started", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePipelineRun,
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
					ContextKey: loader.ReleasePipelineRun,
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
			adapter = createAdapter()
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
					ContextKey: loader.ReleasePipelineRun,
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
			adapter = createAdapter()
		})

		It("skips the operation if the release has not succeeded yet", func() {
			result, err := adapter.EnsureSnapshotEnvironmentBindingExists()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the operation if the release has already being deployed", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()
			adapter.release.MarkDeployed(metav1.ConditionTrue, "", "")

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

		It("does not create the binding if one already exists", func() {
			adapter.release.MarkRunning()
			adapter.release.MarkSucceeded()

			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
				{
					ContextKey: loader.SnapshotEnvironmentBindingContextKey,
					Resource: &applicationapiv1alpha1.SnapshotEnvironmentBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: "binding",
						},
					},
				},
			})

			result, err := adapter.EnsureSnapshotEnvironmentBindingExists()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Status.SnapshotEnvironmentBinding).To(BeEmpty())
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
			adapter = createAdapter()
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
			adapter.release.MarkDeployed(metav1.ConditionTrue, "", "")

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

		It("tracks the binding if one is found", func() {
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
			adapter = createAdapter()

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

	Context("When createSnapshotEnvironmentBinding is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createAdapter()
		})

		It("fails when the required resources are not present", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
				{
					ContextKey: loader.SnapshotEnvironmentBindingResourcesContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})

			_, err := adapter.createSnapshotEnvironmentBinding(releasePlanAdmission)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("creates a new binding owned by the release if the required resources are present", func() {
			adapter.ctx = loader.GetMockedContext(ctx, []loader.MockData{
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

			binding, err := adapter.createSnapshotEnvironmentBinding(releasePlanAdmission)
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
	})

	Context("When finalizeRelease is called", func() {
		var adapter *Adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createAdapter()
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
			adapter = createAdapter()
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

	createAdapter = func() *Adapter {
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

})
