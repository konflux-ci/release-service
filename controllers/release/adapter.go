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
	"os"

	"github.com/redhat-appstudio/operator-toolkit/controller"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/gitops"
	"github.com/redhat-appstudio/release-service/loader"
	"github.com/redhat-appstudio/release-service/metadata"
	"github.com/redhat-appstudio/release-service/syncer"
	"github.com/redhat-appstudio/release-service/tekton"

	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	libhandler "github.com/operator-framework/operator-lib/handler"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// adapter holds the objects needed to reconcile a Release.
type adapter struct {
	client               client.Client
	ctx                  context.Context
	loader               loader.ObjectLoader
	logger               *logr.Logger
	release              *v1alpha1.Release
	releaseServiceConfig *v1alpha1.ReleaseServiceConfig
	syncer               *syncer.Syncer
	validations          []controller.ValidationFunction
}

// newAdapter creates and returns an adapter instance.
func newAdapter(ctx context.Context, client client.Client, release *v1alpha1.Release, loader loader.ObjectLoader, logger *logr.Logger) *adapter {
	releaseAdapter := &adapter{
		client:  client,
		ctx:     ctx,
		loader:  loader,
		logger:  logger,
		release: release,
		syncer:  syncer.NewSyncerWithContext(client, logger, ctx),
	}

	releaseAdapter.validations = []controller.ValidationFunction{
		releaseAdapter.validateProcessingResources,
		releaseAdapter.validateAuthor,
		releaseAdapter.validatePipelineRef,
	}

	return releaseAdapter
}

// EnsureConfigIsLoaded is an operation that will load the service ReleaseServiceConfig from the manager namespace. If not found,
// an empty ReleaseServiceConfig resource will be generated and attached to the adapter.
func (a *adapter) EnsureConfigIsLoaded() (controller.OperationResult, error) {
	namespace := os.Getenv("SERVICE_NAMESPACE")
	if namespace == "" {
		patch := client.MergeFrom(a.release.DeepCopy())
		a.release.MarkValidationFailed("SERVICE_NAMESPACE env var not set")
		a.release.MarkReleaseFailed("Release validation failed")
		return controller.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
	}

	var err error
	a.releaseServiceConfig, err = a.loader.GetReleaseServiceConfig(a.ctx, a.client, v1alpha1.ReleaseServiceConfigResourceName, namespace)
	if err != nil && !errors.IsNotFound(err) {
		return controller.RequeueWithError(err)
	}

	if err != nil {
		a.releaseServiceConfig = a.getEmptyReleaseServiceConfig(namespace)
	}

	return controller.ContinueProcessing()
}

// EnsureFinalizersAreCalled is an operation that will ensure that finalizers are called whenever the Release being
// processed is marked for deletion. Once finalizers get called, the finalizer will be removed and the Release will go
// back to the queue, so it gets deleted. If a finalizer function fails its execution or a finalizer fails to be removed,
// the Release will be requeued with the error attached.
func (a *adapter) EnsureFinalizersAreCalled() (controller.OperationResult, error) {
	// Check if the Release is marked for deletion and continue processing other operations otherwise
	if a.release.GetDeletionTimestamp() == nil {
		return controller.ContinueProcessing()
	}

	if controllerutil.ContainsFinalizer(a.release, metadata.ReleaseFinalizer) {
		if err := a.finalizeRelease(); err != nil {
			return controller.RequeueWithError(err)
		}

		patch := client.MergeFrom(a.release.DeepCopy())
		controllerutil.RemoveFinalizer(a.release, metadata.ReleaseFinalizer)
		err := a.client.Patch(a.ctx, a.release, patch)
		if err != nil {
			return controller.RequeueWithError(err)
		}
	}

	// Requeue the release again so it gets deleted and other operations are not executed
	return controller.Requeue()
}

// EnsureFinalizerIsAdded is an operation that will ensure that the Release being processed contains a finalizer.
func (a *adapter) EnsureFinalizerIsAdded() (controller.OperationResult, error) {
	var finalizerFound bool
	for _, finalizer := range a.release.GetFinalizers() {
		if finalizer == metadata.ReleaseFinalizer {
			finalizerFound = true
		}
	}

	if !finalizerFound {
		a.logger.Info("Adding Finalizer to the Release")
		patch := client.MergeFrom(a.release.DeepCopy())
		controllerutil.AddFinalizer(a.release, metadata.ReleaseFinalizer)
		err := a.client.Patch(a.ctx, a.release, patch)

		return controller.RequeueOnErrorOrContinue(err)
	}

	return controller.ContinueProcessing()
}

