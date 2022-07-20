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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"github.com/kcp-dev/logicalcluster/v2"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
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

		controllerutil.RemoveFinalizer(a.release, finalizerName)
		err := a.client.Update(a.context, a.release)
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
		controllerutil.AddFinalizer(a.release, finalizerName)
		err := a.client.Update(a.context, a.release)

		return results.RequeueOnErrorOrContinue(err)
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

	var releaseStrategy *v1alpha1.ReleaseStrategy

	if pipelineRun == nil {
		releaseStrategy, err = a.getReleaseStrategyFromRelease()
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
		}

		applicationSnapshot, err := a.getApplicationSnapshot()
		if err != nil {
			patch := client.MergeFrom(a.release.DeepCopy())
			a.release.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.client.Status().Patch(a.context, a.release, patch))
		}

		pipelineRun, err = a.createReleasePipelineRun(releaseStrategy, applicationSnapshot)
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

// EnsureTargetContextIsSet is an operation that will ensure that the targetContext of the adapter is not nil. It will
// set it to a context from the workspace in the ReleasePlan if one is defined or to its own context if not.
func (a *Adapter) EnsureTargetContextIsSet() (results.OperationResult, error) {
	if a.targetContext == nil {
		releasePlan, err := a.getReleasePlan()
		if err != nil {
			return results.RequeueWithError(err)
		}

		if releasePlan.Spec.Target.Workspace == "" {
			a.logger.Info("Running in a traditional kubernetes environment")
			a.targetContext = a.context
		} else {
			a.logger.Info("Running in a KCP environment")
			a.targetContext = logicalcluster.WithCluster(a.context, logicalcluster.New(releasePlan.Spec.Target.Workspace))
		}
	}
	return results.ContinueProcessing()
}

// createReleasePipelineRun creates and returns a new release PipelineRun. The new PipelineRun will include owner
// annotations, so it triggers Release reconciles whenever it changes. The Pipeline information and the parameters to it
// will be extracted from the given ReleaseStrategy. The Release's ApplicationSnapshot will also be passed to the
// release PipelineRun.
func (a *Adapter) createReleasePipelineRun(releaseStrategy *v1alpha1.ReleaseStrategy, applicationSnapshot *appstudioshared.ApplicationSnapshot) (*v1beta1.PipelineRun, error) {
	pipelineRun := tekton.NewReleasePipelineRun("release-pipelinerun", releaseStrategy.Namespace).
		WithOwner(a.release).
		WithReleaseLabels(a.release.Name, a.release.Namespace, a.release.GetAnnotations()[logicalcluster.AnnotationKey]).
		WithReleaseStrategy(releaseStrategy).
		WithApplicationSnapshot(applicationSnapshot).
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

// getApplicationSnapshot returns the ApplicationSnapshot referenced by the Release being processed. If the
// ApplicationSnapshot is not found or the Get operation failed, an error will be returned.
func (a *Adapter) getApplicationSnapshot() (*appstudioshared.ApplicationSnapshot, error) {
	applicationSnapshot := &appstudioshared.ApplicationSnapshot{}
	err := a.client.Get(a.context, types.NamespacedName{
		Name:      a.release.Spec.ApplicationSnapshot,
		Namespace: a.release.Namespace,
	}, applicationSnapshot)

	if err != nil {
		return nil, err
	}

	return applicationSnapshot, nil
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

// getReleaseStrategyFromRelease is a utility function to get a ReleaseStrategy from the information contained in the
// Release. The function will get the ReleasePlanAdmission targeted by the ReleasePlan and then call getReleaseStrategy.
func (a *Adapter) getReleaseStrategyFromRelease() (*v1alpha1.ReleaseStrategy, error) {
	releasePlanAdmission, err := a.getActiveReleasePlanAdmission()
	if err != nil {
		return nil, err
	}

	return a.getReleaseStrategy(releasePlanAdmission)
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
