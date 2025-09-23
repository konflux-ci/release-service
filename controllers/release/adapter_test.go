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
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"

	"github.com/konflux-ci/operator-toolkit/controller"
	toolkit "github.com/konflux-ci/operator-toolkit/loader"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/loader"
	"github.com/konflux-ci/release-service/metadata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/operator-framework/operator-lib/handler"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	ecapiv1alpha1 "github.com/conforma/crds/api/v1alpha1"
	applicationapiv1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
		Expect(os.Setenv("DEFAULT_RELEASE_WORKSPACE_NAME", "release-workspace")).To(Succeed())
		Expect(os.Setenv("DEFAULT_RELEASE_WORKSPACE_SIZE", "1Gi")).To(Succeed())

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
			adapter.releaseServiceConfig = releaseServiceConfig
		})

		It("should do nothing if the Release is not set to be deleted", func() {
			result, err := adapter.EnsureFinalizersAreCalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(!result.CancelRequest && !result.RequeueRequest).To(BeTrue())
		})

		It("should finalize the Release if it's set to be deleted and it has a finalizer", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
				{
					ContextKey: loader.SnapshotContextKey,
					Resource:   snapshot,
				},
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
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   roleBinding,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
			})
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Finalizers).To(HaveLen(1))

			result, err = adapter.EnsureTenantPipelineIsProcessed()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			result, err = adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())

			result, err = adapter.EnsureFinalPipelineIsProcessed()
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

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantPipelineType)
			Expect(pipelineRun).To(Or(BeNil(), HaveField("DeletionTimestamp", Not(BeNil()))))
			Expect(err).NotTo(HaveOccurred())

			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedPipelineType)
			Expect(pipelineRun).To(Or(BeNil(), HaveField("DeletionTimestamp", Not(BeNil()))))
			Expect(err).NotTo(HaveOccurred())

			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.FinalPipelineType)
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
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
			})
			adapter.release.MarkFinalPipelineProcessing()
			adapter.release.MarkFinalPipelineProcessed()
			adapter.release.MarkReleaseFailed("")
			result, err := adapter.EnsureReleaseIsCompleted()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasReleaseFinished()).To(BeTrue())
			Expect(adapter.release.IsReleased()).To(BeFalse())
		})

		It("should do nothing if the final processing has not completed", func() {
			result, err := adapter.EnsureReleaseIsCompleted()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasReleaseFinished()).To(BeFalse())
		})

		It("should complete the release if all the required phases have completed", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
			})
			adapter.release.MarkFinalPipelineProcessing()
			adapter.release.MarkFinalPipelineProcessed()
			result, err := adapter.EnsureReleaseIsCompleted()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasReleaseFinished()).To(BeTrue())
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

	When("EnsureFinalPipelineIsProcessed is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.releaseServiceConfig = releaseServiceConfig
		})

		It("should do nothing if the Release final pipeline is complete", func() {
			adapter.release.MarkFinalPipelineProcessing()
			adapter.release.MarkFinalPipelineProcessed()

			result, err := adapter.EnsureFinalPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsFinalPipelineProcessing()).To(BeFalse())
		})

		It("should do nothing if the Release managed pipeline processing has not yet completed", func() {
			adapter.release.MarkManagedPipelineProcessing()

			result, err := adapter.EnsureFinalPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsFinalPipelineProcessing()).To(BeFalse())
		})

		It("should requeue with error if fetching the Release final pipeline returns an error besides not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Err:        fmt.Errorf("some error"),
				},
			})
			adapter.release.MarkManagedPipelineProcessingSkipped()

			result, err := adapter.EnsureFinalPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.release.IsFinalPipelineProcessing()).To(BeFalse())
		})

		It("should continue and mark final processing as skipped if the ReleasePlan has no Final Pipeline set", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.FinalPipeline = nil
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						ReleasePlan: newReleasePlan,
					},
				},
			})
			adapter.release.MarkManagedPipelineProcessingSkipped()

			result, err := adapter.EnsureFinalPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsFinalPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsFinalPipelineProcessedSuccessfully()).To(BeTrue())
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
			})
			adapter.release.MarkFinalPipelineProcessing()

			result, err := adapter.EnsureFinalPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsFinalPipelineProcessing()).To(BeTrue())
		})

		It("should requeue the Release if any of the resources is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        fmt.Errorf("Not Found"),
					Resource:   nil,
				},
			})
			adapter.release.MarkManagedPipelineProcessingSkipped()

			result, err := adapter.EnsureFinalPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should create a pipelineRun and register the processing data if all the required resources are present", func() {
			releasePlan := &v1alpha1.ReleasePlan{}

			parameterizedPipeline := tektonutils.ParameterizedPipeline{}
			parameterizedPipeline.PipelineRef = tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: "my-url"},
					{Name: "revision", Value: "my-revision"},
					{Name: "pathInRepo", Value: "my-path"},
				},
			}
			parameterizedPipeline.Params = []tektonutils.Param{
				{Name: "parameter1", Value: "value1"},
				{Name: "parameter2", Value: "value2"},
			}
			parameterizedPipeline.Timeouts = tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
			}

			releasePlan = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application:            application.Name,
					FinalPipeline:          &parameterizedPipeline,
					ReleaseGracePeriodDays: 6,
				},
			}
			releasePlan.Kind = "ReleasePlan"

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
				{
					ContextKey: loader.SnapshotContextKey,
					Resource:   snapshot,
				},
			})

			adapter.release.MarkManagedPipelineProcessingSkipped()

			result, err := adapter.EnsureFinalPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsFinalPipelineProcessing()).To(BeTrue())

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.FinalPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

	})

	When("EnsureManagedCollectorsPipelineIsProcessed is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.releaseServiceConfig = releaseServiceConfig
			adapter.release.MarkReleasing("")
		})

		It("should do nothing if the Release managed collectors pipeline is complete", func() {
			adapter.release.MarkManagedCollectorsPipelineProcessing()
			adapter.release.MarkManagedCollectorsPipelineProcessed()

			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should do nothing if the Release tenant collectors pipeline processing has not yet completed", func() {
			adapter.release.MarkTenantCollectorsPipelineProcessing()

			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsFinalPipelineProcessing()).To(BeFalse())
		})

		It("should mark the pipeline as Skipped if the release has failed", func() {
			adapter.release.MarkTenantCollectorsPipelineProcessing()
			adapter.release.MarkTenantCollectorsPipelineProcessingFailed("")
			adapter.release.MarkReleaseFailed("")

			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsManagedCollectorsPipelineSkipped()).To(BeTrue())
		})

		It("should requeue with error if fetching the Release managed collectors pipeline returns an error besides not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Err:        fmt.Errorf("some error"),
				},
			})

			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should continue and mark managed collectors processing as skipped if the ReleasePlanAdmission has no collectors defined", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Target = releasePlanAdmission.Namespace
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsManagedCollectorsPipelineSkipped()).To(BeTrue())
		})

		It("should register the processing data if the PipelineRun already exists", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

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
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
			})

			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()
			adapter.release.MarkManagedCollectorsPipelineProcessing()
			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedCollectorsPipelineProcessing()).To(BeTrue())
		})

		It("should requeue the Release if any of the resources is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
					Resource:   nil,
				},
			})

			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should create RoleBindings if all the required resources are present and none exists in the Release Status", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Collectors = &v1alpha1.Collectors{
				Secrets:            []string{"bar", "foo"},
				ServiceAccountName: "foo",
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
			})
			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()

			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.TenantRoleBinding).To(BeEmpty())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.ManagedRoleBinding).To(BeEmpty())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.SecretRoleBinding).To(BeEmpty())
			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// Reset MockedContext so that the RoleBinding that was just created can be fetched
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
			})

			tenantRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing, "tenant")
			Expect(tenantRoleBinding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, tenantRoleBinding)).To(Succeed())

			managedRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing, "managed")
			Expect(managedRoleBinding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, managedRoleBinding)).To(Succeed())

			secretRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing, "secret")
			Expect(secretRoleBinding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, secretRoleBinding)).To(Succeed())

			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedCollectorsPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should not create RoleBindings if the Collectors has no ServiceAccount set", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Collectors = &v1alpha1.Collectors{
				Secrets: []string{"bar", "foo"},
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
			})
			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()

			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.TenantRoleBinding).To(BeEmpty())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.ManagedRoleBinding).To(BeEmpty())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.SecretRoleBinding).To(BeEmpty())
			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// Reset MockedContext so that the RoleBinding that was just created can be fetched
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
			})

			tenantRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing, "tenant")
			Expect(tenantRoleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())

			managedRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing, "managed")
			Expect(managedRoleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())

			secretRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing, "secret")
			Expect(secretRoleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())

			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedCollectorsPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should not create a secrets roleBinding if the Collectors has no secrets set", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Collectors = &v1alpha1.Collectors{
				ServiceAccountName: "foo",
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
			})
			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()

			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.SecretRoleBinding).To(BeEmpty())
			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// Reset MockedContext so that the RoleBinding that was just created can be fetched
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
			})

			secretRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing, "secret")
			Expect(secretRoleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())

			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedCollectorsPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should create a pipelineRun and register the processing data if all the required resources are present", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   newReleasePlanAdmission,
				},
			})

			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureManagedCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedCollectorsPipelineProcessing()).To(BeTrue())

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedCollectorsPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})
	})

	When("EnsureManagedPipelineIsProcessed is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.releaseServiceConfig = releaseServiceConfig
			adapter.release.MarkReleasing("")
		})

		It("should do nothing if the Release managed pipeline is already complete", func() {
			adapter.release.MarkManagedPipelineProcessing()
			adapter.release.MarkManagedPipelineProcessed()
			adapter.release.MarkTenantPipelineProcessingSkipped()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeFalse())
		})

		It("should do nothing if the Release tenant pipeline processing has not yet completed", func() {
			adapter.release.MarkTenantPipelineProcessing()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeFalse())
		})

		It("should mark the Managed Pipeline Processing as Skipped if the release has failed", func() {
			adapter.release.MarkTenantPipelineProcessing()
			adapter.release.MarkTenantPipelineProcessingFailed("")
			adapter.release.MarkReleaseFailed("")

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsManagedPipelineSkipped()).To(BeTrue())
		})

		It("should requeue with error if fetching the Release managed pipeline returns an error besides not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Err:        fmt.Errorf("some error"),
				},
			})
			adapter.release.MarkTenantPipelineProcessingSkipped()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeFalse())
		})

		It("should mark the Managed Pipeline Processing as Skipped if the ReleasePlanAdmission isn't found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Err:        fmt.Errorf("no ReleasePlanAdmissions can be found"),
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
			})
			adapter.release.MarkTenantPipelineProcessingSkipped()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsManagedPipelineSkipped()).To(BeTrue())
		})

		It("should mark the Managed Pipeline Processing as Skipped if the ReleasePlanAdmission has no pipeline", func() {
			newReleasePlanAdmission := &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan-admission",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Applications: []string{application.Name},
					Origin:       "default",
					Policy:       enterpriseContractPolicy.Name,
				},
			}
			newReleasePlanAdmission.Kind = "ReleasePlanAdmission"
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						EnterpriseContractConfigMap: enterpriseContractConfigMap,
						EnterpriseContractPolicy:    enterpriseContractPolicy,
						ReleasePlan:                 releasePlan,
						ReleasePlanAdmission:        newReleasePlanAdmission,
						Snapshot:                    snapshot,
					},
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
			})
			adapter.release.MarkTenantPipelineProcessingSkipped()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsManagedPipelineSkipped()).To(BeTrue())
		})

		It("should continue if the PipelineRun exists and the release managed pipeline processing has started", func() {
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
					ContextKey: loader.RoleBindingContextKey,
					Resource:   roleBinding,
				},
			})
			adapter.release.MarkManagedPipelineProcessing()
			adapter.release.MarkTenantPipelineProcessingSkipped()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
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
					ContextKey: loader.RoleBindingContextKey,
					Resource:   roleBinding,
				},
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						EnterpriseContractConfigMap: enterpriseContractConfigMap,
						EnterpriseContractPolicy:    enterpriseContractPolicy,
						ReleasePlanAdmission:        releasePlanAdmission,
						ReleasePlan:                 releasePlan,
						Snapshot:                    snapshot,
					},
				},
			})
			adapter.release.MarkTenantPipelineProcessingSkipped()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeTrue())
		})

		It("should requeue the Release if any of the resources is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Err:        fmt.Errorf("not found"),
				},
			})
			adapter.release.MarkTenantPipelineProcessingSkipped()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should create a RoleBinding if all the required resources are present and none exists in the Release Status", func() {
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
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
			})
			adapter.release.MarkTenantPipelineProcessingSkipped()

			Expect(adapter.release.Status.ManagedProcessing.RoleBindings.TenantRoleBinding).To(BeEmpty())
			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// Reset MockedContext so that the RoleBinding that was just created can be fetched
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

			tenantRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.ManagedProcessing, "tenant")
			Expect(tenantRoleBinding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, tenantRoleBinding)).To(Succeed())

			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should not create a RoleBinding if the ReleasePlanAdmission has no ServiceAccount set", func() {
			newReleasePlanAdmission := &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan-admission",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Applications: []string{application.Name},
					Origin:       "default",
					Pipeline: &tektonutils.Pipeline{
						PipelineRef: tektonutils.PipelineRef{
							Resolver: "git",
							Params: []tektonutils.Param{
								{Name: "url", Value: "my-url"},
								{Name: "revision", Value: "my-revision"},
								{Name: "pathInRepo", Value: "my-path"},
							},
						},
					},
					Policy: enterpriseContractPolicy.Name,
				},
			}
			newReleasePlanAdmission.Kind = "ReleasePlanAdmission"

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						EnterpriseContractConfigMap: enterpriseContractConfigMap,
						EnterpriseContractPolicy:    enterpriseContractPolicy,
						ReleasePlan:                 releasePlan,
						ReleasePlanAdmission:        newReleasePlanAdmission,
						Snapshot:                    snapshot,
					},
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
			})
			adapter.release.MarkTenantPipelineProcessingSkipped()

			Expect(adapter.release.Status.ManagedProcessing.RoleBindings.TenantRoleBinding).To(BeEmpty())
			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// Reset MockedContext so that the RoleBinding can be fetched if it exists
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						EnterpriseContractConfigMap: enterpriseContractConfigMap,
						EnterpriseContractPolicy:    enterpriseContractPolicy,
						ReleasePlan:                 releasePlan,
						ReleasePlanAdmission:        newReleasePlanAdmission,
						Snapshot:                    snapshot,
					},
				},
			})

			tenantRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.ManagedProcessing, "tenant")
			Expect(tenantRoleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())

			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
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
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   roleBinding,
				},
			})
			adapter.release.MarkTenantPipelineProcessingSkipped()

			result, err := adapter.EnsureManagedPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeTrue())

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})
	})

	When("EnsureTenantCollectorsPipelineIsProcessed is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.releaseServiceConfig = releaseServiceConfig
		})

		It("should do nothing if the Release tenant collectors pipeline is complete", func() {
			adapter.release.MarkTenantCollectorsPipelineProcessing()
			adapter.release.MarkTenantCollectorsPipelineProcessed()

			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should requeue with error if fetching the Release tenant collectors pipeline returns an error besides not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Err:        fmt.Errorf("some error"),
				},
			})

			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should continue and mark tenant collectors processing as skipped if the ReleasePlan has no collectors defined", func() {
			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsTenantCollectorsPipelineSkipped()).To(BeTrue())
		})

		It("should register the processing data if the PipelineRun already exists", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

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
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantCollectorsPipelineProcessing()).To(BeTrue())
		})

		It("should requeue the Release if any of the resources is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
					Resource:   nil,
				},
			})

			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should create RoleBindings if all the required resources are present and none exists in the Release Status", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Collectors = &v1alpha1.Collectors{
				Secrets:            []string{"bar", "foo"},
				ServiceAccountName: "foo",
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings.SecretRoleBinding).To(BeEmpty())
			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// Reset MockedContext so that the RoleBinding that was just created can be fetched
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
			})

			tenantRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing, "secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, tenantRoleBinding)).To(Succeed())

			secretRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing, "tenant")
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, secretRoleBinding)).To(Succeed())

			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantCollectorsPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should not create RoleBindings if the Collectors has no ServiceAccount set", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Collectors = &v1alpha1.Collectors{
				Secrets: []string{"bar", "foo"},
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings.TenantRoleBinding).To(BeEmpty())
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings.SecretRoleBinding).To(BeEmpty())
			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// Reset MockedContext so that the RoleBinding that was just created can be fetched
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
			})

			tenantRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing, "secret")
			Expect(tenantRoleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())

			secretRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing, "tenant")
			Expect(secretRoleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())

			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantCollectorsPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should not create a secrets roleBinding if the Collectors has no secrets set", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Collectors = &v1alpha1.Collectors{
				ServiceAccountName: "foo",
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   nil,
				},
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings.SecretRoleBinding).To(BeEmpty())
			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			// Reset MockedContext so that the RoleBinding that was just created can be fetched
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
			})

			secretRoleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatusPipelineInfo(adapter.ctx, adapter.client, &adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing, "secret")
			Expect(secretRoleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())

			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantCollectorsPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should create a pipelineRun and register the processing data if all the required resources are present", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			result, err := adapter.EnsureTenantCollectorsPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantCollectorsPipelineProcessing()).To(BeTrue())

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantCollectorsPipelineType)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})
	})

	When("EnsureTenantPipelineIsProcessed is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.releaseServiceConfig = releaseServiceConfig
			adapter.release.MarkReleasing("")
		})

		It("should do nothing if the Release tenant pipeline is complete", func() {
			adapter.release.MarkTenantPipelineProcessing()
			adapter.release.MarkTenantPipelineProcessed()

			result, err := adapter.EnsureTenantPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantPipelineProcessing()).To(BeFalse())
		})

		It("should mark the Release tenant pipeline as Skipped if the release has failed", func() {
			adapter.release.MarkManagedCollectorsPipelineProcessing()
			adapter.release.MarkManagedCollectorsPipelineProcessingFailed("")
			adapter.release.MarkReleaseFailed("")

			result, err := adapter.EnsureTenantPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsTenantPipelineSkipped()).To(BeTrue())
		})

		It("should do nothing if the Release managed collectors pipeline processing has not yet completed", func() {
			adapter.release.MarkManagedCollectorsPipelineProcessing()

			result, err := adapter.EnsureTenantPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsFinalPipelineProcessing()).To(BeFalse())
		})

		It("should requeue with error if fetching the Release managed pipeline returns an error besides not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Err:        fmt.Errorf("some error"),
				},
			})

			adapter.release.MarkManagedCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureTenantPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.release.IsTenantPipelineProcessing()).To(BeFalse())
		})

		It("should continue and mark tenant processing as skipped if the ReleasePlan has no Tenant Pipeline set", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.TenantPipeline = nil
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ProcessingResourcesContextKey,
					Resource: &loader.ProcessingResources{
						ReleasePlan: newReleasePlan,
					},
				},
			})

			adapter.release.MarkManagedCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureTenantPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantPipelineProcessing()).To(BeFalse())
			Expect(adapter.release.IsTenantPipelineSkipped()).To(BeTrue())
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
			})
			adapter.release.MarkManagedCollectorsPipelineProcessingSkipped()
			adapter.release.MarkTenantPipelineProcessing()

			result, err := adapter.EnsureTenantPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantPipelineProcessing()).To(BeTrue())
		})

		It("should requeue the Release if any of the resources is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        fmt.Errorf("Not Found"),
					Resource:   nil,
				},
			})

			adapter.release.MarkManagedCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureTenantPipelineIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should create a pipelineRun and register the processing data if all the required resources are present", func() {
			releasePlan := &v1alpha1.ReleasePlan{}

			parameterizedPipeline := tektonutils.ParameterizedPipeline{}
			parameterizedPipeline.PipelineRef = tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: "my-url"},
					{Name: "revision", Value: "my-revision"},
					{Name: "pathInRepo", Value: "my-path"},
				},
			}
			parameterizedPipeline.Params = []tektonutils.Param{
				{Name: "parameter1", Value: "value1"},
				{Name: "parameter2", Value: "value2"},
			}
			parameterizedPipeline.Timeouts = tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
			}

			releasePlan = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application:            application.Name,
					TenantPipeline:         &parameterizedPipeline,
					FinalPipeline:          &parameterizedPipeline,
					ReleaseGracePeriodDays: 6,
				},
			}
			releasePlan.Kind = "ReleasePlan"

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
				{
					ContextKey: loader.SnapshotContextKey,
					Resource:   snapshot,
				},
			})

			adapter.release.MarkManagedCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureTenantPipelineIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsTenantPipelineProcessing()).To(BeTrue())

			pipelineRun, err := adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantPipelineType)
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

	When("EnsureTenantPielineProcessingIsTracked is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the Release tenant pipeline processing has not started", func() {
			result, err := adapter.EnsureTenantPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should continue if the Release tenant pipeline processing has finished", func() {
			adapter.release.MarkTenantPipelineProcessing()
			adapter.release.MarkTenantPipelineProcessed()

			result, err := adapter.EnsureTenantPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should track the status if the PipelineRun exists", func() {
			adapter.release.MarkTenantPipelineProcessing()

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

			result, err := adapter.EnsureTenantPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasTenantPipelineProcessingFinished()).To(BeTrue())
		})

		It("should continue if the PipelineRun doesn't exist", func() {
			adapter.release.MarkTenantPipelineProcessing()

			result, err := adapter.EnsureTenantPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("EnsureManagedPipelineProcessingIsTracked is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the Release managed pipeline processing has not started", func() {
			result, err := adapter.EnsureManagedPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should continue if the Release managed pipeline processing has finished", func() {
			adapter.release.MarkManagedPipelineProcessing()
			adapter.release.MarkManagedPipelineProcessed()

			result, err := adapter.EnsureManagedPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should track the status if the PipelineRun exists", func() {
			adapter.release.MarkManagedPipelineProcessing()

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
				{
					ContextKey: loader.RoleBindingContextKey,
				},
			})

			result, err := adapter.EnsureManagedPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasManagedPipelineProcessingFinished()).To(BeTrue())
		})

		It("should continue if the PipelineRun doesn't exist", func() {
			adapter.release.MarkManagedPipelineProcessing()

			result, err := adapter.EnsureManagedPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("EnsureFinalPipelineProcessingIsTracked is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the Release final pipeline processing has not started", func() {
			result, err := adapter.EnsureFinalPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should continue if the Release final pipeline processing has finished", func() {
			adapter.release.MarkFinalPipelineProcessing()
			adapter.release.MarkFinalPipelineProcessed()

			result, err := adapter.EnsureFinalPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should track the status if the PipelineRun exists", func() {
			adapter.release.MarkFinalPipelineProcessing()

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

			result, err := adapter.EnsureFinalPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasFinalPipelineProcessingFinished()).To(BeTrue())
		})

		It("should continue if the PipelineRun doesn't exist", func() {
			adapter.release.MarkFinalPipelineProcessing()

			result, err := adapter.EnsureFinalPipelineProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

	})

	Context("When EnsureApplicationMetadataIsSet is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should do nothing if the Release already has an owner reference", func() {
			adapter.release.OwnerReferences = []metav1.OwnerReference{
				{Kind: "Application", Name: "foo"},
			}

			Expect(adapter.release.OwnerReferences).To(HaveLen(1))
			result, err := adapter.EnsureApplicationMetadataIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.OwnerReferences).To(HaveLen(1))
		})

		It("should fail if the ReleasePlan does not exist", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result, err := adapter.EnsureApplicationMetadataIsSet()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.release.OwnerReferences).To(HaveLen(0))
		})

		It("should fail if the Snapshot does not exist", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.SnapshotContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result, err := adapter.EnsureApplicationMetadataIsSet()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(adapter.release.OwnerReferences).To(HaveLen(0))
		})

		It("should fail if the Application does not exist", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ApplicationContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result, err := adapter.EnsureApplicationMetadataIsSet()
			Expect(!result.RequeueRequest && result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
			Expect(adapter.release.OwnerReferences).To(HaveLen(0))
		})

		It("should set the owner reference", func() {
			Expect(adapter.release.OwnerReferences).To(HaveLen(0))
			result, err := adapter.EnsureApplicationMetadataIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			boolTrue := true
			expectedOwnerReference := metav1.OwnerReference{
				Kind:               "Application",
				APIVersion:         "appstudio.redhat.com/v1alpha1",
				UID:                application.UID,
				Name:               application.Name,
				Controller:         &boolTrue,
				BlockOwnerDeletion: &boolTrue,
			}
			Expect(adapter.release.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
		})

		It("should add the annotations and labels that have the proper prefix from the snapshot", func() {
			metadataSnapshot := &applicationapiv1alpha1.Snapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "metadata-snapshot",
					Namespace: "default",
					Labels: map[string]string{
						metadata.PipelinesAsCodePrefix + "/foo": "value",
						metadata.RhtapDomain + "/bar":           "value2",
						"something-else":                        "value3",
					},
					Annotations: map[string]string{
						metadata.PipelinesAsCodePrefix + "/test": "value",
						metadata.RhtapDomain + "/baz":            "value2",
						"something-else-else":                    "value3",
					},
				},
				Spec: applicationapiv1alpha1.SnapshotSpec{
					Application: application.Name,
				},
			}
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.SnapshotContextKey,
					Resource:   metadataSnapshot,
				},
			})
			Expect(adapter.release.Labels).To(HaveLen(0))
			Expect(adapter.release.Annotations).To(HaveLen(0))

			result, err := adapter.EnsureApplicationMetadataIsSet()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.Labels).To(HaveLen(2))
			Expect(adapter.release.Annotations).To(HaveLen(2))
		})
	})

	When("EnsureReleaseExpirationTimeIsAdded is called", func() {
		var adapter *adapter
		var newReleasePlan *v1alpha1.ReleasePlan

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			newReleasePlan = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application:            application.Name,
					Target:                 "default",
					ReleaseGracePeriodDays: 6,
				},
			}
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
			})
		})

		It("should set the ExpirationTime with the value of Release's GracePeriodDays and then continue", func() {
			expireDays := time.Duration(3)
			adapter.release.Spec.GracePeriodDays = 3
			creationTime := adapter.release.CreationTimestamp
			expectedExpirationTime := &metav1.Time{Time: creationTime.Add(time.Hour * 24 * expireDays)}

			result, err := adapter.EnsureReleaseExpirationTimeIsAdded()
			Expect(adapter.release.Status.ExpirationTime).To(Equal(expectedExpirationTime))
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should set the ExpirationTime with the value of ReleasePlan's ReleaseGracePeriodDays and then continue", func() {
			expireDays := time.Duration(newReleasePlan.Spec.ReleaseGracePeriodDays)
			creationTime := adapter.release.CreationTimestamp
			expectedExpirationTime := &metav1.Time{Time: creationTime.Add(time.Hour * 24 * expireDays)}

			result, err := adapter.EnsureReleaseExpirationTimeIsAdded()
			Expect(adapter.release.Status.ExpirationTime).To(Equal(expectedExpirationTime))
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not change the ExpirationTime if it is already set", func() {
			expireDays := time.Duration(5)
			creationTime := adapter.release.CreationTimestamp
			expectedExpirationTime := &metav1.Time{Time: creationTime.Add(time.Hour * 24 * expireDays)}

			adapter.release.Status.ExpirationTime = expectedExpirationTime
			result, err := adapter.EnsureReleaseExpirationTimeIsAdded()
			Expect(adapter.release.Status.ExpirationTime).To(Equal(expectedExpirationTime))
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("EnsureCollectorsProcessingResourcesAreCleanedUp is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the Release Tenant Collectors processing has not finished", func() {
			result, err := adapter.EnsureCollectorsProcessingResourcesAreCleanedUp()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should continue if the Release Managed Collectors processing has not finished", func() {
			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()
			result, err := adapter.EnsureCollectorsProcessingResourcesAreCleanedUp()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should cleanup Collectors Tenant Role and RoleBindings after processing has finished", func() {
			tenantRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-tenant-role-binding",
					Namespace: "default",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "ClusterRole",
					Name:     "clusterrole",
				},
			}
			secretRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-secret-role-binding",
					Namespace: "default",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "Role",
					Name:     "foo-role",
				},
			}
			secretRole := &rbac.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-role",
					Namespace: "default",
				},
			}
			Expect(adapter.client.Create(adapter.ctx, tenantRoleBinding)).To(Succeed())
			Expect(adapter.client.Create(adapter.ctx, secretRole)).To(Succeed())
			Expect(adapter.client.Create(adapter.ctx, secretRoleBinding)).To(Succeed())

			adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings.TenantRoleBinding =
				fmt.Sprintf("%s%c%s", tenantRoleBinding.Namespace, types.Separator, tenantRoleBinding.Name)
			adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings.SecretRoleBinding =
				fmt.Sprintf("%s%c%s", secretRoleBinding.Namespace, types.Separator, secretRoleBinding.Name)

			adapter.release.MarkTenantCollectorsPipelineProcessing()
			adapter.release.MarkTenantCollectorsPipelineProcessed()
			adapter.release.MarkManagedCollectorsPipelineProcessingSkipped()

			res, err := adapter.EnsureCollectorsProcessingResourcesAreCleanedUp()
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueRequest).To(BeFalse())
			Expect(res.CancelRequest).To(BeFalse())

			checkTenantRoleBinding := &rbac.RoleBinding{}
			err = toolkit.GetObject(tenantRoleBinding.Name, tenantRoleBinding.Namespace, adapter.client, adapter.ctx, tenantRoleBinding)
			Expect(checkTenantRoleBinding).To(Equal(&rbac.RoleBinding{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())

			checkSecretRole := &rbac.Role{}
			err = toolkit.GetObject(secretRole.Name, secretRole.Namespace, adapter.client, adapter.ctx, checkSecretRole)
			Expect(checkSecretRole).To(Equal(&rbac.Role{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())

			checkSecretRoleBinding := &rbac.RoleBinding{}
			err = toolkit.GetObject(secretRoleBinding.Name, secretRoleBinding.Namespace, adapter.client, adapter.ctx, checkSecretRoleBinding)
			Expect(checkSecretRoleBinding).To(Equal(&rbac.RoleBinding{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should cleanup Collectors Managed Role and RoleBindings after processing has finished", func() {
			tenantRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-tenant-role-binding",
					Namespace: "default",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "ClusterRole",
					Name:     "clusterrole",
				},
			}
			managedRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-managed-role-binding",
					Namespace: "default",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "ClusterRole",
					Name:     "clusterrole",
				},
			}
			secretRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-secret-role-binding",
					Namespace: "default",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "Role",
					Name:     "foo-role",
				},
			}
			secretRole := &rbac.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-role",
					Namespace: "default",
				},
			}
			Expect(adapter.client.Create(adapter.ctx, tenantRoleBinding)).To(Succeed())
			Expect(adapter.client.Create(adapter.ctx, managedRoleBinding)).To(Succeed())
			Expect(adapter.client.Create(adapter.ctx, secretRole)).To(Succeed())
			Expect(adapter.client.Create(adapter.ctx, secretRoleBinding)).To(Succeed())

			adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.TenantRoleBinding =
				fmt.Sprintf("%s%c%s", tenantRoleBinding.Namespace, types.Separator, tenantRoleBinding.Name)
			adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.ManagedRoleBinding =
				fmt.Sprintf("%s%c%s", managedRoleBinding.Namespace, types.Separator, managedRoleBinding.Name)
			adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.SecretRoleBinding =
				fmt.Sprintf("%s%c%s", secretRoleBinding.Namespace, types.Separator, secretRoleBinding.Name)

			adapter.release.MarkTenantCollectorsPipelineProcessingSkipped()
			adapter.release.MarkManagedCollectorsPipelineProcessing()
			adapter.release.MarkManagedCollectorsPipelineProcessed()

			res, err := adapter.EnsureCollectorsProcessingResourcesAreCleanedUp()
			Expect(err).NotTo(HaveOccurred())
			Expect(res.RequeueRequest).To(BeFalse())
			Expect(res.CancelRequest).To(BeFalse())

			checkTenantRoleBinding := &rbac.RoleBinding{}
			err = toolkit.GetObject(tenantRoleBinding.Name, tenantRoleBinding.Namespace, adapter.client, adapter.ctx, checkTenantRoleBinding)
			Expect(checkTenantRoleBinding).To(Equal(&rbac.RoleBinding{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())

			checkManagedRoleBinding := &rbac.RoleBinding{}
			err = toolkit.GetObject(managedRoleBinding.Name, managedRoleBinding.Namespace, adapter.client, adapter.ctx, checkManagedRoleBinding)
			Expect(checkManagedRoleBinding).To(Equal(&rbac.RoleBinding{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())

			checkSecretRole := &rbac.Role{}
			err = toolkit.GetObject(secretRole.Name, secretRole.Namespace, adapter.client, adapter.ctx, checkSecretRole)
			Expect(checkSecretRole).To(Equal(&rbac.Role{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())

			checkSecretRoleBinding := &rbac.RoleBinding{}
			err = toolkit.GetObject(secretRoleBinding.Name, secretRoleBinding.Namespace, adapter.client, adapter.ctx, checkSecretRoleBinding)
			Expect(checkSecretRoleBinding).To(Equal(&rbac.RoleBinding{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	When("EnsureReleaseProcessingResourcesAreCleanedUp is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the Release tenant processing has not finished", func() {
			adapter.release.MarkManagedPipelineProcessingSkipped()
			result, err := adapter.EnsureReleaseProcessingResourcesAreCleanedUp()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should continue if the Release managed processing has not finished", func() {
			adapter.release.MarkTenantPipelineProcessingSkipped()
			result, err := adapter.EnsureReleaseProcessingResourcesAreCleanedUp()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should continue if the Release final processing has not finished", func() {
			adapter.release.MarkFinalPipelineProcessingSkipped()
			result, err := adapter.EnsureReleaseProcessingResourcesAreCleanedUp()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call finalizeRelease with false if all release processing is complete", func() {
			adapter.releaseServiceConfig = releaseServiceConfig
			resources := &loader.ProcessingResources{
				ReleasePlan:                 releasePlan,
				ReleasePlanAdmission:        releasePlanAdmission,
				EnterpriseContractConfigMap: enterpriseContractConfigMap,
				EnterpriseContractPolicy:    enterpriseContractPolicy,
				Snapshot:                    snapshot,
			}
			parameterizedPipeline := tektonutils.ParameterizedPipeline{}
			parameterizedPipeline.PipelineRef = tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: "my-url"},
					{Name: "revision", Value: "my-revision"},
					{Name: "pathInRepo", Value: "my-path"},
				},
			}
			parameterizedPipeline.Params = []tektonutils.Param{
				{Name: "parameter1", Value: "value1"},
				{Name: "parameter2", Value: "value2"},
			}
			parameterizedPipeline.Timeouts = tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
			}

			newReleasePlan := &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application:            application.Name,
					TenantPipeline:         &parameterizedPipeline,
					FinalPipeline:          &parameterizedPipeline,
					ReleaseGracePeriodDays: 6,
				},
			}
			newReleasePlan.Kind = "ReleasePlan"

			// Create tenant, managed, final and pipelineRuns
			pipelineRun, err := adapter.createTenantPipelineRun(newReleasePlan, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			pipelineRun, err = adapter.createFinalPipelineRun(newReleasePlan, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			adapter.release.MarkTenantPipelineProcessing()
			adapter.release.MarkTenantPipelineProcessed()
			adapter.release.MarkManagedPipelineProcessing()
			adapter.release.MarkManagedPipelineProcessed()
			adapter.release.MarkFinalPipelineProcessing()
			adapter.release.MarkFinalPipelineProcessed()

			// Ensure all pipelineRuns have finalizers removed
			result, err := adapter.EnsureReleaseProcessingResourcesAreCleanedUp()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).NotTo(BeNil())
			Expect(pipelineRun.Finalizers).To(HaveLen(0))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())

			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).NotTo(BeNil())
			Expect(pipelineRun.Finalizers).To(HaveLen(0))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())

			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.FinalPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).NotTo(BeNil())
			Expect(pipelineRun.Finalizers).To(HaveLen(0))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})
	})

	When("cleanupProcessingResources is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("removes the roleBinding if present", func() {
			newRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-role-binding",
					Namespace: "default",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "ClusterRole",
					Name:     "clusterrole",
				},
			}
			// The resource needs to be created as it will get patched
			Expect(adapter.client.Create(adapter.ctx, newRoleBinding)).To(Succeed())

			err := adapter.cleanupProcessingResources(nil, newRoleBinding)
			Expect(err).NotTo(HaveOccurred())

			checkRoleBinding := &rbac.RoleBinding{}
			err = toolkit.GetObject(newRoleBinding.Name, newRoleBinding.Namespace, adapter.client, adapter.ctx, checkRoleBinding)
			Expect(checkRoleBinding).To(Equal(&rbac.RoleBinding{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("removes the rolebinding and role if present", func() {
			newRole := &rbac.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-role",
					Namespace: "default",
				},
			}
			newRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-role-binding",
					Namespace: "default",
				},
				RoleRef: rbac.RoleRef{
					APIGroup: rbac.GroupName,
					Kind:     "Role",
					Name:     "foo-role",
				},
			}
			// The resource needs to be created as it will get patched
			Expect(adapter.client.Create(adapter.ctx, newRole)).To(Succeed())
			Expect(adapter.client.Create(adapter.ctx, newRoleBinding)).To(Succeed())

			err := adapter.cleanupProcessingResources(nil, newRoleBinding)
			Expect(err).NotTo(HaveOccurred())

			checkRoleBinding := &rbac.RoleBinding{}
			err = toolkit.GetObject(newRoleBinding.Name, newRoleBinding.Namespace, adapter.client, adapter.ctx, checkRoleBinding)
			Expect(checkRoleBinding).To(Equal(&rbac.RoleBinding{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())

			checkRole := &rbac.Role{}
			err = toolkit.GetObject(newRole.Name, newRole.Namespace, adapter.client, adapter.ctx, checkRole)
			Expect(checkRole).To(Equal(&rbac.Role{}))
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("removes the pipelineRun if present", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "pipeline-run",
					Namespace:  "default",
					Finalizers: []string{metadata.ReleaseFinalizer},
				},
			}
			// The resource needs to be created as it will get patched
			Expect(adapter.client.Create(adapter.ctx, pipelineRun)).To(Succeed())

			err := adapter.cleanupProcessingResources(pipelineRun, nil)
			Expect(err).NotTo(HaveOccurred())

			checkPipelineRun := &tektonv1.PipelineRun{}
			err = toolkit.GetObject(pipelineRun.Name, pipelineRun.Namespace, adapter.client, adapter.ctx, checkPipelineRun)
			Expect(err).NotTo(HaveOccurred())
			Expect(checkPipelineRun).NotTo(BeNil())
			Expect(checkPipelineRun.Finalizers).To(HaveLen(0))

			// Cleanup as only the finalizer was removed
			Expect(adapter.client.Delete(adapter.ctx, checkPipelineRun)).To(Succeed())
		})

		It("should not error if either resource is nil", func() {
			err := adapter.cleanupProcessingResources(nil, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("createManagedCollectorsPipelineRun is called", func() {
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

			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			var err error
			pipelineRun, err = adapter.createManagedCollectorsPipelineRun(newReleasePlanAdmission)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a PipelineRun with the right prefix", func() {
			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1.PipelineRun{})))
			Expect(pipelineRun.Name).To(HavePrefix(metadata.ManagedCollectorsPipelineType.String()))
		})

		It("has the release reference", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(adapter.release.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", adapter.release.Namespace, types.Separator, adapter.release.Name))))
		})
		It("has owner annotations", func() {
			Expect(pipelineRun.GetAnnotations()[handler.NamespacedNameAnnotation]).To(ContainSubstring(adapter.release.Name))
			Expect(pipelineRun.GetAnnotations()[handler.TypeAnnotation]).To(ContainSubstring("Release"))
		})

		It("has release labels", func() {
			Expect(pipelineRun.GetLabels()[metadata.PipelinesTypeLabel]).To(Equal(metadata.ManagedCollectorsPipelineType.String()))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNameLabel]).To(Equal(adapter.release.Name))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNamespaceLabel]).To(Equal(testNamespace))
			Expect(pipelineRun.GetLabels()[metadata.ServiceNameLabel]).To(Equal(metadata.ServiceName))
		})

		It("contains a parameter with the collectorsResourceType", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitUrl")))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", "releaseplanadmission")))
		})

		It("contains a parameter with the taskGitUrl", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitUrl")))
			var url string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "url" {
					url = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", url)))
		})

		It("contains a parameter with the taskGitRevision", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitRevision")))
			var revision string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "revision" {
					revision = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", revision)))
		})
	})

	When("createTenantCollectorsPipelineRun is called", func() {
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

			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}

			var err error
			pipelineRun, err = adapter.createTenantCollectorsPipelineRun(newReleasePlan, releasePlanAdmission)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a PipelineRun with the right prefix", func() {
			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1.PipelineRun{})))
			Expect(pipelineRun.Name).To(HavePrefix(metadata.TenantCollectorsPipelineType.String()))
		})

		It("has the release reference", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(adapter.release.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", adapter.release.Namespace, types.Separator, adapter.release.Name))))
		})

		It("has owner annotations", func() {
			Expect(pipelineRun.GetAnnotations()[handler.NamespacedNameAnnotation]).To(ContainSubstring(adapter.release.Name))
			Expect(pipelineRun.GetAnnotations()[handler.TypeAnnotation]).To(ContainSubstring("Release"))
		})

		It("has release labels", func() {
			Expect(pipelineRun.GetLabels()[metadata.PipelinesTypeLabel]).To(Equal(metadata.TenantCollectorsPipelineType.String()))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNameLabel]).To(Equal(adapter.release.Name))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNamespaceLabel]).To(Equal(testNamespace))
			Expect(pipelineRun.GetLabels()[metadata.ServiceNameLabel]).To(Equal(metadata.ServiceName))
		})

		It("contains a parameter with the collectorsResourceType", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitUrl")))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", "releaseplan")))
		})

		It("contains a parameter with the taskGitUrl", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitUrl")))
			var url string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "url" {
					url = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", url)))
		})

		It("contains a parameter with the taskGitRevision", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitRevision")))
			var revision string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "revision" {
					revision = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", revision)))
		})
	})

	When("createTenantPipelineRun is called", func() {
		var (
			adapter        *adapter
			pipelineRun    *tektonv1.PipelineRun
			newReleasePlan *v1alpha1.ReleasePlan
		)

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)

			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()

			parameterizedPipeline := tektonutils.ParameterizedPipeline{}
			parameterizedPipeline.PipelineRef = tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: "my-url"},
					{Name: "revision", Value: "my-revision"},
					{Name: "pathInRepo", Value: "my-path"},
				},
			}
			parameterizedPipeline.Params = []tektonutils.Param{
				{Name: "parameter1", Value: "value1"},
				{Name: "parameter2", Value: "value2"},
			}
			parameterizedPipeline.TaskRunSpecs = []tektonv1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "task1",
					ComputeResources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
				{
					PipelineTaskName: "task2",
					ComputeResources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			}
			parameterizedPipeline.Timeouts = tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
			}

			newReleasePlan = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application:            application.Name,
					TenantPipeline:         &parameterizedPipeline,
					ReleaseGracePeriodDays: 6,
				},
			}
			newReleasePlan.Kind = "ReleasePlan"

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
			})

			var err error
			pipelineRun, err = adapter.createTenantPipelineRun(newReleasePlan, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a PipelineRun with the right prefix", func() {
			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1.PipelineRun{})))
			Expect(pipelineRun.Name).To(HavePrefix("tenant"))
		})

		It("has the release reference", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(adapter.release.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", adapter.release.Namespace, types.Separator, adapter.release.Name))))
		})

		It("has the releasePlan reference", func() {
			name := []rune(releasePlan.Kind)
			name[0] = unicode.ToLower(name[0])

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", string(name))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releasePlan.Namespace, types.Separator, releasePlan.Name))))
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
			Expect(pipelineRun.GetLabels()[metadata.PipelinesTypeLabel]).To(Equal(metadata.TenantPipelineType.String()))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNameLabel]).To(Equal(adapter.release.Name))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNamespaceLabel]).To(Equal(testNamespace))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseSnapshotLabel]).To(Equal(adapter.release.Spec.Snapshot))
			Expect(pipelineRun.GetLabels()[metadata.ServiceNameLabel]).To(Equal(metadata.ServiceName))
		})

		It("contains a parameter with the taskGitUrl", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitUrl")))
			var url string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "url" {
					url = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", url)))
		})

		It("contains a parameter with the taskGitRevision", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitRevision")))
			var revision string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "revision" {
					revision = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", revision)))
		})

		It("contains the proper taskRunSpecs", func() {
			Expect(pipelineRun.Spec.TaskRunSpecs).To(Equal(newReleasePlan.Spec.TenantPipeline.TaskRunSpecs))
		})

		It("contains the proper timeout value", func() {
			Expect(pipelineRun.Spec.Timeouts.Pipeline).To(Equal(newReleasePlan.Spec.TenantPipeline.Timeouts.Pipeline))
		})
	})

	When("createManagedPipelineRun is called", func() {
		var (
			adapter     *adapter
			pipelineRun *tektonv1.PipelineRun
			resources   *loader.ProcessingResources
		)

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)

			if pipelineRun != nil {
				Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
			}
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			adapter.releaseServiceConfig = releaseServiceConfig
			pipelineRun = nil
			resources = &loader.ProcessingResources{
				ReleasePlan:                 releasePlan,
				ReleasePlanAdmission:        releasePlanAdmission,
				EnterpriseContractConfigMap: enterpriseContractConfigMap,
				EnterpriseContractPolicy:    enterpriseContractPolicy,
				Snapshot:                    snapshot,
			}
		})

		It("returns a PipelineRun with the right prefix", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1.PipelineRun{})))
			Expect(pipelineRun.Name).To(HavePrefix("managed"))
		})

		It("has the release reference", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(adapter.release.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", adapter.release.Namespace, types.Separator, adapter.release.Name))))
		})

		It("has the releasePlan reference", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			name := []rune(releasePlan.Kind)
			name[0] = unicode.ToLower(name[0])

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", string(name))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releasePlan.Namespace, types.Separator, releasePlan.Name))))
		})

		It("has the releasePlanAdmission reference", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			name := []rune(releasePlanAdmission.Kind)
			name[0] = unicode.ToLower(name[0])

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", string(name))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releasePlanAdmission.Namespace, types.Separator, releasePlanAdmission.Name))))
		})

		It("has the releaseServiceConfig reference", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			name := []rune(releaseServiceConfig.Kind)
			name[0] = unicode.ToLower(name[0])

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", string(name))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releaseServiceConfig.Namespace, types.Separator, releaseServiceConfig.Name))))
		})

		It("has the snapshot reference", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(snapshot.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", snapshot.Namespace, types.Separator, snapshot.Name))))
		})

		It("has owner annotations", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.GetAnnotations()[handler.NamespacedNameAnnotation]).To(ContainSubstring(adapter.release.Name))
			Expect(pipelineRun.GetAnnotations()[handler.TypeAnnotation]).To(ContainSubstring("Release"))
		})

		It("has release labels", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.GetLabels()[metadata.PipelinesTypeLabel]).To(Equal(metadata.ManagedPipelineType.String()))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNameLabel]).To(Equal(adapter.release.Name))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNamespaceLabel]).To(Equal(testNamespace))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseSnapshotLabel]).To(Equal(adapter.release.Spec.Snapshot))
			Expect(pipelineRun.GetLabels()[metadata.ServiceNameLabel]).To(Equal(metadata.ServiceName))
		})

		It("references the pipeline specified in the ReleasePlanAdmission", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			var pipelineUrl string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "url" {
					pipelineUrl = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineUrl).To(Equal(releasePlanAdmission.Spec.Pipeline.PipelineRef.Params[0].Value))
		})

		It("contains a parameter with the taskGitUrl", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitUrl")))
			var url string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "url" {
					url = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", url)))
		})

		It("contains a parameter with the taskGitRevision", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitRevision")))
			var revision string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "revision" {
					revision = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", revision)))
		})

		It("contains the proper taskRunSpecs", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun.Spec.TaskRunSpecs).To(Equal(releasePlanAdmission.Spec.Pipeline.TaskRunSpecs))
		})

		It("contains the proper timeout value", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.Spec.Timeouts.Pipeline).To(Equal(releasePlanAdmission.Spec.Pipeline.Timeouts.Pipeline))
		})

		It("contains parameters with the verify ec task bundle and verify conforma git revision", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			bundle := enterpriseContractConfigMap.Data["verify_ec_task_bundle"]
			revision := enterpriseContractConfigMap.Data["verify_ec_task_git_revision"]
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(bundle)))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(revision)))))
		})

		It("contains a parameter with the json representation of the EnterpriseContractPolicy", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			jsonSpec, _ := json.Marshal(enterpriseContractPolicy.Spec)
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(jsonSpec)))))
		})

		It("contains a workspace using EmptyDir if there's an override for the pipeline", func() {
			url, revision, pathInRepo, err := releasePlanAdmission.Spec.Pipeline.PipelineRef.GetGitResolverParams()
			Expect(err).To(BeNil())
			Expect(url).NotTo(BeEmpty())
			Expect(revision).NotTo(BeEmpty())
			Expect(pathInRepo).NotTo(BeEmpty())

			releaseServiceConfig := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					EmptyDirOverrides: []v1alpha1.EmptyDirOverrides{
						{
							Url:        url,
							Revision:   revision,
							PathInRepo: pathInRepo,
						},
					},
				},
			}
			releaseServiceConfig.Kind = "ReleaseServiceConfig"
			adapter.releaseServiceConfig = releaseServiceConfig

			var pipelineRun *tektonv1.PipelineRun
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.Spec.Workspaces).To(HaveLen(1))
			Expect(pipelineRun.Spec.Workspaces[0].EmptyDir).NotTo(BeNil())
			Expect(pipelineRun.Spec.Workspaces[0].VolumeClaimTemplate).To(BeNil())
		})

		It("contains a workspace using EmptyDir if there's an override for the pipeline using regex", func() {
			_, _, pathInRepo, err := releasePlanAdmission.Spec.Pipeline.PipelineRef.GetGitResolverParams()
			Expect(err).To(BeNil())
			Expect(pathInRepo).NotTo(BeEmpty())

			releaseServiceConfig := &v1alpha1.ReleaseServiceConfig{
				Spec: v1alpha1.ReleaseServiceConfigSpec{
					EmptyDirOverrides: []v1alpha1.EmptyDirOverrides{
						{
							Url:        ".*",
							Revision:   ".*",
							PathInRepo: pathInRepo,
						},
					},
				},
			}
			releaseServiceConfig.Kind = "ReleaseServiceConfig"
			adapter.releaseServiceConfig = releaseServiceConfig

			var pipelineRun *tektonv1.PipelineRun
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.Spec.Workspaces).To(HaveLen(1))
			Expect(pipelineRun.Spec.Workspaces[0].EmptyDir).NotTo(BeNil())
			Expect(pipelineRun.Spec.Workspaces[0].VolumeClaimTemplate).To(BeNil())
		})

		It("contains a workspace using EmptyDir if there's not an override for the pipeline", func() {
			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineRun.Spec.Workspaces).To(HaveLen(1))
			Expect(pipelineRun.Spec.Workspaces[0].VolumeClaimTemplate).NotTo(BeNil())
			Expect(pipelineRun.Spec.Workspaces[0].EmptyDir).To(BeNil())
		})
	})

	When("createFinalPipelineRun is called", func() {
		var (
			adapter        *adapter
			pipelineRun    *tektonv1.PipelineRun
			newReleasePlan *v1alpha1.ReleasePlan
		)

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)

			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()

			parameterizedPipeline := tektonutils.ParameterizedPipeline{}
			parameterizedPipeline.PipelineRef = tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: "my-url"},
					{Name: "revision", Value: "my-revision"},
					{Name: "pathInRepo", Value: "my-path"},
				},
			}
			parameterizedPipeline.Params = []tektonutils.Param{
				{Name: "parameter1", Value: "value1"},
				{Name: "parameter2", Value: "value2"},
			}
			parameterizedPipeline.TaskRunSpecs = []tektonv1.PipelineTaskRunSpec{
				{
					PipelineTaskName: "task1",
					ComputeResources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
				{
					PipelineTaskName: "task2",
					ComputeResources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			}
			parameterizedPipeline.Timeouts = tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
			}

			newReleasePlan = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application:            application.Name,
					FinalPipeline:          &parameterizedPipeline,
					ReleaseGracePeriodDays: 6,
				},
			}
			newReleasePlan.Kind = "ReleasePlan"

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
			})

			var err error
			pipelineRun, err = adapter.createFinalPipelineRun(newReleasePlan, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a PipelineRun with the right prefix", func() {
			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1.PipelineRun{})))
			Expect(pipelineRun.Name).To(HavePrefix("final"))
		})

		It("has the release reference", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", strings.ToLower(adapter.release.Kind))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", adapter.release.Namespace, types.Separator, adapter.release.Name))))
		})

		It("has the releasePlan reference", func() {
			name := []rune(releasePlan.Kind)
			name[0] = unicode.ToLower(name[0])

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", string(name))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releasePlan.Namespace, types.Separator, releasePlan.Name))))
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
			Expect(pipelineRun.GetLabels()[metadata.PipelinesTypeLabel]).To(Equal(metadata.FinalPipelineType.String()))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNameLabel]).To(Equal(adapter.release.Name))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNamespaceLabel]).To(Equal(testNamespace))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseSnapshotLabel]).To(Equal(adapter.release.Spec.Snapshot))
		})

		It("contains a parameter with the taskGitUrl", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitUrl")))
			var url string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "url" {
					url = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", url)))
		})

		It("contains a parameter with the taskGitRevision", func() {
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", "taskGitRevision")))
			var revision string
			resolverParams := pipelineRun.Spec.PipelineRef.ResolverRef.Params
			for i := range resolverParams {
				if resolverParams[i].Name == "revision" {
					revision = resolverParams[i].Value.StringVal
				}
			}
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", revision)))
		})

		It("contains the proper timeout value", func() {
			Expect(pipelineRun.Spec.Timeouts.Pipeline).To(Equal(newReleasePlan.Spec.FinalPipeline.Timeouts.Pipeline))
		})

		It("contains the proper taskRunSpecs", func() {
			Expect(pipelineRun.Spec.TaskRunSpecs).To(Equal(newReleasePlan.Spec.FinalPipeline.TaskRunSpecs))
		})

	})

	When("createRoleBindingForCollectorSecrets is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("creates a new roleBinding", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			roleBinding, err := adapter.createRoleBindingForCollectorSecrets("foo", adapter.release.Namespace, "default", []string{"bar", "foo"})
			Expect(err).NotTo(HaveOccurred())
			Expect(roleBinding).NotTo(BeNil())
			Expect(roleBinding.Name).To(ContainSubstring("rolebinding-for-foo"))
			Expect(roleBinding.RoleRef.Name).To(ContainSubstring("role-for-foo"))
			Expect(k8sClient.Delete(ctx, roleBinding)).Should(Succeed())
		})

	})

	When("createRoleBindingForClusterRole is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("fails when the releasePlanAdmission has no serviceAccount", func() {
			newReleasePlanAdmission := &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan-admission",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Applications: []string{application.Name},
					Origin:       "default",
					Pipeline: &tektonutils.Pipeline{
						PipelineRef: tektonutils.PipelineRef{
							Resolver: "git",
							Params: []tektonutils.Param{
								{Name: "url", Value: "my-url"},
								{Name: "revision", Value: "my-revision"},
								{Name: "pathInRepo", Value: "my-path"},
							},
						},
					},
					Policy: enterpriseContractPolicy.Name,
				},
			}
			roleBinding, err := adapter.createRoleBindingForClusterRole("foo", newReleasePlanAdmission.Spec.Origin, newReleasePlanAdmission.Spec.Pipeline.ServiceAccountName, newReleasePlanAdmission.Namespace)
			Expect(err).To(HaveOccurred())
			Expect(roleBinding).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("is invalid"))
		})

		It("creates a new roleBinding", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			roleBinding, err := adapter.createRoleBindingForClusterRole("foo", releasePlanAdmission.Spec.Origin, releasePlanAdmission.Spec.Pipeline.ServiceAccountName, releasePlanAdmission.Namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(roleBinding).NotTo(BeNil())
			Expect(roleBinding.RoleRef.Name).To(Equal("foo"))

			Expect(k8sClient.Delete(ctx, roleBinding)).Should(Succeed())
		})
	})

	When("finalizeRelease is called", func() {
		var adapter *adapter
		var parameterizedPipeline *tektonutils.ParameterizedPipeline
		var newReleasePlan *v1alpha1.ReleasePlan

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			parameterizedPipeline = &tektonutils.ParameterizedPipeline{}
			parameterizedPipeline.PipelineRef = tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: "my-url"},
					{Name: "revision", Value: "my-revision"},
					{Name: "pathInRepo", Value: "my-path"},
				},
			}
			parameterizedPipeline.Params = []tektonutils.Param{
				{Name: "parameter1", Value: "value1"},
				{Name: "parameter2", Value: "value2"},
			}
			parameterizedPipeline.Timeouts = tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
			}
			newReleasePlan = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "release-plan",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application:            application.Name,
					TenantPipeline:         parameterizedPipeline,
					FinalPipeline:          parameterizedPipeline,
					ReleaseGracePeriodDays: 6,
				},
			}
			newReleasePlan.Kind = "ReleasePlan"
		})

		It("finalizes the Release and removes the finalizer from the Managed Collector PipelineRun when called with false", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}
			pipelineRun, err := adapter.createManagedCollectorsPipelineRun(newReleasePlanAdmission)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(false)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedCollectorsPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).NotTo(BeNil())
			Expect(pipelineRun.Finalizers).To(HaveLen(0))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("finalizes the Release and removes the finalizer from the Tenant Collector PipelineRun when called with false", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}
			pipelineRun, err := adapter.createTenantCollectorsPipelineRun(newReleasePlan, releasePlanAdmission)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(false)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantCollectorsPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).NotTo(BeNil())
			Expect(pipelineRun.Finalizers).To(HaveLen(0))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("finalizes the Release and removes the finalizer from the Tenant PipelineRun when called with false", func() {
			pipelineRun, err := adapter.createTenantPipelineRun(newReleasePlan, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(false)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).NotTo(BeNil())
			Expect(pipelineRun.Finalizers).To(HaveLen(0))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("finalizes the Release and removes the finalizer from the Managed PipelineRun when called with false", func() {
			adapter.releaseServiceConfig = releaseServiceConfig
			resources := &loader.ProcessingResources{
				ReleasePlan:                 releasePlan,
				ReleasePlanAdmission:        releasePlanAdmission,
				EnterpriseContractConfigMap: enterpriseContractConfigMap,
				EnterpriseContractPolicy:    enterpriseContractPolicy,
				Snapshot:                    snapshot,
			}
			pipelineRun, err := adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(false)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).NotTo(BeNil())
			Expect(pipelineRun.Finalizers).To(HaveLen(0))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("finalizes the Release and removes the finalizer from the Final PipelineRun when called with false", func() {
			pipelineRun, err := adapter.createFinalPipelineRun(newReleasePlan, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(false)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.FinalPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).NotTo(BeNil())
			Expect(pipelineRun.Finalizers).To(HaveLen(0))
			Expect(k8sClient.Delete(ctx, pipelineRun)).To(Succeed())
		})

		It("finalizes the Release and deletes the Managed Collectors PipelineRun when called with true", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}
			pipelineRun, err := adapter.createManagedCollectorsPipelineRun(newReleasePlanAdmission)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(true)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedCollectorsPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).To(BeNil())
		})

		It("finalizes the Release and deletes the Tenant Collectors PipelineRun when called with true", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Collectors = &v1alpha1.Collectors{
				Items: []v1alpha1.CollectorItem{
					{
						Name:   "foo",
						Type:   "bar",
						Params: []v1alpha1.Param{},
					},
				},
			}
			pipelineRun, err := adapter.createTenantCollectorsPipelineRun(newReleasePlan, releasePlanAdmission)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(true)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantCollectorsPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).To(BeNil())
		})

		It("finalizes the Release and deletes the Tenant PipelineRun when called with true", func() {
			pipelineRun, err := adapter.createTenantPipelineRun(newReleasePlan, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(true)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.TenantPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).To(BeNil())
		})

		It("finalizes the Release and deletes the Managed PipelineRun when called with true", func() {
			adapter.releaseServiceConfig = releaseServiceConfig
			resources := &loader.ProcessingResources{
				ReleasePlan:                 releasePlan,
				ReleasePlanAdmission:        releasePlanAdmission,
				EnterpriseContractConfigMap: enterpriseContractConfigMap,
				EnterpriseContractPolicy:    enterpriseContractPolicy,
				Snapshot:                    snapshot,
			}
			pipelineRun, err := adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(true)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.ManagedPipelineType)
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineRun).To(BeNil())
		})

		It("finalizes the Release and deletes the Final PipelineRun when called with true", func() {
			pipelineRun, err := adapter.createFinalPipelineRun(newReleasePlan, snapshot)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(adapter.finalizeRelease(true)).To(Succeed())
			pipelineRun, err = adapter.loader.GetReleasePipelineRun(adapter.ctx, adapter.client, adapter.release, metadata.FinalPipelineType)
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

		It("should return a ReleaseServiceConfig without Spec and with the right ObjectMeta and Kind set", func() {
			releaseServiceConfig := adapter.getEmptyReleaseServiceConfig("namespace")
			Expect(releaseServiceConfig).NotTo(BeNil())
			Expect(releaseServiceConfig.Name).To(Equal(v1alpha1.ReleaseServiceConfigResourceName))
			Expect(releaseServiceConfig.Namespace).To(Equal("namespace"))
			Expect(releaseServiceConfig.Kind).To(Equal("ReleaseServiceConfig"))
		})
	})

	When("registerManagedCollectorsProcessingData is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerManagedCollectorsProcessingData(nil, nil, nil, nil)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.PipelineRun).To(BeEmpty())
		})

		It("registers the Release managed collectors processing data", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			tenantRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role-binding",
					Namespace: "default",
				},
			}
			managedRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role-binding",
					Namespace: "default",
				},
			}
			secretRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role-binding",
					Namespace: "default",
				},
			}
			Expect(adapter.registerManagedCollectorsProcessingData(pipelineRun, tenantRoleBinding, managedRoleBinding, secretRoleBinding)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.PipelineRun).To(Equal(fmt.Sprintf("%s%c%s",
				pipelineRun.Namespace, types.Separator, pipelineRun.Name)))
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.TenantRoleBinding).To(Equal(fmt.Sprintf("%s%c%s",
				tenantRoleBinding.Namespace, types.Separator, tenantRoleBinding.Name)))
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.ManagedRoleBinding).To(Equal(fmt.Sprintf("%s%c%s",
				managedRoleBinding.Namespace, types.Separator, managedRoleBinding.Name)))
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings.SecretRoleBinding).To(Equal(fmt.Sprintf("%s%c%s",
				secretRoleBinding.Namespace, types.Separator, secretRoleBinding.Name)))
			Expect(adapter.release.IsManagedCollectorsPipelineProcessing()).To(BeTrue())
		})

		It("does not set RoleBinding when no RoleBinding is passed", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}

			Expect(adapter.registerManagedCollectorsProcessingData(pipelineRun, nil, nil, nil)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.RoleBindings).To(Equal(v1alpha1.RoleBindingType{}))
			Expect(adapter.release.IsManagedCollectorsPipelineProcessing()).To(BeTrue())
		})
	})

	When("registerTenantCollectorsProcessingData is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerTenantCollectorsProcessingData(nil, nil, nil)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.PipelineRun).To(BeEmpty())
		})

		It("registers the Release tenant collectors processing data", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			tenantRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tenant-role-binding",
					Namespace: "default",
				},
			}
			secretRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-role-binding",
					Namespace: "default",
				},
			}
			Expect(adapter.registerTenantCollectorsProcessingData(pipelineRun, tenantRoleBinding, secretRoleBinding)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.PipelineRun).To(Equal(fmt.Sprintf("%s%c%s",
				pipelineRun.Namespace, types.Separator, pipelineRun.Name)))
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings.TenantRoleBinding).To(Equal(fmt.Sprintf("%s%c%s",
				tenantRoleBinding.Namespace, types.Separator, tenantRoleBinding.Name)))
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings.SecretRoleBinding).To(Equal(fmt.Sprintf("%s%c%s",
				secretRoleBinding.Namespace, types.Separator, secretRoleBinding.Name)))
			Expect(adapter.release.IsTenantCollectorsPipelineProcessing()).To(BeTrue())
		})

		It("does not set RoleBinding when no RoleBinding is passed", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}

			Expect(adapter.registerTenantCollectorsProcessingData(pipelineRun, nil, nil)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.RoleBindings).To(Equal(v1alpha1.RoleBindingType{}))
			Expect(adapter.release.IsTenantCollectorsPipelineProcessing()).To(BeTrue())
		})
	})

	When("registerTenantProcessingData is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerTenantProcessingData(nil)).To(Succeed())
			Expect(adapter.release.Status.TenantProcessing.PipelineRun).To(BeEmpty())
		})

		It("registers the Release tenant processing data", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			Expect(adapter.registerTenantProcessingData(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.TenantProcessing.PipelineRun).To(Equal(fmt.Sprintf("%s%c%s",
				pipelineRun.Namespace, types.Separator, pipelineRun.Name)))
			Expect(adapter.release.IsTenantPipelineProcessing()).To(BeTrue())
		})
	})

	When("registerManagedProcessingData is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerManagedProcessingData(nil, nil)).To(Succeed())
			Expect(adapter.release.Status.ManagedProcessing.PipelineRun).To(BeEmpty())
		})

		It("registers the Release managed processing data", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			tenantRoleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tenant-role-binding",
					Namespace: "default",
				},
			}
			Expect(adapter.registerManagedProcessingData(pipelineRun, tenantRoleBinding)).To(Succeed())
			Expect(adapter.release.Status.ManagedProcessing.PipelineRun).To(Equal(fmt.Sprintf("%s%c%s",
				pipelineRun.Namespace, types.Separator, pipelineRun.Name)))
			Expect(adapter.release.Status.ManagedProcessing.RoleBindings.TenantRoleBinding).To(Equal(fmt.Sprintf("%s%c%s",
				tenantRoleBinding.Namespace, types.Separator, tenantRoleBinding.Name)))
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeTrue())
		})

		It("does not set RoleBinding when no RoleBinding is passed", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}

			Expect(adapter.registerManagedProcessingData(pipelineRun, nil)).To(Succeed())
			Expect(adapter.release.Status.ManagedProcessing.RoleBindings).To(Equal(v1alpha1.RoleBindingType{}))
			Expect(adapter.release.IsManagedPipelineProcessing()).To(BeTrue())
		})
	})

	When("registerFinalProcessingData is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerFinalProcessingData(nil)).To(Succeed())
			Expect(adapter.release.Status.FinalProcessing.PipelineRun).To(BeEmpty())
		})

		It("registers the Release final processing data", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			Expect(adapter.registerFinalProcessingData(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.FinalProcessing.PipelineRun).To(Equal(fmt.Sprintf("%s%c%s",
				pipelineRun.Namespace, types.Separator, pipelineRun.Name)))

		})
	})

	When("registerManagedCollectorsProcessingStatus is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerManagedCollectorsProcessingStatus(nil)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime).To(BeNil())
		})

		It("does nothing if the PipelineRun is not done", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			Expect(adapter.registerManagedCollectorsProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime).To(BeNil())
		})

		It("sets the Release as Managed Collectors Processed if the PipelineRun succeeded", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.release.MarkManagedCollectorsPipelineProcessing()

			Expect(adapter.registerManagedCollectorsProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.IsManagedCollectorsPipelineProcessedSuccessfully()).To(BeTrue())
		})

		It("sets the Release as ManagedCollectors Processing failed if the PipelineRun didn't succeed", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkFailed("", "")
			adapter.release.MarkManagedCollectorsPipelineProcessing()

			Expect(adapter.registerManagedCollectorsProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.HasManagedCollectorsPipelineProcessingFinished()).To(BeTrue())
			Expect(adapter.release.IsManagedCollectorsPipelineProcessedSuccessfully()).To(BeFalse())
		})
	})

	When("registerTenantCollectorsProcessingStatus is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerTenantCollectorsProcessingStatus(nil)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime).To(BeNil())
		})

		It("does nothing if the PipelineRun is not done", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			Expect(adapter.registerTenantCollectorsProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime).To(BeNil())
		})

		It("sets the Release as Tenant Collectors Processed if the PipelineRun succeeded", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.release.MarkTenantCollectorsPipelineProcessing()

			Expect(adapter.registerTenantCollectorsProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.IsTenantCollectorsPipelineProcessedSuccessfully()).To(BeTrue())
		})

		It("sets the Release as Tenant Collectors Processing failed if the PipelineRun didn't succeed", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkFailed("", "")
			adapter.release.MarkTenantCollectorsPipelineProcessing()

			Expect(adapter.registerTenantCollectorsProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.HasTenantCollectorsPipelineProcessingFinished()).To(BeTrue())
			Expect(adapter.release.IsTenantCollectorsPipelineProcessedSuccessfully()).To(BeFalse())
		})
	})

	When("registerTenantProcessingStatus is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerTenantProcessingStatus(nil)).To(Succeed())
			Expect(adapter.release.Status.TenantProcessing.CompletionTime).To(BeNil())
		})

		It("does nothing if the PipelineRun is not done", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			Expect(adapter.registerTenantProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.TenantProcessing.CompletionTime).To(BeNil())
		})

		It("sets the Release as Tenant Processed if the PipelineRun succeeded", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.release.MarkTenantPipelineProcessing()

			Expect(adapter.registerTenantProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.IsTenantPipelineProcessedSuccessfully()).To(BeTrue())
		})

		It("sets the Release as Tenant Processing failed if the PipelineRun didn't succeed", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkFailed("", "")
			adapter.release.MarkTenantPipelineProcessing()

			Expect(adapter.registerTenantProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.HasTenantPipelineProcessingFinished()).To(BeTrue())
			Expect(adapter.release.IsTenantPipelineProcessedSuccessfully()).To(BeFalse())
		})
	})

	When("registerManagedProcessingStatus is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerManagedProcessingStatus(nil)).To(Succeed())
			Expect(adapter.release.Status.ManagedProcessing.CompletionTime).To(BeNil())
		})

		It("does nothing if the PipelineRun is not done", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			Expect(adapter.registerManagedProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.ManagedProcessing.CompletionTime).To(BeNil())
		})

		It("sets the Release as Managed Processed if the PipelineRun succeeded", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.release.MarkManagedPipelineProcessing()

			Expect(adapter.registerManagedProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.IsManagedPipelineProcessedSuccessfully()).To(BeTrue())
		})

		It("sets the Release as Managed Processing failed if the PipelineRun didn't succeed", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkFailed("", "")
			adapter.release.MarkManagedPipelineProcessing()

			Expect(adapter.registerManagedProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.HasManagedPipelineProcessingFinished()).To(BeTrue())
			Expect(adapter.release.IsManagedPipelineProcessedSuccessfully()).To(BeFalse())
		})
	})

	When("registerFinalProcessingStatus is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerFinalProcessingStatus(nil)).To(Succeed())
			Expect(adapter.release.Status.FinalProcessing.CompletionTime).To(BeNil())
		})

		It("does nothing if the PipelineRun is not done", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			Expect(adapter.registerFinalProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.Status.FinalProcessing.CompletionTime).To(BeNil())
		})

		It("sets the Release as Final Processed if the PipelineRun succeeded", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkSucceeded("", "")
			adapter.release.MarkFinalPipelineProcessing()

			Expect(adapter.registerFinalProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.IsFinalPipelineProcessedSuccessfully()).To(BeTrue())
		})

		It("sets the Release as Final Processing failed if the PipelineRun didn't succeed", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			pipelineRun.Status.MarkFailed("", "")
			adapter.release.MarkFinalPipelineProcessing()

			Expect(adapter.registerFinalProcessingStatus(pipelineRun)).To(Succeed())
			Expect(adapter.release.HasFinalPipelineProcessingFinished()).To(BeTrue())
			Expect(adapter.release.IsFinalPipelineProcessedSuccessfully()).To(BeFalse())
		})

	})

	When("validateApplication is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("returns valid and no error if the Application match", func() {
			result := adapter.validateApplication()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("returns invalid and error if the Application doesn't match", func() {
			var conditionMsg string
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Spec.Application = "non-existent"
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   newReleasePlan,
				},
			})

			result := adapter.validateApplication()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			for i := range adapter.release.Status.Conditions {
				if adapter.release.Status.Conditions[i].Type == "Validated" {
					conditionMsg = adapter.release.Status.Conditions[i].Message
				}
			}
			Expect(conditionMsg).To(Equal("different Application referenced in ReleasePlan and Snapshot"))
		})

		It("returns invalid if the ReleasePlan is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result := adapter.validateApplication()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("returns invalid if the Snapshot is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.SnapshotContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result := adapter.validateApplication()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
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
			AfterEach(func() {
				_ = adapter.client.Delete(ctx, adapter.release)
			})

			BeforeEach(func() {
				adapter = createReleaseAndAdapter()
				adapter.release.Labels = map[string]string{
					metadata.AutomatedLabel: "true",
				}
				adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
					{
						ContextKey: loader.ReleasePlanContextKey,
						Resource:   releasePlan,
					},
				})
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

			It("returns invalid if the error appears after 5 minutes of being created", func() {
				adapter.release.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(-7 * time.Minute)})
				result := adapter.validateAuthor()
				Expect(result.Valid).To(BeFalse())
				Expect(result.Err).NotTo(HaveOccurred())
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
						ReleasePlan:          releasePlan,
					},
				},
			})

			result := adapter.validateProcessingResources()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).To(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})
	})

	When("validatePipelineSource is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			releaseServiceConfig.Spec = v1alpha1.ReleaseServiceConfigSpec{}
			adapter.releaseServiceConfig = releaseServiceConfig
		})

		It("should return invalid and no error if the ReleasePlan is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result := adapter.validatePipelineSource()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("should return invalid and no error if the ReleasePlanAdmission is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			result := adapter.validatePipelineSource()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("should return invalid and an error if some other type of error occurs when retrieving the ReleasePlan", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        fmt.Errorf("internal error"),
					Resource:   releasePlan,
				},
			})

			result := adapter.validatePipelineSource()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).To(HaveOccurred())
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

			result := adapter.validatePipelineSource()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).To(HaveOccurred())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("returns invalid and no error if debug is false and the Final PipelineRef uses a cluster resolver", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
							Labels: map[string]string{
								metadata.AutoReleaseLabel: "true",
							},
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Target: "default",
							FinalPipeline: &tektonutils.ParameterizedPipeline{
								Pipeline: tektonutils.Pipeline{
									PipelineRef: tektonutils.PipelineRef{
										Resolver: "cluster",
										Params: []tektonutils.Param{
											{Name: "name", Value: "release-pipeline"},
											{Name: "namespace", Value: "default"},
											{Name: "kind", Value: "pipeline"},
										},
									},
								},

								Params: []tektonutils.Param{},
							},
						},
					},
				},
			})
			adapter.releaseServiceConfig.Spec.Debug = false
			result := adapter.validatePipelineSource()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).To(BeNil())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("returns invalid and no error if debug is false and the Managed PipelineRef uses a cluster resolver", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource: &v1alpha1.ReleasePlanAdmission{
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
									Resolver: "cluster",
									Params: []tektonutils.Param{
										{Name: "name", Value: "release-pipeline"},
										{Name: "namespace", Value: "default"},
										{Name: "kind", Value: "pipeline"},
									},
								},
							},
							Policy: enterpriseContractPolicy.Name,
						},
					},
				},
			})
			adapter.releaseServiceConfig.Spec.Debug = false

			result := adapter.validatePipelineSource()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).To(BeNil())
			Expect(adapter.release.IsValid()).To(BeFalse())
		})

		It("returns invalid and no error if debug is false and the Tenant PipelineRef uses a cluster resolver", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
							Labels: map[string]string{
								metadata.BlockReleasesLabel: "false",
							},
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Target: "default",
							TenantPipeline: &tektonutils.ParameterizedPipeline{
								Pipeline: tektonutils.Pipeline{
									PipelineRef: tektonutils.PipelineRef{
										Resolver: "cluster",
										Params: []tektonutils.Param{
											{Name: "name", Value: "release-pipeline"},
											{Name: "namespace", Value: "default"},
											{Name: "kind", Value: "pipeline"},
										},
									},
								},

								Params: []tektonutils.Param{},
							},
						},
					},
				},
			})
			adapter.releaseServiceConfig.Spec.Debug = false
			result := adapter.validatePipelineSource()
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

			result := adapter.validatePipelineSource()
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
			result := adapter.validatePipelineSource()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).To(BeNil())
		})
	})

	When("validatePipelineDefined is called", func() {
		var adapter *adapter
		var parameterizedPipeline *tektonutils.ParameterizedPipeline

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
			parameterizedPipeline = &tektonutils.ParameterizedPipeline{}
			parameterizedPipeline.PipelineRef = tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: "my-url"},
					{Name: "revision", Value: "my-revision"},
					{Name: "pathInRepo", Value: "my-path"},
				},
			}
			parameterizedPipeline.Params = []tektonutils.Param{
				{Name: "parameter1", Value: "value1"},
				{Name: "parameter2", Value: "value2"},
			}
			parameterizedPipeline.Timeouts = tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
			}
		})

		It("should return true if ReleasePlanAdmission and ReleasePlan have Pipeline Set", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource: &v1alpha1.ReleasePlanAdmission{
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
									Resolver: "cluster",
									Params: []tektonutils.Param{
										{Name: "name", Value: "release-pipeline"},
										{Name: "namespace", Value: "default"},
										{Name: "kind", Value: "pipeline"},
									},
								},
							},
							Policy: enterpriseContractPolicy.Name,
						},
					},
				},
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							TenantPipeline:         parameterizedPipeline,
							FinalPipeline:          parameterizedPipeline,
							ReleaseGracePeriodDays: 6,
							Target:                 "default",
						},
					},
				},
			})

			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return true if only ReleasePlan has Pipeline Set", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							TenantPipeline:         parameterizedPipeline,
							FinalPipeline:          parameterizedPipeline,
							ReleaseGracePeriodDays: 6,
							Target:                 "default",
						},
					},
				},
			})

			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return true if only ReleasePlan has TenantPipeline Set and it has no Target", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							TenantPipeline:         parameterizedPipeline,
							ReleaseGracePeriodDays: 6,
						},
					},
				},
			})

			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return true if only ReleasePlanAdmission has Pipeline Set", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource: &v1alpha1.ReleasePlanAdmission{
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
									Resolver: "cluster",
									Params: []tektonutils.Param{
										{Name: "name", Value: "release-pipeline"},
										{Name: "namespace", Value: "default"},
										{Name: "kind", Value: "pipeline"},
									},
								},
							},
							Policy: enterpriseContractPolicy.Name,
						},
					},
				},
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							ReleaseGracePeriodDays: 6,
							Target:                 "default",
						},
					},
				},
			})

			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeTrue())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return false if ReleasePlan has no Pipeline or Target set", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							ReleaseGracePeriodDays: 6,
						},
					},
				},
			})

			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return false if only the Final Pipeline is set", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							ReleaseGracePeriodDays: 6,
							FinalPipeline:          parameterizedPipeline,
						},
					},
				},
			})
			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return false if neither ReleasePlanAdmission nor ReleasePlan have Pipeline Set", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource: &v1alpha1.ReleasePlanAdmission{
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
							Policy:       enterpriseContractPolicy.Name,
						},
					},
				},
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							ReleaseGracePeriodDays: 6,
							Target:                 "default",
						},
					},
				},
			})

			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return false if ReleasePlan has no Pipeline Set and ReleasePlanAdmission is not found", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							ReleaseGracePeriodDays: 6,
							Target:                 "default",
						},
					},
				},
			})

			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should return false if ReleasePlan has no Pipeline Set and ReleasePlanAdmission is set to auto-release false", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Err:        fmt.Errorf("with auto-release label set to false"),
				},
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							ReleaseGracePeriodDays: 6,
							Target:                 "default",
						},
					},
				},
			})

			result := adapter.validatePipelineDefined()
			Expect(result.Valid).To(BeFalse())
			Expect(result.Err).NotTo(HaveOccurred())
		})

		It("should set the target in the release status to the ReleasePlan namespace if no target is defined", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							TenantPipeline:         parameterizedPipeline,
							FinalPipeline:          parameterizedPipeline,
							ReleaseGracePeriodDays: 6,
						},
					},
				},
			})

			_ = adapter.validatePipelineDefined()
			Expect(adapter.release.Status.Target).To(Equal("default"))
		})

		It("should set the target in the release status to the ReleasePlan target if one is defined", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource: &v1alpha1.ReleasePlan{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "release-plan",
							Namespace: "default",
						},
						Spec: v1alpha1.ReleasePlanSpec{
							Application:            application.Name,
							ReleaseGracePeriodDays: 6,
							Target:                 "foo",
						},
					},
				},
				{
					ContextKey: loader.ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})

			_ = adapter.validatePipelineDefined()
			Expect(adapter.release.Status.Target).To(Equal("foo"))
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
				"verify_ec_task_bundle":       "test-bundle",
				"verify_ec_task_git_revision": "main",
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
		releasePlan.Kind = "ReleasePlan"

		releaseServiceConfig = &v1alpha1.ReleaseServiceConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1alpha1.ReleaseServiceConfigResourceName,
				Namespace: "default",
			},
		}
		Expect(k8sClient.Create(ctx, releaseServiceConfig)).To(Succeed())
		releaseServiceConfig.Kind = "ReleaseServiceConfig"

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
						Resolver: "git",
						Params: []tektonutils.Param{
							{Name: "url", Value: "my-url"},
							{Name: "revision", Value: "my-revision"},
							{Name: "pathInRepo", Value: "my-path"},
						},
					},
					ServiceAccountName: "service-account",
					TaskRunSpecs: []tektonv1.PipelineTaskRunSpec{
						{
							PipelineTaskName: "task1",
							ComputeResources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
						{
							PipelineTaskName: "task2",
							ComputeResources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("400m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
					Timeouts: tektonv1.TimeoutFields{
						Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
					},
				},
				Policy: enterpriseContractPolicy.Name,
			},
		}
		Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())
		releasePlanAdmission.Kind = "ReleasePlanAdmission"

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
		snapshot.Kind = "Snapshot"
	}

	deleteResources = func() {
		Expect(k8sClient.Delete(ctx, application)).To(Succeed())
		Expect(k8sClient.Delete(ctx, component)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, enterpriseContractConfigMap)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, enterpriseContractPolicy)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlan)).To(Succeed())
		Expect(k8sClient.Delete(ctx, releasePlanAdmission)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, releaseServiceConfig)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, snapshot)).To(Succeed())
	}

})