// EnsureReleaseIsCompleted is an operation that will ensure that a Release is completed (marked as released) when
// all required phases (e.g. deployment or processing) have been completed.
func (a *adapter) EnsureReleaseIsCompleted() (controller.OperationResult, error) {
	// Do nothing if the release status has been already added
	if a.release.HasReleaseFinished() {
		return controller.ContinueProcessing()
	}

	// The processing has to complete for a Release to be completed
	if !a.release.HasProcessingFinished() {
		return controller.ContinueProcessing()
	}

	// The deployment has to complete if the environment field in the ReleasePlanAdmission is set
	releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
	if err == nil && releasePlanAdmission.Spec.Environment != "" && !a.release.HasDeploymentFinished() {
		return controller.ContinueProcessing()
	}

	patch := client.MergeFrom(a.release.DeepCopy())
	a.release.MarkReleased()
	return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
}

// EnsureReleaseIsDeployed is an operation that will ensure that a SnapshotEnvironmentBinding
// associated to the Release being processed exists. Otherwise, it will create a new one.
func (a *adapter) EnsureReleaseIsDeployed() (controller.OperationResult, error) {
	if !a.release.IsProcessed() || a.release.HasDeploymentFinished() || a.release.IsDeploying() {
		return controller.ContinueProcessing()
	}

	releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
	if err != nil {
		return controller.RequeueWithError(err)
	}

	// If no environment is set in the ReleasePlanAdmission, skip the Binding creation
	if releasePlanAdmission.Spec.Environment == "" {
		return controller.ContinueProcessing()
	}

	err = a.syncResources()
	if err != nil {
		return controller.RequeueWithError(err)
	}

	binding, err := a.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
	if err != nil {
		return controller.RequeueWithError(err)
	}

	a.logger.Info("Created/updated SnapshotEnvironmentBinding",
		"SnapshotEnvironmentBinding.Name", binding.Name, "SnapshotEnvironmentBinding.Namespace", binding.Namespace)

	return controller.RequeueOnErrorOrContinue(a.registerDeploymentData(binding, releasePlanAdmission))
}

// EnsureReleaseDeploymentIsTracked is an operation that will ensure that the SnapshotEnvironmentBinding
// Deployment status is tracked in the Release being processed.
func (a *adapter) EnsureReleaseDeploymentIsTracked() (controller.OperationResult, error) {
	if !a.release.IsDeploying() || a.release.HasDeploymentFinished() {
		return controller.ContinueProcessing()
	}

	// Search for an existing binding
	binding, err := a.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(a.ctx, a.client, a.release)
	if err != nil {
		return controller.RequeueWithError(err)
	}

	// Do nothing if the release does not own the binding
	if binding.GetAnnotations()[libhandler.TypeAnnotation] != a.release.GetObjectKind().GroupVersionKind().GroupKind().String() ||
		binding.GetAnnotations()[libhandler.NamespacedNameAnnotation] != fmt.Sprintf("%s/%s", a.release.GetNamespace(), a.release.GetName()) {
		return controller.ContinueProcessing()
	}

	return controller.RequeueOnErrorOrContinue(a.registerDeploymentStatus(binding))
}

// EnsureReleaseIsRunning is an operation that will ensure that a Release has not finished already and that
// it is marked as releasing. If the Release has finished, no other operation after this one will be executed.
func (a *adapter) EnsureReleaseIsRunning() (controller.OperationResult, error) {
	if a.release.HasReleaseFinished() {
		return controller.StopProcessing()
	}

	if !a.release.IsReleasing() {
		patch := client.MergeFrom(a.release.DeepCopy())
		a.release.MarkReleasing("")
		return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
	}

	return controller.ContinueProcessing()
}

