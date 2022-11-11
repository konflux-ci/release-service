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
	"time"

	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	"github.com/redhat-appstudio/release-service/gitops"
	"github.com/redhat-appstudio/release-service/kcp"
	"github.com/redhat-appstudio/release-service/syncer"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"github.com/kcp-dev/logicalcluster/v2"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/controllers/results"
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
	release       *v1alpha1.Release
	logger        logr.Logger
	client        client.Client
	context       context.Context
	syncer        *syncer.Syncer
	targetContext context.Context
}

// finalizerName is the finalizer name to be added to the Releases
const finalizerName string = "appstudio.redhat.com/release-finalizer"

// NewAdapter creates and returns an Adapter instance.
func NewAdapter(release *v1alpha1.Release, logger logr.Logger, client client.Client, context context.Context) *Adapter {
	return &Adapter{
		release: release,
		logger:  logger,
		client:  client,
		context: context,
		syncer:  syncer.NewSyncerWithContext(client, logger, context),
	}
}

// EnsureFinalizersAreCalled is an operation that will ensure that finalizers are called whenever the Release being
// processed is marked for deletion. Once finalizers get called, the finalizer will be removed and the Release will go
// back to the queue, so it gets deleted. If a finalizer function fails its execution or a finalizer fails to be removed,
// the Release will be requeued with the error attached.
func (a *Adapter) EnsureFinalizersAreCalled() (results.OperationResult, error) {
	// Check if the Release is marked for deletion and continue processing other operations otherwise
	if a.release.GetDeletionTimestamp() == nil {
		return results.ContinueProcessing()
	}

	if controllerutil.ContainsFinalizer(a.release, finalizerName) {
		if err := a.finalizeRelease(); err != nil {
			return results.RequeueWithError(err)
		}

		patch := client.MergeFrom(a.release.DeepCopy())
		controllerutil.RemoveFinalizer(a.release, finalizerName)
		err := a.client.Patch(a.context, a.release, patch)
		if err != nil {
			return results.RequeueWithError(err)
		}
	}

	// Requeue the release again so it gets deleted and other operations are not executed
	return results.Requeue()
}

// EnsureFinalizerIsAdded is an operation that will ensure that the Release being processed contains a finalizer.
func (a *Adapter) EnsureFinalizerIsAdded() (results.OperationResult, error) {
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
		err := a.client.Patch(a.context, a.release, patch)

		return results.RequeueOnErrorOrContinue(err)
	}

	return results.ContinueProcessing()
}

// EnsureReleasePlanAdmissionEnabled is an operation that will ensure that the ReleasePlanAdmission is enabled.
// If it is not, no further operations will occur for this Release.
func (a *Adapter) EnsureReleasePlanAdmissionEnabled() (results.OperationResult, error) {
	_, err := a.getActiveReleasePlanAdmission()
	if err != nil && strings.Contains(err.Error(), "auto-release label set to false") {
		patch := client.MergeFrom(a.release.DeepCopy())
		a.release.MarkInvalid(v1alpha1.ReleaseReasonTargetDisabledError, err.Error())
		return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
	}
	return results.ContinueProcessing()
}

// EnsureReleasePipelineRunExists is an operation that will ensure that a release PipelineRun associated to the Release
// being processed exists. Otherwise, it will create a new release PipelineRun.
func (a *Adapter) EnsureReleasePipelineRunExists() (results.OperationResult, error) {
	pipelineRun, err := a.getReleasePipelineRun()
	if err != nil && !errors.IsNotFound(err) {
		return results.RequeueWithError(err)
	}

	var (
		releasePlanAdmission *v1alpha1.ReleasePlanAdmission
		releaseStrategy      *v1alpha1.ReleaseStrategy
	)

	if pipelineRun == nil {
		releasePlanAdmission, err = a.getActiveReleasePlanAdmission()
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonReleasePlanValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
		}
		releaseStrategy, err = a.getReleaseStrategy(releasePlanAdmission)
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
		}
		enterpriseContractPolicy, err := a.getEnterpriseContractPolicy(releaseStrategy)
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
		}

		snapshot, err := a.getSnapshot()
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
		}

		pipelineRun, err = a.createReleasePipelineRun(releaseStrategy, enterpriseContractPolicy, snapshot)
		if err != nil {
			return results.RequeueWithError(err)
		}

		a.logger.Info("Created release PipelineRun",
			"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
	}

	return results.RequeueOnErrorOrContinue(a.registerReleaseStatusData(pipelineRun, releaseStrategy))
}

