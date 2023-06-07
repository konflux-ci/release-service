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
	"strings"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/gitops"
	"github.com/redhat-appstudio/release-service/loader"
	"github.com/redhat-appstudio/release-service/metadata"
	"github.com/redhat-appstudio/release-service/syncer"
	"github.com/redhat-appstudio/release-service/tekton"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/operator-goodies/reconciler"

	libhandler "github.com/operator-framework/operator-lib/handler"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
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

// EnsureReleaseIsCompleted is an operation that will ensure that a Release is completed (marked as released) when
// all required phases (e.g. deployment or processing) have been completed.
func (a *Adapter) EnsureReleaseIsCompleted() (reconciler.OperationResult, error) {
	// Do nothing if the release status has been already added
	if a.release.HasReleaseFinished() {
		return reconciler.ContinueProcessing()
	}

	// The processing has to complete for a Release to be completed
	if !a.release.HasProcessingFinished() {
		return reconciler.ContinueProcessing()
	}

	// The deployment has to complete if the environment field in the ReleasePlanAdmission is set
	releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
	if err == nil && releasePlanAdmission.Spec.Environment != "" && !a.release.HasDeploymentFinished() {
		return reconciler.ContinueProcessing()
	}

	patch := client.MergeFrom(a.release.DeepCopy())
	a.release.MarkReleased()
	return reconciler.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
}

