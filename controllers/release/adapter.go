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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/konflux-ci/operator-toolkit/controller"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/loader"
	"github.com/konflux-ci/release-service/metadata"
	"github.com/konflux-ci/release-service/syncer"
	"github.com/konflux-ci/release-service/tekton/utils"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	integrationgitops "github.com/redhat-appstudio/integration-service/gitops"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		releaseAdapter.validatePipelineDefined,
		releaseAdapter.validateProcessingResources,
		releaseAdapter.validateAuthor,
		releaseAdapter.validatePipelineSource,
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
		// call finalizeRelease in case Release is deleted before processing finishes
		if err := a.finalizeRelease(true); err != nil {
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

	// The managed pipeline processing has to complete for a Release to be completed
	if !a.release.HasManagedPipelineProcessingFinished() {
		return controller.ContinueProcessing()
	}

	patch := client.MergeFrom(a.release.DeepCopy())
	a.release.MarkReleased()
	return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
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

// EnsureTenantPipelineIsProcessed is an operation that will ensure that a Tenant Release PipelineRun associated to the Release
// being processed exist. Otherwise, it will be created.
func (a *adapter) EnsureTenantPipelineIsProcessed() (controller.OperationResult, error) {
	if a.release.HasTenantPipelineProcessingFinished() {
		return controller.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release, metadata.TenantPipelineType)
	if err != nil && !errors.IsNotFound(err) {
		return controller.RequeueWithError(err)
	}

	if pipelineRun == nil || !a.release.IsTenantPipelineProcessing() {
		releasePlan, err := a.loader.GetReleasePlan(a.ctx, a.client, a.release)
		if err != nil {
			return controller.RequeueWithError(err)
		}

		if releasePlan.Spec.Pipeline == nil {
			// no tenant pipeline to run
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkTenantPipelineProcessingSkipped()
			return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
		}

		if pipelineRun == nil {
			snapshot, err := a.loader.GetSnapshot(a.ctx, a.client, a.release)
			if err != nil {
				return controller.RequeueWithError(err)
			}

			pipelineRun, err = a.createTenantPipelineRun(releasePlan, snapshot)
			if err != nil {
				return controller.RequeueWithError(err)
			}

			a.logger.Info(fmt.Sprintf("Created %s Release PipelineRun", metadata.TenantPipelineType),
				"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
		}

		return controller.RequeueOnErrorOrContinue(a.registerTenantProcessingData(pipelineRun))
	}

	return controller.ContinueProcessing()
}

// EnsureManagedPipelineIsProcessed is an operation that will ensure that a managed Release PipelineRun associated to the Release
// being processed and a RoleBinding to grant its serviceAccount permissions exist. Otherwise, it will create them.
func (a *adapter) EnsureManagedPipelineIsProcessed() (controller.OperationResult, error) {
	if a.release.HasManagedPipelineProcessingFinished() || !a.release.HasTenantPipelineProcessingFinished() {
		return controller.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release, metadata.ManagedPipelineType)
	if err != nil && !errors.IsNotFound(err) {
		return controller.RequeueWithError(err)
	}

	roleBinding, _ := a.loader.GetRoleBindingFromReleaseStatus(a.ctx, a.client, a.release)
	if err != nil && !errors.IsNotFound(err) && !strings.Contains(err.Error(), "valid reference to a RoleBinding") {
		return controller.RequeueWithError(err)
	}

	if pipelineRun == nil || !a.release.IsManagedPipelineProcessing() {
		resources, err := a.loader.GetProcessingResources(a.ctx, a.client, a.release)
		if err != nil {
			if strings.Contains(err.Error(), "no ReleasePlanAdmissions can be found") {
				// No ReleasePlanAdmission, so no managed pipeline to run
				patch := client.MergeFrom(a.release.DeepCopy())
				a.release.MarkManagedPipelineProcessingSkipped()
				return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
			}
			return controller.RequeueWithError(err)
		}

		if pipelineRun == nil {
			if resources.ReleasePlanAdmission.Spec.Pipeline == nil {
				// no managed pipeline to run
				patch := client.MergeFrom(a.release.DeepCopy())
				a.release.MarkManagedPipelineProcessingSkipped()
				return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
			}

			// Only create a RoleBinding if a ServiceAccount is specified
			if roleBinding == nil && resources.ReleasePlanAdmission.Spec.Pipeline.ServiceAccountName != "" {
				// This string should probably be a constant somewhere
				roleBinding, err = a.createRoleBindingForClusterRole("release-pipeline-resource-role", resources.ReleasePlanAdmission)
				if err != nil {
					return controller.RequeueWithError(err)
				}
			}

			pipelineRun, err = a.createManagedPipelineRun(resources)
			if err != nil {
				return controller.RequeueWithError(err)
			}

			a.logger.Info(fmt.Sprintf("Created %s Release PipelineRun", metadata.ManagedPipelineType),
				"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
		}

		return controller.RequeueOnErrorOrContinue(a.registerManagedProcessingData(pipelineRun, roleBinding))
	}

	return controller.ContinueProcessing()
}

// EnsureReleaseExpirationTimeIsAdded is an operation that ensures that a Release has the ExpirationTime set.
func (a *adapter) EnsureReleaseExpirationTimeIsAdded() (controller.OperationResult, error) {
	if a.release.Status.ExpirationTime == nil {
		releasePlan, err := a.loader.GetReleasePlan(a.ctx, a.client, a.release)
		if err != nil && !errors.IsNotFound(err) {
			return controller.RequeueWithError(err)
		}

		patch := client.MergeFrom(a.release.DeepCopy())
		if a.release.Spec.GracePeriodDays == 0 {
			a.release.Spec.GracePeriodDays = releasePlan.Spec.ReleaseGracePeriodDays
		}
		a.release.SetExpirationTime(time.Duration(a.release.Spec.GracePeriodDays))

		return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.release, patch))
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

// EnsureTenantPipelineProcessingIsTracked is an operation that will ensure that the Release Tenant PipelineRun status
// is tracked in the Release being processed.
func (a *adapter) EnsureTenantPipelineProcessingIsTracked() (controller.OperationResult, error) {
	if !a.release.IsTenantPipelineProcessing() || a.release.HasTenantPipelineProcessingFinished() {
		return controller.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release, metadata.TenantPipelineType)
	if err != nil {
		return controller.RequeueWithError(err)
	}
	if pipelineRun != nil {
		err = a.registerTenantProcessingStatus(pipelineRun)
		if err != nil {
			return controller.RequeueWithError(err)
		}
	}

	return controller.ContinueProcessing()
}

// EnsureManagedPipelineProcessingIsTracked is an operation that will ensure that the Release Managed PipelineRun status
// is tracked in the Release being processed.
func (a *adapter) EnsureManagedPipelineProcessingIsTracked() (controller.OperationResult, error) {
	if !a.release.IsManagedPipelineProcessing() || a.release.HasManagedPipelineProcessingFinished() {
		return controller.ContinueProcessing()
	}

	pipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release, metadata.ManagedPipelineType)
	if err != nil {
		return controller.RequeueWithError(err)
	}
	if pipelineRun != nil {
		err = a.registerManagedProcessingStatus(pipelineRun)
		if err != nil {
			return controller.RequeueWithError(err)
		}
	}

	return controller.ContinueProcessing()
}

// EnsureReleaseProcessingResourcesAreCleanedUp is an operation that will ensure that the resources created for the Release
// Processing step are cleaned up once processing is finished. This exists in conjunction with EnsureFinalizersAreCalled because
// the finalizers should be removed from the pipelineRuns even if the Release is not marked for deletion for quota reasons.
func (a *adapter) EnsureReleaseProcessingResourcesAreCleanedUp() (controller.OperationResult, error) {
	if !a.release.HasTenantPipelineProcessingFinished() || !a.release.HasManagedPipelineProcessingFinished() {
		return controller.ContinueProcessing()
	}

	return controller.RequeueOnErrorOrContinue(a.finalizeRelease(false))
}

// cleanupProcessingResources removes the finalizer from the PipelineRun created for the Release Processing
// and removes the roleBinding that was created in order for the PipelineRun to succeed.
func (a *adapter) cleanupProcessingResources(pipelineRun *tektonv1.PipelineRun, roleBinding *rbac.RoleBinding) error {
	if roleBinding != nil {
		err := a.client.Delete(a.ctx, roleBinding)
		if err != nil {
			return err
		}
	}

	if pipelineRun != nil {
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
	}

	return nil
}

// createManagedPipelineRun creates and returns a new managed Release PipelineRun. The new PipelineRun will include owner
// annotations, so it triggers Release reconciles whenever it changes. The Pipeline information and the parameters to it
// will be extracted from the given ReleasePlanAdmission. The Release's Snapshot will also be passed to the release
// PipelineRun.
func (a *adapter) createManagedPipelineRun(resources *loader.ProcessingResources) (*tektonv1.PipelineRun, error) {
	pipelineRun, err := utils.NewPipelineRunBuilder(metadata.ManagedPipelineType, resources.ReleasePlanAdmission.Namespace).
		WithAnnotations(metadata.GetAnnotationsWithPrefix(a.release, integrationgitops.PipelinesAsCodePrefix)).
		WithFinalizer(metadata.ReleaseFinalizer).
		WithLabels(map[string]string{
			metadata.ApplicationNameLabel:  resources.ReleasePlan.Spec.Application,
			metadata.PipelinesTypeLabel:    metadata.ManagedPipelineType,
			metadata.ReleaseNameLabel:      a.release.Name,
			metadata.ReleaseNamespaceLabel: a.release.Namespace,
			metadata.ReleaseSnapshotLabel:  a.release.Spec.Snapshot,
		}).
		WithObjectReferences(a.release, resources.ReleasePlan, resources.ReleasePlanAdmission, a.releaseServiceConfig,
			resources.Snapshot).
		WithObjectSpecsAsJson(resources.EnterpriseContractPolicy).
		WithOwner(a.release).
		WithParamsFromConfigMap(resources.EnterpriseContractConfigMap, []string{"verify_ec_task_bundle"}).
		WithPipelineRef(resources.ReleasePlanAdmission.Spec.Pipeline.PipelineRef.ToTektonPipelineRef()).
		WithServiceAccount(resources.ReleasePlanAdmission.Spec.Pipeline.ServiceAccountName).
		WithTimeouts(&resources.ReleasePlanAdmission.Spec.Pipeline.Timeouts, &a.releaseServiceConfig.Spec.DefaultTimeouts).
		WithWorkspaceFromVolumeTemplate(
			os.Getenv("DEFAULT_RELEASE_WORKSPACE_NAME"),
			os.Getenv("DEFAULT_RELEASE_WORKSPACE_SIZE"),
		).
		Build()

	if err != nil {
		return nil, err
	}

	err = a.client.Create(a.ctx, pipelineRun)
	if err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

// createTenantPipelineRun creates and returns a new tenant Release PipelineRun. The new PipelineRun will include owner
// annotations, so it triggers Release reconciles whenever it changes. The Pipeline information and the parameters to it
// will be extracted from the given ReleasePlan. The Release's Snapshot will also be passed to the release
// PipelineRun.
func (a *adapter) createTenantPipelineRun(releasePlan *v1alpha1.ReleasePlan, snapshot *applicationapiv1alpha1.Snapshot) (*tektonv1.PipelineRun, error) {
	pipelineRun, err := utils.NewPipelineRunBuilder(metadata.TenantPipelineType, releasePlan.Namespace).
		WithAnnotations(metadata.GetAnnotationsWithPrefix(a.release, integrationgitops.PipelinesAsCodePrefix)).
		WithFinalizer(metadata.ReleaseFinalizer).
		WithLabels(map[string]string{
			metadata.ApplicationNameLabel:  releasePlan.Spec.Application,
			metadata.PipelinesTypeLabel:    metadata.TenantPipelineType,
			metadata.ReleaseNameLabel:      a.release.Name,
			metadata.ReleaseNamespaceLabel: a.release.Namespace,
			metadata.ReleaseSnapshotLabel:  a.release.Spec.Snapshot,
		}).
		WithObjectReferences(a.release, releasePlan, snapshot).
		WithParams(releasePlan.Spec.Pipeline.GetTektonParams()...).
		WithOwner(a.release).
		WithPipelineRef(releasePlan.Spec.Pipeline.PipelineRef.ToTektonPipelineRef()).
		WithServiceAccount(releasePlan.Spec.Pipeline.ServiceAccountName).
		WithTimeouts(&releasePlan.Spec.Pipeline.Timeouts, &a.releaseServiceConfig.Spec.DefaultTimeouts).
		WithWorkspaceFromVolumeTemplate(
			os.Getenv("DEFAULT_RELEASE_WORKSPACE_NAME"),
			os.Getenv("DEFAULT_RELEASE_WORKSPACE_SIZE"),
		).
		Build()

	if err != nil {
		return nil, err
	}

	err = a.client.Create(a.ctx, pipelineRun)
	if err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

// createRoleBindingForClusterRole creates a RoleBinding that binds the serviceAccount from the passed
// ReleasePlanAdmission to the passed ClusterRole. If the creation fails, the error is returned. If the creation
// is successful, the RoleBinding is returned.
func (a *adapter) createRoleBindingForClusterRole(clusterRole string, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*rbac.RoleBinding, error) {
	roleBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-rolebinding-for-%s-", a.release.Name, clusterRole),
			Namespace:    releasePlanAdmission.Spec.Origin,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      releasePlanAdmission.Spec.Pipeline.ServiceAccountName,
				Namespace: releasePlanAdmission.Namespace,
			},
		},
	}

	// Set ownerRef so it is deleted if the Release is deleted
	err := ctrl.SetControllerReference(a.release, roleBinding, a.client.Scheme())
	if err != nil {
		return nil, err
	}

	err = a.client.Create(a.ctx, roleBinding)
	if err != nil {
		return nil, err
	}

	return roleBinding, nil
}

// finalizeRelease will finalize the Release being processed, removing the associated resources. The pipelineRuns are optionally
// deleted so that EnsureReleaseProcessingResourcesAreCleanedUp can call this and just remove the finalizers, but
// EnsureFinalizersAreCalled will remove the finalizers and delete the pipelineRuns. If the pipelineRuns were deleted in
// EnsureReleaseProcessingResourcesAreCleanedUp, they could be removed before all the tracking data is saved.
func (a *adapter) finalizeRelease(delete bool) error {
	// Cleanup Tenant Processing Resources
	tenantPipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release, metadata.TenantPipelineType)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	err = a.cleanupProcessingResources(tenantPipelineRun, nil)
	if err != nil {
		return err
	}

	if delete && tenantPipelineRun != nil {
		err = a.client.Delete(a.ctx, tenantPipelineRun)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Cleanup Managed Processing Resources
	managedPipelineRun, err := a.loader.GetReleasePipelineRun(a.ctx, a.client, a.release, metadata.ManagedPipelineType)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	roleBinding, err := a.loader.GetRoleBindingFromReleaseStatus(a.ctx, a.client, a.release)
	if err != nil && !errors.IsNotFound(err) && !strings.Contains(err.Error(), "valid reference to a RoleBinding") {
		return err
	}

	err = a.cleanupProcessingResources(managedPipelineRun, roleBinding)
	if err != nil {
		return err
	}

	if delete && managedPipelineRun != nil {
		err = a.client.Delete(a.ctx, managedPipelineRun)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	a.logger.Info("Successfully finalized Release")

	return nil
}

// getEmptyReleaseServiceConfig creates and returns an empty ReleaseServiceConfig resource.
func (a *adapter) getEmptyReleaseServiceConfig(namespace string) *v1alpha1.ReleaseServiceConfig {
	releaseServiceConfig := &v1alpha1.ReleaseServiceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.ReleaseServiceConfigResourceName,
			Namespace: namespace,
		},
	}
	releaseServiceConfig.Kind = "ReleaseServiceConfig"
	return releaseServiceConfig
}