// EnsureReleasePipelineStatusIsTracked is an operation that will ensure that the release PipelineRun status is tracked
// in the Release being processed.
func (a *Adapter) EnsureReleasePipelineStatusIsTracked() (results.OperationResult, error) {
	if !a.release.HasStarted() || a.release.IsDone() {
		return results.ContinueProcessing()
	}

	pipelineRun, err := a.getReleasePipelineRun()
	if err != nil {
		return results.RequeueWithError(err)
	}
	if pipelineRun != nil {
		return results.RequeueOnErrorOrContinue(a.registerReleasePipelineRunStatus(pipelineRun))
	}

	return results.ContinueProcessing()
}

// EnsureSnapshotEnvironmentBindingIsCreated is an operation that will ensure that a SnapshotEnvironmentBinding is created
// or updated for the current Release.
func (a *Adapter) EnsureSnapshotEnvironmentBindingIsCreated() (results.OperationResult, error) {
	if !a.release.HasSucceeded() || a.release.HasBeenDeployed() {
		return results.ContinueProcessing()
	}

	releasePlanAdmission, err := a.getActiveReleasePlanAdmission()
	if err != nil {
		return results.RequeueWithError(err)
	}

	// If no environment is set in the ReleasePlanAdmission, skip the Binding creation
	if releasePlanAdmission.Spec.Environment == "" {
		return results.ContinueProcessing()
	}

	err = a.syncResources()
	if err != nil {
		return results.RequeueWithError(err)
	}

	binding, err := a.createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission)
	if err != nil {
		return results.RequeueWithError(err)
	}

	a.logger.Info("Created/updated SnapshotEnvironmentBinding",
		"SnapshotEnvironmentBinding.Name", binding.Name, "SnapshotEnvironmentBinding.Namespace", binding.Namespace)

	patch := client.MergeFrom(a.release.DeepCopy())
	a.release.Status.SnapshotEnvironmentBinding = fmt.Sprintf("%s%c%s", binding.Namespace, types.Separator, binding.Name)

	return results.RequeueOnErrorOrContinue(a.client.Status().Patch(a.context, a.release, patch))
}

