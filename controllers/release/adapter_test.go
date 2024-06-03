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

			pipelineRun, err := adapter.loader.GetManagedReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
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

		It("should complete the release if all the required phases have completed", func() {
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

		It("should mark the Release as releasing even if it is queued", func() {
			adapter.release.MarkReleasing("")
			adapter.release.MarkReleaseQueued("")
			Expect(adapter.release.IsReleaseQueued()).To(BeTrue())
			Expect(adapter.release.IsReleasing()).To(BeFalse())
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
			adapter.releaseServiceConfig = releaseServiceConfig
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
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   roleBinding,
				},
			})
			adapter.release.MarkProcessing("")

			result, err := adapter.EnsureReleaseIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should queue the Release if another PipelineRun is running for the same Application", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ActiveManagedReleasePipelineRunsContextKey,
					Resource: &tektonv1.PipelineRunList{
						Items: []tektonv1.PipelineRun{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "pipeline-run",
									Namespace: "default",
								},
							},
						},
					},
				},
			})

			adapter.release.MarkReleasing("")
			result, err := adapter.EnsureReleaseIsProcessed()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(result.RequeueDelay).To(Equal(time.Minute))
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsProcessing()).To(BeFalse())
			Expect(adapter.release.IsReleaseQueued()).To(BeTrue())

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

			Expect(adapter.release.Status.Processing.RoleBinding).To(BeEmpty())
			result, err := adapter.EnsureReleaseIsProcessed()
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

			roleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatus(adapter.ctx, adapter.client, adapter.release)
			Expect(roleBinding).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, roleBinding)).To(Succeed())
			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetManagedReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
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
					ContextKey: loader.ActiveManagedReleasePipelineRunsContextKey,
					Resource:   &tektonv1.PipelineRunList{},
				},
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

			Expect(adapter.release.Status.Processing.RoleBinding).To(BeEmpty())
			result, err := adapter.EnsureReleaseIsProcessed()
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

			roleBinding, err := adapter.loader.GetRoleBindingFromReleaseStatus(adapter.ctx, adapter.client, adapter.release)
			Expect(roleBinding).To(BeNil())
			Expect(err).To(HaveOccurred())
			// Still need to cleanup the PipelineRun
			pipelineRun, err := adapter.loader.GetManagedReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should create a pipelineRun and register the processing data if all the required resources are present", func() {
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ActiveManagedReleasePipelineRunsContextKey,
					Resource:   &tektonv1.PipelineRunList{},
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
			})

			result, err := adapter.EnsureReleaseIsProcessed()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.IsProcessing()).To(BeTrue())

			pipelineRun, err := adapter.loader.GetManagedReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
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
				{
					ContextKey: loader.RoleBindingContextKey,
				},
			})

			result, err := adapter.EnsureReleaseProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.release.HasProcessingFinished()).To(BeTrue())
		})

		It("should continue if the PipelineRun doesn't exist", func() {
			adapter.release.MarkProcessing("")

			result, err := adapter.EnsureReleaseProcessingIsTracked()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
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

	When("EnsureReleaseProcessingResourcesAreCleanedUp is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("should continue if the Release processing has not finished", func() {
			result, err := adapter.EnsureReleaseProcessingResourcesAreCleanedUp()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should requeue the release if an error occurs fetching the managed pipelineRun", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Err:        fmt.Errorf("error"),
				},
			})
			result, err := adapter.EnsureReleaseProcessingResourcesAreCleanedUp()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should requeue the release if an error occurs fetching the roleBinding", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource:   pipelineRun,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Err:        fmt.Errorf("error"),
				},
			})
			adapter.release.Status.Processing.RoleBinding = "one/two"
			result, err := adapter.EnsureReleaseProcessingResourcesAreCleanedUp()
			Expect(result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).To(HaveOccurred())
		})

		It("should call cleanupManagedPipelineRunResources if all the resources are present", func() {
			adapter.release.MarkProcessing("")
			adapter.release.MarkProcessed()
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			newRoleBinding := roleBinding.DeepCopy()

			adapter.ctx = toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePipelineRunContextKey,
					Resource:   pipelineRun,
				},
				{
					ContextKey: loader.RoleBindingContextKey,
					Resource:   newRoleBinding,
				},
			})
			adapter.release.Status.Processing.RoleBinding = fmt.Sprintf("%s%c%s",
				newRoleBinding.Namespace, types.Separator, newRoleBinding.Name)

			result, err := adapter.EnsureReleaseProcessingResourcesAreCleanedUp()
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
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

		It("removes the pipelineRun finalizer if present", func() {
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
			Expect(pipelineRun.Finalizers).To(BeEmpty())

			// Clean up at the end
			Expect(adapter.client.Delete(adapter.ctx, pipelineRun)).To(Succeed())
		})

		It("should not error if either resource is nil", func() {
			err := adapter.cleanupProcessingResources(nil, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("createManagedPipelineRun is called", func() {
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
			adapter.releaseServiceConfig = releaseServiceConfig
			resources := &loader.ProcessingResources{
				ReleasePlan:                 releasePlan,
				ReleasePlanAdmission:        releasePlanAdmission,
				EnterpriseContractConfigMap: enterpriseContractConfigMap,
				EnterpriseContractPolicy:    enterpriseContractPolicy,
				Snapshot:                    snapshot,
			}

			var err error
			pipelineRun, err = adapter.createManagedPipelineRun(resources)
			Expect(pipelineRun).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a PipelineRun with the right prefix", func() {
			Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1.PipelineRun{})))
			Expect(pipelineRun.Name).To(HavePrefix("managed"))
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

		It("has the releasePlanAdmission reference", func() {
			name := []rune(releasePlanAdmission.Kind)
			name[0] = unicode.ToLower(name[0])

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", string(name))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releasePlanAdmission.Namespace, types.Separator, releasePlanAdmission.Name))))
		})

		It("has the releaseServiceConfig reference", func() {
			name := []rune(releaseServiceConfig.Kind)
			name[0] = unicode.ToLower(name[0])

			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Name", string(name))))
			Expect(pipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal",
				fmt.Sprintf("%s%c%s", releaseServiceConfig.Namespace, types.Separator, releaseServiceConfig.Name))))
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
			Expect(pipelineRun.GetLabels()[metadata.PipelinesTypeLabel]).To(Equal(metadata.ManagedPipelineType))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNameLabel]).To(Equal(adapter.release.Name))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseNamespaceLabel]).To(Equal(testNamespace))
			Expect(pipelineRun.GetLabels()[metadata.ReleaseSnapshotLabel]).To(Equal(adapter.release.Spec.Snapshot))
		})

		It("references the pipeline specified in the ReleasePlanAdmission", func() {
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
			Expect(pipelineRun.Spec.Timeouts.Pipeline).To(Equal(releasePlanAdmission.Spec.Pipeline.Timeouts.Pipeline))
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
			roleBinding, err := adapter.createRoleBindingForClusterRole("foo", newReleasePlanAdmission)
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

			roleBinding, err := adapter.createRoleBindingForClusterRole("foo", releasePlanAdmission)
			Expect(err).NotTo(HaveOccurred())
			Expect(roleBinding).NotTo(BeNil())
			Expect(roleBinding.RoleRef.Name).To(Equal("foo"))

			Expect(k8sClient.Delete(ctx, roleBinding)).Should(Succeed())
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

			Expect(adapter.finalizeRelease()).To(Succeed())
			pipelineRun, err = adapter.loader.GetManagedReleasePipelineRun(adapter.ctx, adapter.client, adapter.release)
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

	When("registerProcessingData is called", func() {
		var adapter *adapter

		AfterEach(func() {
			_ = adapter.client.Delete(ctx, adapter.release)
		})

		BeforeEach(func() {
			adapter = createReleaseAndAdapter()
		})

		It("does nothing if there is no PipelineRun", func() {
			Expect(adapter.registerProcessingData(nil, nil)).To(Succeed())
			Expect(adapter.release.Status.Processing.PipelineRun).To(BeEmpty())
		})

		It("registers the Release processing data", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}
			roleBinding := &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role-binding",
					Namespace: "default",
				},
			}
			Expect(adapter.registerProcessingData(pipelineRun, roleBinding)).To(Succeed())
			Expect(adapter.release.Status.Processing.PipelineRun).To(Equal(fmt.Sprintf("%s%c%s",
				pipelineRun.Namespace, types.Separator, pipelineRun.Name)))
			Expect(adapter.release.Status.Processing.RoleBinding).To(Equal(fmt.Sprintf("%s%c%s",
				roleBinding.Namespace, types.Separator, roleBinding.Name)))
			Expect(adapter.release.Status.Target).To(Equal(pipelineRun.Namespace))
			Expect(adapter.release.IsProcessing()).To(BeTrue())
		})

		It("does not set RoleBinding when no RoleBinding is passed", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pipeline-run",
					Namespace: "default",
				},
			}

			Expect(adapter.registerProcessingData(pipelineRun, nil)).To(Succeed())
			Expect(adapter.release.Status.Processing.RoleBinding).To(BeEmpty())
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
					metadata.AutoReleaseLabel: "true",
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
					ServiceAccount: "service-account",
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