// EnsureReleaseIsProcessed is an operation that will ensure that a release PipelineRun associated to the Release
// being processed exists. Otherwise, it will create a new release PipelineRun.
func (a *adapter) EnsureReleaseIsProcessed() (controller.OperationResult, error) {
	if a.release.HasProcessingFinished() {
		return controller.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release)
	if err != nil && !errors.IsNotFound(err) {
		return controller.RequeueWithError(err)
	}

	if pipelineRun == nil || !a.release.IsProcessing() {
		resources, err := a.loader.GetProcessingResources(a.ctx, a.client, a.release)
		if err != nil {
			return controller.RequeueWithError(err)
		}

		if pipelineRun == nil {
			pipelineRun, err = a.createReleasePipelineRun(resources)
			if err != nil {
				return controller.RequeueWithError(err)
			}

			a.logger.Info("Created release PipelineRun",
				"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
		}

		return controller.RequeueOnErrorOrContinue(a.registerProcessingData(pipelineRun))
	}

	return controller.ContinueProcessing()
}

// EnsureReleaseIsValid is an operation that will ensure that a Release is valid by performing all
// validation checks.
func (a *adapter) EnsureReleaseIsValid() (controller.OperationResult, error) {
	patch := client.MergeFrom(a.release.DeepCopy())

	result := controller.Validate(a.validations...)
	if !result.Valid {
		if result.Err != nil {
			return controller.RequeueWithError(result.Err)
		}
		a.release.MarkReleaseFailed("Release validation failed")
	}

	// IsReleasing will be false if MarkReleaseFailed was called
	if a.release.IsReleasing() {
		a.release.MarkValidated()
		return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
	}

	return controller.RequeueOnErrorOrStop(a.client.Status().Patch(a.ctx, a.release, patch))
}

// EnsureReleaseProcessingIsTracked is an operation that will ensure that the release PipelineRun status is tracked
// in the Release being processed.
func (a *adapter) EnsureReleaseProcessingIsTracked() (controller.OperationResult, error) {
	if !a.release.IsProcessing() || a.release.HasProcessingFinished() {
		return controller.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release)
	if err != nil {
		return controller.RequeueWithError(err)
	}
	if pipelineRun != nil {
		err = a.registerProcessingStatus(pipelineRun)
		if err != nil {
			return controller.RequeueWithError(err)
		}
	}

	// This condition can only be true if the call to registerProcessingStatus changed the state
	if a.release.HasProcessingFinished() {
		// At this point it's safe to remove the PipelineRun, so the finalizer can be removed
		if controllerutil.ContainsFinalizer(pipelineRun, metadata.ReleaseFinalizer) {
			patch := client.MergeFrom(pipelineRun.DeepCopy())
			controllerutil.RemoveFinalizer(pipelineRun, metadata.ReleaseFinalizer)
			return controller.RequeueOnErrorOrContinue(a.client.Patch(a.ctx, pipelineRun, patch))
		}
	}

	return controller.ContinueProcessing()
}

// createReleasePipelineRun creates and returns a new release PipelineRun. The new PipelineRun will include owner
// annotations, so it triggers Release reconciles whenever it changes. The Pipeline information and the parameters to it
// will be extracted from the given ReleaseStrategy. The Release's Snapshot will also be passed to the release
// PipelineRun.
func (a *adapter) createReleasePipelineRun(resources *loader.ProcessingResources) (*tektonv1.PipelineRun, error) {
	pipelineRun := tekton.NewReleasePipelineRun("release-pipelinerun", resources.ReleasePlanAdmission.Namespace).
		WithObjectReferences(a.release, resources.ReleasePlan,
			resources.ReleasePlanAdmission, resources.Snapshot).
		WithOwner(a.release).
		WithReleaseAndApplicationMetadata(a.release, resources.Snapshot.Spec.Application).
		WithWorkspace(os.Getenv("DEFAULT_RELEASE_WORKSPACE_NAME"), os.Getenv("DEFAULT_RELEASE_PVC")).
		WithServiceAccount(resources.ReleasePlanAdmission.Spec.ServiceAccount).
		WithTimeout(resources.ReleasePlanAdmission.Spec.PipelineRef.Timeout).
		WithPipelineRef(resources.ReleasePlanAdmission.Spec.PipelineRef.ToTektonPipelineRef()).
		WithEnterpriseContractConfigMap(resources.EnterpriseContractConfigMap).
		WithEnterpriseContractPolicy(resources.EnterpriseContractPolicy).
		AsPipelineRun()

	err := a.client.Create(a.ctx, pipelineRun)
	if err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

// createSnapshotEnvironmentBinding creates or updates a SnapshotEnvironmentBinding for the Release being processed.
func (a *adapter) createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
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
func (a *adapter) finalizeRelease() error {
	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release)
	if err != nil {
		return err
	}

	if pipelineRun != nil {
		// The finalizer could still exist at this point in the case of the PipelineRun not having succeeded at the time
		// of finalizing the Release.
		if controllerutil.ContainsFinalizer(pipelineRun, metadata.ReleaseFinalizer) {
			patch := client.MergeFrom(pipelineRun.DeepCopy())
			removedFinalizer := controllerutil.RemoveFinalizer(pipelineRun, metadata.ReleaseFinalizer)
			if !removedFinalizer {
				return fmt.Errorf("finalizer not removed")
			}
			err := a.client.Patch(a.ctx, pipelineRun, patch)
			if err != nil {
				return err
			}
		}

		err = a.client.Delete(a.ctx, pipelineRun)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	a.logger.Info("Successfully finalized Release")

	return nil
}

// getEmptyReleaseServiceConfig creates and returns an empty ReleaseServiceConfig resource.
func (a *adapter) getEmptyReleaseServiceConfig(namespace string) *v1alpha1.ReleaseServiceConfig {
	return &v1alpha1.ReleaseServiceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.ReleaseServiceConfigResourceName,
			Namespace: namespace,
		},
	}
}