// EnsureReleaseIsDeployed is an operation that will ensure that a SnapshotEnvironmentBinding
// associated to the Release being processed exists. Otherwise, it will create a new one.
func (a *Adapter) EnsureReleaseIsDeployed() (reconciler.OperationResult, error) {
	if !a.release.IsProcessed() || a.release.HasDeploymentFinished() || a.release.IsDeploying() {
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

	err = a.syncResources()
	if err != nil {
		return reconciler.RequeueWithError(err)
	}

	binding, err := a.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
	if err != nil {
		return reconciler.RequeueWithError(err)
	}

	a.logger.Info("Created/updated SnapshotEnvironmentBinding",
		"SnapshotEnvironmentBinding.Name", binding.Name, "SnapshotEnvironmentBinding.Namespace", binding.Namespace)

	return reconciler.RequeueOnErrorOrContinue(a.registerDeploymentData(binding, releasePlanAdmission))
}

// EnsureReleaseDeploymentIsTracked is an operation that will ensure that the SnapshotEnvironmentBinding
// Deployment status is tracked in the Release being processed.
func (a *Adapter) EnsureReleaseDeploymentIsTracked() (reconciler.OperationResult, error) {
	if !a.release.IsDeploying() || a.release.HasDeploymentFinished() {
		return reconciler.ContinueProcessing()
	}

	// Search for an existing binding
	binding, err := a.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(a.ctx, a.client, a.release)
	if err != nil {
		return reconciler.RequeueWithError(err)
	}

	// Do nothing if the release does not own the binding
	if binding.GetAnnotations()[libhandler.TypeAnnotation] != a.release.GetObjectKind().GroupVersionKind().GroupKind().String() ||
		binding.GetAnnotations()[libhandler.NamespacedNameAnnotation] != fmt.Sprintf("%s/%s", a.release.GetNamespace(), a.release.GetName()) {
		return reconciler.ContinueProcessing()
	}

	return reconciler.RequeueOnErrorOrContinue(a.registerDeploymentStatus(binding))
}

// EnsureReleaseIsRunning is an operation that will ensure that a Release has not finished already and that
// it is marked as releasing. If the Release has finished, no other operation after this one will be executed.
func (a *Adapter) EnsureReleaseIsRunning() (reconciler.OperationResult, error) {
	if a.release.HasReleaseFinished() {
		return reconciler.StopProcessing()
	}

	if !a.release.IsReleasing() {
		patch := client.MergeFrom(a.release.DeepCopy())
		a.release.MarkReleasing("")
		return reconciler.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
	}

	return reconciler.ContinueProcessing()
}

// EnsureReleaseIsProcessed is an operation that will ensure that a release PipelineRun associated to the Release
// being processed exists. Otherwise, it will create a new release PipelineRun.
func (a *Adapter) EnsureReleaseIsProcessed() (reconciler.OperationResult, error) {
	if a.release.HasProcessingFinished() {
		return reconciler.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release)
	if err != nil && !errors.IsNotFound(err) {
		return reconciler.RequeueWithError(err)
	}

	if pipelineRun == nil || !a.release.IsProcessing() {
		resources, err := a.loader.GetProcessingResources(a.ctx, a.client, a.release)
		if err != nil {
			return reconciler.RequeueWithError(err)
		}

		if pipelineRun == nil {
			pipelineRun, err = a.createReleasePipelineRun(resources.ReleaseStrategy, resources.EnterpriseContractPolicy, resources.Snapshot)
			if err != nil {
				return reconciler.RequeueWithError(err)
			}

			a.logger.Info("Created release PipelineRun",
				"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
		}

		return reconciler.RequeueOnErrorOrContinue(a.registerProcessingData(pipelineRun, resources.ReleaseStrategy))
	}

	return reconciler.ContinueProcessing()
}

// EnsureReleaseIsValid is an operation that will ensure that a Release is valid by checking all the resources needed
// to process it.
func (a *Adapter) EnsureReleaseIsValid() (reconciler.OperationResult, error) {
	patch := client.MergeFrom(a.release.DeepCopy())
	resources, err := a.loader.GetProcessingResources(a.ctx, a.client, a.release)

	if err != nil {
		if resources == nil || resources.ReleasePlanAdmission == nil || errors.IsNotFound(err) {
			a.release.MarkValidationFailed(err.Error())
			a.release.MarkReleaseFailed("Release validation failed")
			return reconciler.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
		}

		return reconciler.RequeueWithError(err)
	}

	err = a.validateAuthor()
	if err != nil {
		if strings.Contains(err.Error(), "automated not set in status") {
			return reconciler.RequeueWithError(err)
		}

		a.release.MarkValidationFailed(err.Error())
		a.release.MarkReleaseFailed("Author validation failed")
		return reconciler.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
	}

	a.release.MarkValidated()

	return reconciler.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
}

// EnsureReleaseProcessingIsTracked is an operation that will ensure that the release PipelineRun status is tracked
// in the Release being processed.
func (a *Adapter) EnsureReleaseProcessingIsTracked() (reconciler.OperationResult, error) {
	if !a.release.IsProcessing() || a.release.HasProcessingFinished() {
		return reconciler.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release)
	if err != nil {
		return reconciler.RequeueWithError(err)
	}
	if pipelineRun != nil {
		return reconciler.RequeueOnErrorOrContinue(a.registerProcessingStatus(pipelineRun))
	}

	return reconciler.ContinueProcessing()
}

// createReleasePipelineRun creates and returns a new release PipelineRun. The new PipelineRun will include owner
// annotations, so it triggers Release reconciles whenever it changes. The Pipeline information and the parameters to it
// will be extracted from the given ReleaseStrategy. The Release's Snapshot will also be passed to the release
// PipelineRun.
func (a *Adapter) createReleasePipelineRun(releaseStrategy *v1alpha1.ReleaseStrategy,
	enterpriseContractPolicy *ecapiv1alpha1.EnterpriseContractPolicy,
	snapshot *applicationapiv1alpha1.Snapshot) (*v1beta1.PipelineRun, error) {
	pipelineRun := tekton.NewReleasePipelineRun("release-pipelinerun", releaseStrategy.Namespace).
		WithObjectReferences(a.release).
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

// createSnapshotEnvironmentBinding creates or updates a SnapshotEnvironmentBinding for the Release being processed.
func (a *Adapter) createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	resources, err := a.loader.GetDeploymentResources(a.ctx, a.client, a.release, releasePlanAdmission)
	if err != nil {
		return nil, err
	}

	// The binding information needs to be updated no matter if it already exists or not
	binding := gitops.NewSnapshotEnvironmentBinding(resources.ApplicationComponents, resources.Snapshot, resources.Environment)

	// Search for an existing binding
	existingBinding, err := a.loader.GetSnapshotEnvironmentBinding(a.ctx, a.client, releasePlanAdmission)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if existingBinding == nil {
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
	} else {
		// We create the binding so if the owner reference is not already present, there must be a good reason for that
		patch := client.MergeFrom(existingBinding.DeepCopy())
		existingBinding.Spec = binding.Spec

		// Add owner annotations so the controller can watch for status updates to the binding and track them
		// in the release
		err = libhandler.SetOwnerAnnotations(a.release, existingBinding)
		if err != nil {
			return nil, err
		}

		return existingBinding, a.client.Patch(a.ctx, existingBinding, patch)
	}
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

// registerDeploymentData adds all the Release deployment information to its Status and marks it as processing.
func (a *Adapter) registerDeploymentData(snapshotEnvironmentBinding *applicationapiv1alpha1.SnapshotEnvironmentBinding,
	releasePlanAdmission *v1alpha1.ReleasePlanAdmission) error {
	if snapshotEnvironmentBinding == nil || releasePlanAdmission == nil {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	if releasePlanAdmission.Spec.Environment != "" {
		a.release.Status.Deployment.Environment = fmt.Sprintf("%s%c%s",
			releasePlanAdmission.Namespace, types.Separator, releasePlanAdmission.Spec.Environment)
	}

	a.release.Status.Deployment.SnapshotEnvironmentBinding = fmt.Sprintf("%s%c%s",
		snapshotEnvironmentBinding.Namespace, types.Separator, snapshotEnvironmentBinding.Name)

	a.release.MarkDeploying("")

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// registerDeploymentStatus updates the status of the Release being processed by monitoring the status of the
// associated SnapshotEnvironmentBinding and setting the appropriate state in the Release.
func (a *Adapter) registerDeploymentStatus(binding *applicationapiv1alpha1.SnapshotEnvironmentBinding) error {
	if binding == nil {
		return nil
	}

	condition := meta.FindStatusCondition(binding.Status.ComponentDeploymentConditions,
		applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed)
	if condition == nil {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	if condition.Status == metav1.ConditionTrue {
		a.release.MarkDeployed()
	} else {
		if condition.Reason == applicationapiv1alpha1.ComponentDeploymentConditionErrorOccurred {
			a.release.MarkDeploymentFailed(condition.Message)
			a.release.MarkReleaseFailed("Release deployment failed")
		} else {
			a.release.MarkDeploying(condition.Message)
		}
	}

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// registerProcessingData adds all the Release processing information to its Status and marks it as processing.
func (a *Adapter) registerProcessingData(releasePipelineRun *v1beta1.PipelineRun, releaseStrategy *v1alpha1.ReleaseStrategy) error {
	if releasePipelineRun == nil || releaseStrategy == nil {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	a.release.Status.Processing.PipelineRun = fmt.Sprintf("%s%c%s",
		releasePipelineRun.Namespace, types.Separator, releasePipelineRun.Name)
	a.release.Status.Processing.ReleaseStrategy = fmt.Sprintf("%s%c%s",
		releaseStrategy.Namespace, types.Separator, releaseStrategy.Name)
	a.release.Status.Target = releasePipelineRun.Namespace

	a.release.MarkProcessing("")

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// registerProcessingStatus updates the status of the Release being processed by monitoring the status of the
// associated release PipelineRun and setting the appropriate state in the Release. If the PipelineRun hasn't
// started/succeeded, no action will be taken.
func (a *Adapter) registerProcessingStatus(pipelineRun *v1beta1.PipelineRun) error {
	if pipelineRun != nil && pipelineRun.IsDone() {
		patch := client.MergeFrom(a.release.DeepCopy())

		condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
		if condition.IsTrue() {
			a.release.MarkProcessed()
		} else {
			a.release.MarkProcessingFailed(condition.Message)
			a.release.MarkReleaseFailed("Release processing failed")
		}

		return a.client.Status().Patch(a.ctx, a.release, patch)
	}

	return nil
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

// registerAttributionData updates the status of the Release being processed with the proper attribution author.
func (a *Adapter) registerAttributionData(releasePlan *v1alpha1.ReleasePlan) error {
	if a.release.IsAttributed() {
		return nil
	}

	var author string
	patch := client.MergeFrom(a.release.DeepCopy())

	if a.release.Labels[metadata.AutomatedLabel] == "true" {
		author = releasePlan.Labels[metadata.AuthorLabel]
		if author == "" {
			return fmt.Errorf("no author in the ReleasePlan found for automated release")
		}
		a.release.Status.Attribution.StandingAuthorization = true
	} else {
		author = a.release.Labels[metadata.AuthorLabel]
		if author == "" { // webhooks prevent this from happening but they could be disabled in some scenarios
			return fmt.Errorf("no author found for manual release")
		}
	}

	a.release.Status.Attribution.Author = author

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// validateAuthor attributes the release to a specific user and ensures that the user is valid in SSO.
func (a *Adapter) validateAuthor() error {
	if a.release.Labels[metadata.AutomatedLabel] == "true" && !a.release.IsAutomated() {
		return fmt.Errorf("automated not set in status for automated release")
	}

	releasePlan, err := a.loader.GetReleasePlan(a.ctx, a.client, a.release)
	if err != nil {
		return err
	}

	return a.registerAttributionData(releasePlan)
}