// registerTenantProcessingData adds all the Release Tenant processing information to its Status and marks it as tenant processing.
func (a *adapter) registerTenantProcessingData(releasePipelineRun *tektonv1.PipelineRun) error {
	if releasePipelineRun == nil {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	a.release.Status.TenantProcessing.PipelineRun = fmt.Sprintf("%s%c%s",
		releasePipelineRun.Namespace, types.Separator, releasePipelineRun.Name)

	a.release.MarkTenantPipelineProcessing()

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// registerProcessingData adds all the Release Managed processing information to its Status and marks it as managed processing.
func (a *adapter) registerManagedProcessingData(releasePipelineRun *tektonv1.PipelineRun, roleBinding *rbac.RoleBinding) error {
	if releasePipelineRun == nil {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	a.release.Status.ManagedProcessing.PipelineRun = fmt.Sprintf("%s%c%s",
		releasePipelineRun.Namespace, types.Separator, releasePipelineRun.Name)
	if roleBinding != nil {
		a.release.Status.ManagedProcessing.RoleBinding = fmt.Sprintf("%s%c%s",
			roleBinding.Namespace, types.Separator, roleBinding.Name)
	}

	a.release.MarkManagedPipelineProcessing()

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// registerTenantProcessingStatus updates the status of the Release being processed by monitoring the status of the
// associated tenant Release PipelineRun and setting the appropriate state in the Release. If the PipelineRun hasn't
// started/succeeded, no action will be taken.
func (a *adapter) registerTenantProcessingStatus(pipelineRun *tektonv1.PipelineRun) error {
	if pipelineRun == nil || !pipelineRun.IsDone() {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
	if condition.IsTrue() {
		a.release.MarkTenantPipelineProcessed()
	} else {
		a.release.MarkTenantPipelineProcessingFailed(condition.Message)
		a.release.MarkManagedPipelineProcessingSkipped() // Do not run managed pipeline if tenant pipeline fails
		a.release.MarkReleaseFailed("Release processing failed on tenant pipelineRun")
	}

	return a.client.Status().Patch(a.ctx, a.release, patch)
}

// registerManagedProcessingStatus updates the status of the Release being processed by monitoring the status of the
// associated managed Release PipelineRun and setting the appropriate state in the Release. If the PipelineRun hasn't
// started/succeeded, no action will be taken.
func (a *adapter) registerManagedProcessingStatus(pipelineRun *tektonv1.PipelineRun) error {
	if pipelineRun == nil || !pipelineRun.IsDone() {
		return nil
	}

	patch := client.MergeFrom(a.release.DeepCopy())

	condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
	if condition.IsTrue() {
		a.release.MarkManagedPipelineProcessed()
	} else {
		a.release.MarkManagedPipelineProcessingFailed(condition.Message)
		a.release.MarkReleaseFailed("Release processing failed on managed pipelineRun")
	}

	return a.client.Status().Patch(a.ctx, a.release, patch)
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
		if a.release.CreationTimestamp.Add(5 * time.Minute).Before(time.Now()) {
			return &controller.ValidationResult{Valid: false}
		}
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
	releasePlan, err := a.loader.GetReleasePlan(a.ctx, a.client, a.release)
	if err != nil {
		if errors.IsNotFound(err) {
			a.release.MarkValidationFailed(err.Error())
			return &controller.ValidationResult{Valid: false}
		}
		return &controller.ValidationResult{Err: err}
	}

	if releasePlan.Spec.Pipeline == nil {
		resources, err := a.loader.GetProcessingResources(a.ctx, a.client, a.release)
		if err != nil {
			if resources == nil || resources.ReleasePlan == nil || resources.ReleasePlanAdmission == nil || errors.IsNotFound(err) {
				a.release.MarkValidationFailed(err.Error())
				return &controller.ValidationResult{Valid: false}
			}

			return &controller.ValidationResult{Err: err}
		}
	}
	return &controller.ValidationResult{Valid: true}
}

// validatePipelineSource checks that the Release PipelineRun ref passes the checks from the ReleaseServiceConfig.
func (a *adapter) validatePipelineSource() *controller.ValidationResult {
	pipelineRef := utils.PipelineRef{}
	releasePlan, err := a.loader.GetReleasePlan(a.ctx, a.client, a.release)
	if err != nil {
		return a.validationError(err)
	}

	if releasePlan.Spec.Pipeline != nil {
		pipelineRef = releasePlan.Spec.Pipeline.PipelineRef
	} else {
		releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
		if err != nil {
			return a.validationError(err)
		}

		pipelineRef = releasePlanAdmission.Spec.Pipeline.PipelineRef
	}

	if !a.releaseServiceConfig.Spec.Debug && pipelineRef.IsClusterScoped() {
		a.release.MarkValidationFailed("tried using debug only options while debug mode is disabled in the ReleaseServiceConfig")
		return &controller.ValidationResult{Valid: false}
	}

	return &controller.ValidationResult{Valid: true}
}

// validatePipelineDefined checks that a Pipeline is defined in either the ReleasePlan or in the ReleasePlanAdmission.
func (a *adapter) validatePipelineDefined() *controller.ValidationResult {
	releasePlan, err := a.loader.GetReleasePlan(a.ctx, a.client, a.release)
	if err != nil {
		if errors.IsNotFound(err) {
			a.release.MarkValidationFailed(err.Error())
			return &controller.ValidationResult{Valid: false}
		}

		return &controller.ValidationResult{Err: err}
	}

	if releasePlan.Spec.Target == "" {
		a.release.Status.Target = releasePlan.Namespace
	} else {
		a.release.Status.Target = releasePlan.Spec.Target
	}

	if releasePlan.Spec.Pipeline == nil {
		if releasePlan.Spec.Target == "" {
			errString := "releasePlan has no pipeline or target. Each Release should define a tenant pipeline, managed pipeline, or both"
			a.release.MarkValidationFailed(errString)
			return &controller.ValidationResult{Valid: false}
		}
		releasePlanAdmission, err := a.loader.GetActiveReleasePlanAdmissionFromRelease(a.ctx, a.client, a.release)
		if err != nil {
			if errors.IsNotFound(err) {
				a.release.MarkValidationFailed(err.Error())
				return &controller.ValidationResult{Valid: false}
			}

			return &controller.ValidationResult{Err: err}
		}
		if releasePlanAdmission.Spec.Pipeline == nil {
			errString := "releasePlan and releasePlanAdmission both have no pipeline. Each Release should define a tenant pipeline, managed pipeline, or both"
			a.release.MarkValidationFailed(errString)
			return &controller.ValidationResult{Valid: false}
		}
	}

	return &controller.ValidationResult{Valid: true}
}

// validationError checks the error type, marks the release as failed when the error for known errors, and returns the
// ValidationResult for the error found.
func (a *adapter) validationError(err error) *controller.ValidationResult {
	if errors.IsNotFound(err) {
		a.release.MarkValidationFailed(err.Error())
		return &controller.ValidationResult{Valid: false}
	}
	return &controller.ValidationResult{Err: err}
}
