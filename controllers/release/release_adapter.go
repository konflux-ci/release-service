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
	"fmt"
	"github.com/redhat-appstudio/release-service/loader"
	"strings"
	"time"

	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"github.com/redhat-appstudio/release-service/gitops"
	"github.com/redhat-appstudio/release-service/syncer"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/go-logr/logr"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"github.com/redhat-appstudio/operator-goodies/reconciler"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Adapter holds the objects needed to reconcile a Release.
type Adapter struct {
	client  client.Client
	ctx     context.Context
	loader  loader.ObjectLoader
	logger  logr.Logger
	release *v1alpha1.Release
	syncer  *syncer.Syncer
}

// finalizerName is the finalizer name to be added to the Releases
const finalizerName string = "appstudio.redhat.com/release-finalizer"

// NewAdapter creates and returns an Adapter instance.
func NewAdapter(ctx context.Context, client client.Client, release *v1alpha1.Release, loader loader.ObjectLoader, logger logr.Logger) *Adapter {
	return &Adapter{
		client:  client,
		ctx:     ctx,
		loader:  loader,
		logger:  logger,
		release: release,
		syncer:  syncer.NewSyncerWithContext(client, logger, ctx),
	}
}

// EnsureFinalizersAreCalled is an operation that will ensure that finalizers are called whenever the Release being
// processed is marked for deletion. Once finalizers get called, the finalizer will be removed and the Release will go
// back to the queue, so it gets deleted. If a finalizer function fails its execution or a finalizer fails to be removed,
// the Release will be requeued with the error attached.
func (a *Adapter) EnsureFinalizersAreCalled() (reconciler.OperationResult, error) {
	// Check if the Release is marked for deletion and continue processing other operations otherwise
	if a.release.GetDeletionTimestamp() == nil {
		return reconciler.ContinueProcessing()
	}

	if controllerutil.ContainsFinalizer(a.release, finalizerName) {
		if err := a.finalizeRelease(); err != nil {
			return reconciler.RequeueWithError(err)
		}

		patch := client.MergeFrom(a.release.DeepCopy())
		controllerutil.RemoveFinalizer(a.release, finalizerName)
		err := a.client.Patch(a.ctx, a.release, patch)
		if err != nil {
			return reconciler.RequeueWithError(err)
		}
	}

	// Requeue the release again so it gets deleted and other operations are not executed
	return reconciler.Requeue()
}

// EnsureFinalizerIsAdded is an operation that will ensure that the Release being processed contains a finalizer.
func (a *Adapter) EnsureFinalizerIsAdded() (reconciler.OperationResult, error) {
	var finalizerFound bool
	for _, finalizer := range a.release.GetFinalizers() {
		if finalizer == finalizerName {
			finalizerFound = true
		}
	}

	if !finalizerFound {
		a.logger.Info("Adding Finalizer to the Release")
		patch := client.MergeFrom(a.release.DeepCopy())
		controllerutil.AddFinalizer(a.release, finalizerName)
		err := a.client.Patch(a.ctx, a.release, patch)

		return reconciler.RequeueOnErrorOrContinue(err)
	}

	return reconciler.ContinueProcessing()
}

// EnsureReleasePlanAdmissionEnabled is an operation that will ensure that the ReleasePlanAdmission is enabled.
// If it is not, no further operations will occur for this Release.
func (a *Adapter) EnsureReleasePlanAdmissionEnabled() (reconciler.OperationResult, error) {
	_, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
	if err != nil && strings.Contains(err.Error(), "multiple ReleasePlanAdmissions found") {
		patch := client.MergeFrom(a.release.DeepCopy())
		a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
		return reconciler.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
	}
	if err != nil && strings.Contains(err.Error(), "auto-release label set to false") {
		patch := client.MergeFrom(a.release.DeepCopy())
		a.release.MarkInvalid(v1alpha1.ReleaseReasonTargetDisabledError, err.Error())
		return reconciler.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
	}
	return reconciler.ContinueProcessing()
}