// registerDeploymentData adds all the Release deployment information to its Status and marks it as processing.
func (a *adapter) registerDeploymentData(snapshotEnvironmentBinding *applicationapiv1alpha1.SnapshotEnvironmentBinding,
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
func (a *adapter) registerDeploymentStatus(binding *applicationapiv1alpha1.SnapshotEnvironmentBinding) error {
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
func (a *adapter) registerProcessingData(releasePipelineRun *tektonv1.PipelineRun) error {
	if releasePipelineRun == nil {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	a.release.Status.Processing.PipelineRun = fmt.Sprintf("%s%c%s",
		releasePipelineRun.Namespace, types.Separator, releasePipelineRun.Name)
	a.release.Status.Target = releasePipelineRun.Namespace

	a.release.MarkProcessing("")

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// registerProcessingStatus updates the status of the Release being processed by monitoring the status of the
// associated release PipelineRun and setting the appropriate state in the Release. If the PipelineRun hasn't
// started/succeeded, no action will be taken.
func (a *adapter) registerProcessingStatus(pipelineRun *tektonv1.PipelineRun) error {
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
func (a *adapter) syncResources() error {
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

// validateAuthor will ensure that a valid author exists for the Release and add it to its status. If the Release
// has the automated label but doesn't have automated set in its status, this function will return an error so the
// operation knows to requeue the Release.
func (a *adapter) validateAuthor() *controller.ValidationResult {
	if a.release.IsAttributed() {
		return &controller.ValidationResult{Valid: true}
	}

	if a.release.Labels[metadata.AutomatedLabel] == "true" && !a.release.IsAutomated() {
		err := fmt.Errorf("automated not set in status for automated release")
		a.release.MarkValidationFailed(err.Error())
		return &controller.ValidationResult{Err: err}
	}

	releasePlan, err := a.loader.GetReleasePlan(a.ctx, a.client, a.release)
	if err != nil {
		if errors.IsNotFound(err) {
			a.release.MarkValidationFailed(err.Error())
			return &controller.ValidationResult{Valid: false}
		}
		return &controller.ValidationResult{Err: err}
	}

	var author string

	if a.release.Labels[metadata.AutomatedLabel] == "true" {
		author = releasePlan.Labels[metadata.AuthorLabel]
		if author == "" {
			a.release.MarkValidationFailed("no author in the ReleasePlan found for automated release")
			return &controller.ValidationResult{Valid: false}
		}
		a.release.Status.Attribution.StandingAuthorization = true
	} else {
		author = a.release.Labels[metadata.AuthorLabel]
		if author == "" { // webhooks prevent this from happening but they could be disabled in some scenarios
			a.release.MarkValidationFailed("no author found for manual release")
			return &controller.ValidationResult{Valid: false}
		}
	}

	a.release.Status.Attribution.Author = author
	return &controller.ValidationResult{Valid: true}
}

// validateProcessingResources will ensure that all the resources needed to process the Release exist.
func (a *adapter) validateProcessingResources() *controller.ValidationResult {
	resources, err := a.loader.GetProcessingResources(a.ctx, a.client, a.release)
	if err != nil {
		if resources == nil || resources.ReleasePlanAdmission == nil || errors.IsNotFound(err) {
			a.release.MarkValidationFailed(err.Error())
			return &controller.ValidationResult{Valid: false}
		}

		return &controller.ValidationResult{Err: err}
	}
	return &controller.ValidationResult{Valid: true}
}

// validatePipelineRef checks that the release PipelineRun ref passes the checks from the ReleaseServiceConfig.
func (a *adapter) validatePipelineRef() *controller.ValidationResult {
	releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
	if err != nil {
		if errors.IsNotFound(err) {
			a.release.MarkValidationFailed(err.Error())
			return &controller.ValidationResult{Valid: false}
		}
		return &controller.ValidationResult{Err: err}
	}

	if !a.releaseServiceConfig.Spec.Debug && releasePlanAdmission.Spec.PipelineRef.IsClusterScoped() {
		a.release.MarkValidationFailed("tried using debug only options while debug mode is disabled in the ReleaseServiceConfig")
		return &controller.ValidationResult{Valid: false}
	}

	return &controller.ValidationResult{Valid: true}
}