// EnsureTargetContextIsSet is an operation that will ensure that the targetContext of the adapter is not nil. It will
// set it to a context from the workspace in the ReleasePlan if one is defined or to its own context if not.
func (a *Adapter) EnsureTargetContextIsSet() (results.OperationResult, error) {
	if a.release.Status.Target == (kcp.NamespaceReference{}) {
		patch := client.MergeFrom(a.release.DeepCopy())
		releasePlan, err := a.getReleasePlan()
		if err != nil {
			a.release.MarkInvalid(v1alpha1.ReleaseReasonReleasePlanValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
		}

		if _, ok := logicalcluster.ClusterFromContext(a.context); ok {
			a.logger.Info("Running in a KCP environment")

			if releasePlan.Spec.Target.Workspace == "" {
				a.release.MarkInvalid(v1alpha1.ReleaseReasonReleasePlanValidationError, "ReleasePlan Workspace is not set while running on KCP")
				return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
			}
		} else if releasePlan.Spec.Target.Workspace != "" {
			a.release.MarkInvalid(v1alpha1.ReleaseReasonReleasePlanValidationError, "ReleasePlan Workspace is set while not running on KCP")
			return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
		}

		a.release.Status.Target = releasePlan.Spec.Target
		err = a.client.Status().Patch(a.context, a.release, patch)
		if err != nil {
			return results.RequeueWithError(err)
		}
	}

	if a.release.Status.Target.Workspace == "" {
		a.targetContext = a.context
	} else {
		a.targetContext = logicalcluster.WithCluster(a.context, logicalcluster.New(a.release.Status.Target.Workspace))
		a.syncer.SetContext(a.targetContext)
	}

	return results.ContinueProcessing()
}

// createOrUpdateSnapshotEnvironmentBinding creates or updates a SnapshotEnvironmentBinding for the Release being
// processed.
func (a *Adapter) createOrUpdateSnapshotEnvironmentBinding(releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	application, components, snapshot, environment, err := a.getSnapshotEnvironmentResources(releasePlanAdmission)
	if err != nil {
		return nil, err
	}

	// The binding information needs to be updated no matter if it already exists or not
	binding := gitops.NewSnapshotEnvironmentBinding(components, snapshot, environment)

	// Search for an existing binding
	existingBinding, err := a.getSnapshotEnvironmentBinding(environment, releasePlanAdmission)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if existingBinding == nil {
		err = ctrl.SetControllerReference(application, binding, a.client.Scheme())
		if err != nil {
			return nil, err
		}
		return binding, a.client.Create(a.targetContext, binding)
	} else {
		// We create the binding so if the owner reference is not already present, there must be a good reason for that
		patch := client.MergeFrom(existingBinding.DeepCopy())
		existingBinding.Spec = binding.Spec
		return existingBinding, a.client.Patch(a.targetContext, existingBinding, patch)
	}
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

	err := a.client.Create(a.targetContext, pipelineRun)
	if err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

// finalizeRelease will finalize the Release being processed, removing the associated resources.
func (a *Adapter) finalizeRelease() error {
	pipelineRun, err := a.getReleasePipelineRun()
	if err != nil {
		return err
	}

	if pipelineRun != nil {
		err = a.client.Delete(a.context, pipelineRun)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	a.logger.Info("Successfully finalized Release")

	return nil
}

// getActiveReleasePlanAdmission returns the ReleasePlanAdmission targeted by the ReleasePlan in the Release being
// processed. Only ReleasePlanAdmissions with the 'auto-release' label set to true (or missing the label, which is
// treated the same as having the label and it being set to true) will be searched for. If a matching
// ReleasePlanAdmission is not found or the List operation fails, an error will be returned.
func (a *Adapter) getActiveReleasePlanAdmission() (*v1alpha1.ReleasePlanAdmission, error) {
	releasePlan, err := a.getReleasePlan()
	if err != nil {
		return nil, err
	}

	releasePlanAdmissions := &v1alpha1.ReleasePlanAdmissionList{}
	opts := []client.ListOption{
		client.InNamespace(releasePlan.Spec.Target.Namespace),
		client.MatchingFields{"spec.origin.namespace": releasePlan.Namespace},
	}

	err = a.client.List(a.targetContext, releasePlanAdmissions, opts...)
	if err != nil {
		return nil, err
	}

	activeReleasePlanAdmissionFound := false

	for _, releasePlanAdmission := range releasePlanAdmissions.Items {
		if releasePlanAdmission.Spec.Application == releasePlan.Spec.Application {
			labelValue, found := releasePlanAdmission.GetLabels()[v1alpha1.AutoReleaseLabel]
			if found && labelValue == "false" {
				return nil, fmt.Errorf("found ReleasePlanAdmission '%s' with auto-release label set to false",
					releasePlanAdmission.Name)
			}
			activeReleasePlanAdmissionFound = true
		}
	}

	if !activeReleasePlanAdmissionFound {
		return nil, fmt.Errorf("no ReleasePlanAdmission found in the target (%+v) for application '%s'",
			releasePlan.Spec.Target, releasePlan.Spec.Application)
	}

	return &releasePlanAdmissions.Items[0], nil
}

// getApplication returns the Application referenced by the ReleasePlanAdmission. If the Application is not found or
// the Get operation failed, an error will be returned.
func (a *Adapter) getApplication(releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.Application, error) {
	application := &applicationapiv1alpha1.Application{}
	err := a.client.Get(a.targetContext, types.NamespacedName{
		Name:      releasePlanAdmission.Spec.Application,
		Namespace: releasePlanAdmission.Namespace,
	}, application)

	if err != nil {
		return nil, err
	}

	return application, nil
}

// getApplicationComponents returns a list of all the Components associated with the given Application.
func (a *Adapter) getApplicationComponents(application *applicationapiv1alpha1.Application) ([]applicationapiv1alpha1.Component, error) {
	applicationComponents := &applicationapiv1alpha1.ComponentList{}
	opts := []client.ListOption{
		client.InNamespace(application.Namespace),
		client.MatchingFields{"spec.application": application.Name},
	}

	err := a.client.List(a.targetContext, applicationComponents, opts...)
	if err != nil {
		return nil, err
	}

	return applicationComponents.Items, nil
}

// getSnapshot returns the Snapshot referenced by the Release being processed. If the Snapshot
// is not found or the Get operation failed, an error is returned.
func (a *Adapter) getSnapshot() (*applicationapiv1alpha1.Snapshot, error) {
	snapshot := &applicationapiv1alpha1.Snapshot{}
	err := a.client.Get(a.context, types.NamespacedName{
		Name:      a.release.Spec.Snapshot,
		Namespace: a.release.Namespace,
	}, snapshot)

	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// getEnvironment returns the Environment referenced by the ReleasePlanAdmission used during this release. If the
// Environment is not found or the Get operation fails, an error will be returned.
func (a *Adapter) getEnvironment(releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.Environment, error) {
	environment := &applicationapiv1alpha1.Environment{}
	err := a.client.Get(a.targetContext, types.NamespacedName{
		Name:      releasePlanAdmission.Spec.Environment,
		Namespace: releasePlanAdmission.Namespace,
	}, environment)

	if err != nil {
		return nil, err
	}

	return environment, nil
}

// getReleasePlan returns the ReleasePlan referenced by the Release being processed. If the ReleasePlan is not found or
// the Get operation fails, an error will be returned.
func (a *Adapter) getReleasePlan() (*v1alpha1.ReleasePlan, error) {
	releasePlan := &v1alpha1.ReleasePlan{}
	err := a.client.Get(a.context, types.NamespacedName{
		Namespace: a.release.Namespace,
		Name:      a.release.Spec.ReleasePlan,
	}, releasePlan)
	if err != nil {
		return nil, err
	}

	return releasePlan, nil
}

// getReleasePipelineRun returns the PipelineRun referenced by the Release being processed or nil if it's not found.
// In the case the List operation fails, an error will be returned.
func (a *Adapter) getReleasePipelineRun() (*v1beta1.PipelineRun, error) {
	pipelineRuns := &v1beta1.PipelineRunList{}
	opts := []client.ListOption{
		client.Limit(1),
		client.MatchingLabels{
			tekton.ReleaseNameLabel:      a.release.Name,
			tekton.ReleaseNamespaceLabel: a.release.Namespace,
			tekton.ReleaseWorkspaceLabel: a.release.GetAnnotations()[logicalcluster.AnnotationKey],
		},
	}

	err := a.client.List(a.targetContext, pipelineRuns, opts...)
	if err == nil && len(pipelineRuns.Items) > 0 {
		return &pipelineRuns.Items[0], nil
	}

	return nil, err
}

// getReleaseStrategy returns the ReleaseStrategy referenced by the given ReleasePlanAdmission. If the ReleaseStrategy
// is not found or the Get operation fails, an error will be returned.
func (a *Adapter) getReleaseStrategy(releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*v1alpha1.ReleaseStrategy, error) {
	releaseStrategy := &v1alpha1.ReleaseStrategy{}
	err := a.client.Get(a.targetContext, types.NamespacedName{
		Name:      releasePlanAdmission.Spec.ReleaseStrategy,
		Namespace: releasePlanAdmission.Namespace,
	}, releaseStrategy)

	if err != nil {
		return nil, err
	}

	return releaseStrategy, nil
}

// getEnterpriseContractPolicy return the EnterpriseContractPolicy referenced by the given ReleaseStrategy.
func (a *Adapter) getEnterpriseContractPolicy(releaseStrategy *v1alpha1.ReleaseStrategy) (*ecapiv1alpha1.EnterpriseContractPolicy, error) {
	enterpriseContractPolicy := &ecapiv1alpha1.EnterpriseContractPolicy{}
	err := a.client.Get(a.targetContext, types.NamespacedName{
		Name:      releaseStrategy.Spec.Policy,
		Namespace: releaseStrategy.Namespace,
	}, enterpriseContractPolicy)

	if err != nil {
		return nil, err
	}

	return enterpriseContractPolicy, nil
}

// getSnapshotEnvironmentBinding returns the SnapshotEnvironmentBinding associated with the Release being processed.
// That association is defined by both the Environment and Application matching between the ReleasePlanAdmission and
// the SnapshotEnvironmentBinding. If the Get operation fails, an error will be returned.
func (a *Adapter) getSnapshotEnvironmentBinding(environment *applicationapiv1alpha1.Environment,
	releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	bindingList := &applicationapiv1alpha1.SnapshotEnvironmentBindingList{}
	opts := []client.ListOption{
		client.InNamespace(environment.Namespace),
		client.MatchingFields{"spec.environment": environment.Name},
	}

	err := a.client.List(a.targetContext, bindingList, opts...)
	if err != nil {
		return nil, err
	}

	for _, binding := range bindingList.Items {
		if binding.Spec.Application == releasePlanAdmission.Spec.Application {
			return &binding, nil
		}
	}

	return nil, nil
}

// getSnapshotEnvironmentResources returns all the resources required to create a SnapshotEnvironmentBinding. If any of
// those resources cannot be retrieved from the cluster, an error will be returned.
func (a *Adapter) getSnapshotEnvironmentResources(releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (
	*applicationapiv1alpha1.Application, []applicationapiv1alpha1.Component,
	*applicationapiv1alpha1.Snapshot, *applicationapiv1alpha1.Environment, error,
) {
	environment, err := a.getEnvironment(releasePlanAdmission)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	application, err := a.getApplication(releasePlanAdmission)
	if err != nil {
		return application, nil, nil, environment, err
	}

	components, err := a.getApplicationComponents(application)
	if err != nil {
		return application, nil, nil, environment, err
	}

	snapshot, err := a.getSnapshot()
	if err != nil {
		return application, components, nil, environment, err
	}

	return application, components, snapshot, environment, err
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

		return a.client.Status().Patch(a.context, a.release, patch)
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
	a.release.Status.Target.Namespace = releasePipelineRun.Namespace
	a.release.Status.Target.Workspace = releasePipelineRun.GetAnnotations()[logicalcluster.AnnotationKey]

	a.release.MarkRunning()

	return a.client.Status().Patch(a.context, a.release, patch)
}

// syncResources sync all the resources needed to trigger the deployment of the Release being processed.
func (a *Adapter) syncResources() error {
	releasePlanAdmission, err := a.getActiveReleasePlanAdmission()
	if err != nil {
		return err
	}

	snapshot, err := a.getSnapshot()
	if err != nil {
		return err
	}

	return a.syncer.SyncSnapshot(snapshot, releasePlanAdmission.Namespace)
}