// EnsureReleasePipelineRunExists is an operation that will ensure that a release PipelineRun associated to the Release
// being processed exists. Otherwise, it will create a new release PipelineRun.
func (a *Adapter) EnsureReleasePipelineRunExists() (reconciler.OperationResult, error) {
	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release)
	if err != nil && !errors.IsNotFound(err) {
		return reconciler.RequeueWithError(err)
	}

	if pipelineRun == nil || !a.release.HasStarted() {
		releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonReleasePlanValidationError, err.Error())
			return reconciler.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
		}

		releaseStrategy, err := a.loader.GetReleaseStrategy(a.ctx, a.client, releasePlanAdmission)
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return reconciler.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
		}

		enterpriseContractPolicy, err := a.loader.GetEnterpriseContractPolicy(a.ctx, a.client, releaseStrategy)
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return reconciler.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
		}

		snapshot, err := a.loader.GetSnapshot(a.ctx, a.client, a.release)
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return reconciler.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
		}

		if pipelineRun == nil {
			pipelineRun, err = a.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
			if err != nil {
				return reconciler.RequeueWithError(err)
			}

			a.logger.Info("Created release PipelineRun",
				"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
		}

		return reconciler.RequeueOnErrorOrContinue(a.registerReleaseStatusData(pipelineRun, releaseStrategy))
	}

	return reconciler.ContinueProcessing()
}

// EnsureReleasePipelineStatusIsTracked is an operation that will ensure that the release PipelineRun status is tracked
// in the Release being processed.
func (a *Adapter) EnsureReleasePipelineStatusIsTracked() (reconciler.OperationResult, error) {
	if !a.release.HasStarted() || a.release.IsDone() {
		return reconciler.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release)
	if err != nil {
		return reconciler.RequeueWithError(err)
	}
	if pipelineRun != nil {
		return reconciler.RequeueOnErrorOrContinue(a.registerReleasePipelineRunStatus(pipelineRun))
	}

	return reconciler.ContinueProcessing()
}

// EnsureSnapshotEnvironmentBindingExists is an operation that will ensure that a SnapshotEnvironmentBinding
// associated to the Release being processed exists. Otherwise, it will create a new one.
func (a *Adapter) EnsureSnapshotEnvironmentBindingExists() (reconciler.OperationResult, error) {
	if !a.release.HasSucceeded() || a.release.HasBeenDeployed() {
		return reconciler.ContinueProcessing()
	}

	releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
	if err != nil {
		return reconciler.RequeueWithError(err)
	}

	// If no environment is set in the ReleasePlanAdmission, skip the Binding creation
	if releasePlanAdmission.Spec.Environment == "" {
		return reconciler.ContinueProcessing()
	}

	// Search for an existing binding
	binding, err := a.loader.GetSnapshotEnvironmentBinding(a.ctx, a.client, releasePlanAdmission)
	if err != nil && !errors.IsNotFound(err) {
		return reconciler.RequeueWithError(err)
	}

	if binding == nil {
		err = a.syncResources()
		if err != nil {
			return reconciler.RequeueWithError(err)
		}

		patch := client.MergeFrom(a.release.DeepCopy())

		binding, err := a.createSnapshotEnvironmentBinding(releasePlanAdmission)
		if err != nil {
			return reconciler.RequeueWithError(err)
		}

		a.logger.Info("Created SnapshotEnvironmentBinding",
			"SnapshotEnvironmentBinding.Name", binding.Name, "SnapshotEnvironmentBinding.Namespace", binding.Namespace)

		a.release.Status.SnapshotEnvironmentBinding = fmt.Sprintf("%s%c%s", binding.Namespace, types.Separator, binding.Name)

		return reconciler.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
	}

	return reconciler.ContinueProcessing()
}

// EnsureSnapshotEnvironmentBindingIsTracked is an operation that will ensure that the SnapshotEnvironmentBinding
// Deployment status is tracked in the Release being processed.
func (a *Adapter) EnsureSnapshotEnvironmentBindingIsTracked() (reconciler.OperationResult, error) {
	if !a.release.HasSucceeded() || a.release.Status.SnapshotEnvironmentBinding == "" || a.release.HasBeenDeployed() {
		return reconciler.ContinueProcessing()
	}

	// Search for an existing binding
	binding, err := a.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(a.ctx, a.client, a.release)
	if err != nil {
		return reconciler.RequeueWithError(err)
	}

	return reconciler.RequeueOnErrorOrContinue(a.registerGitOpsDeploymentStatus(binding))
}

// createReleasePipelineRun creates and returns a new release PipelineRun. The new PipelineRun will include owner
// annotations, so it triggers Release reconciles whenever it changes. The Pipeline information and the parameters to it
// will be extracted from the given ReleaseStrategy. The Release's Snapshot will also be passed to the release
// PipelineRun.
func (a *Adapter) createReleasePipelineRun(releaseStrategy *v1alpha1.ReleaseStrategy,
	enterpriseContractPolicy *ecapiv1alpha1.EnterpriseContractPolicy,
	snapshot *applicationapiv1alpha1.Snapshot) (*v1beta1.PipelineRun, error) {
	pipelineRun := tekton.NewReleasePipelineRun("release-pipelinerun", releaseStrategy.Namespace).
		WithOwner(a.release).
		WithReleaseAndApplicationMetadata(a.release, snapshot.Spec.Application).
		WithReleaseStrategy(releaseStrategy).
		WithEnterpriseContractPolicy(enterpriseContractPolicy).
		WithSnapshot(snapshot).
		AsPipelineRun()

	err := a.client.Create(a.ctx, pipelineRun)
	if err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

// createSnapshotEnvironmentBinding creates a SnapshotEnvironmentBinding for the Release being processed.
func (a *Adapter) createSnapshotEnvironmentBinding(releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	resources, err := a.loader.GetSnapshotEnvironmentBindingResources(a.ctx, a.client, a.release, releasePlanAdmission)
	if err != nil {
		return nil, err
	}

	binding := gitops.NewSnapshotEnvironmentBinding(resources.ApplicationComponents, resources.Snapshot, resources.Environment)

	// Set owner references so the binding is deleted if the application is deleted
	err = ctrl.SetControllerReference(resources.Application, binding, a.client.Scheme())
	if err != nil {
		return nil, err
	}

	// Add owner annotations so the controller can watch for status updates to the binding and track them
	// in the release
	err = libhandler.SetOwnerAnnotations(a.release, binding)
	if err != nil {
		return nil, err
	}

	return binding, a.client.Create(a.ctx, binding)
}

// finalizeRelease will finalize the Release being processed, removing the associated resources.
func (a *Adapter) finalizeRelease() error {
	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release)
	if err != nil {
		return err
	}

	if pipelineRun != nil {
		err = a.client.Delete(a.ctx, pipelineRun)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	a.logger.Info("Successfully finalized Release")

	return nil
}

// registerGitOpsDeploymentStatus updates the status of the Release being processed by monitoring the status of the
// associated SnapshotEnvironmentBinding and setting the appropriate state in the Release.
func (a *Adapter) registerGitOpsDeploymentStatus(binding *applicationapiv1alpha1.SnapshotEnvironmentBinding) error {
	if binding == nil {
		return nil
	}

	condition := meta.FindStatusCondition(binding.Status.ComponentDeploymentConditions,
		applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed)
	if condition == nil {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	if condition.Status == metav1.ConditionUnknown {
		a.release.MarkDeploying(condition.Reason, condition.Message)
	} else {
		a.release.MarkDeployed(condition.Status, condition.Reason, condition.Message)
	}

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// registerReleasePipelineRunStatus updates the status of the Release being processed by monitoring the status of the
// associated release PipelineRun and setting the appropriate state in the Release. If the PipelineRun hasn't
// started/succeeded, no action will be taken.
func (a *Adapter) registerReleasePipelineRunStatus(pipelineRun *v1beta1.PipelineRun) error {
	if pipelineRun != nil && pipelineRun.IsDone() {
		patch := client.MergeFrom(a.release.DeepCopy())

		a.release.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
		if condition.IsTrue() {
			a.release.MarkSucceeded()
		} else {
			a.release.MarkFailed(v1alpha1.ReleaseReasonPipelineFailed, condition.Message)
		}

		return a.client.Status().Patch(a.ctx, a.release, patch)
	}

	return nil
}

// registerReleaseStatusData adds all the Release information to its Status.
func (a *Adapter) registerReleaseStatusData(releasePipelineRun *v1beta1.PipelineRun, releaseStrategy *v1alpha1.ReleaseStrategy) error {
	if releasePipelineRun == nil || releaseStrategy == nil {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	a.release.Status.ReleasePipelineRun = fmt.Sprintf("%s%c%s",
		releasePipelineRun.Namespace, types.Separator, releasePipelineRun.Name)
	a.release.Status.ReleaseStrategy = fmt.Sprintf("%s%c%s",
		releaseStrategy.Namespace, types.Separator, releaseStrategy.Name)
	a.release.Status.Target = releasePipelineRun.Namespace

	a.release.MarkRunning()

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// syncResources sync all the resources needed to trigger the deployment of the Release being processed.
func (a *Adapter) syncResources() error {
	releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
	if err != nil {
		return err
	}

	snapshot, err := a.loader.GetSnapshot(a.ctx, a.client, a.release)
	if err != nil {
		return err
	}

	return a.syncer.SyncSnapshot(snapshot, releasePlanAdmission.Namespace)
}
