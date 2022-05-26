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

	"github.com/go-logr/logr"
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
	release *v1alpha1.Release
	logger  logr.Logger
	client  client.Client
	context context.Context
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
	if err != nil {
		return results.RequeueWithError(err)
	}

	if pipelineRun == nil {
		targetReleaseLink, err := a.getTargetReleaseLink()
		if err != nil {
			a.release.Status.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.updateStatus())
		}

		releaseStrategy, err := a.getReleaseStrategy(targetReleaseLink)
		if err != nil {
			a.release.Status.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.updateStatus())
		}

		applicationSnapshot, err := a.getApplicationSnapshot()
		if err != nil {
			a.release.Status.MarkInvalid(v1alpha1.ReleaseReasonValidationError, err.Error())
			return results.RequeueOnErrorOrStop(a.updateStatus())
		}

		pipelineRun, err = a.createReleasePipelineRun(releaseStrategy, applicationSnapshot)
		if err != nil {
			return results.RequeueWithError(err)
		}

		a.logger.Info("Created release PipelineRun",
			"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
	}

	if !a.release.HasStarted() {
		a.release.Status.ReleasePipelineRun = fmt.Sprintf("%s%c%s",
			pipelineRun.Namespace, types.Separator, pipelineRun.Name)
		a.release.Status.MarkRunning()

		return results.RequeueOnErrorOrContinue(a.updateStatus())
	}

	return results.ContinueProcessing()
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

// createReleasePipelineRun creates and returns a new release PipelineRun. The new PipelineRun will include owner
// annotations, so it triggers Release reconciles whenever it changes. The Pipeline information and the parameters to it
// will be extracted from the given ReleaseStrategy. The Release's ApplicationSnapshot will also be passed to the
// release PipelineRun.
func (a *Adapter) createReleasePipelineRun(releaseStrategy *v1alpha1.ReleaseStrategy, applicationSnapshot *v1alpha1.ApplicationSnapshot) (*v1beta1.PipelineRun, error) {
	pipelineRun := tekton.NewReleasePipelineRun(releaseStrategy.Name, releaseStrategy.Namespace).
		WithOwner(a.release).
		WithReleaseLabels(a.release.Name, a.release.Namespace).
		WithReleaseStrategy(releaseStrategy).
		WithApplicationSnapshot(applicationSnapshot).
		AsPipelineRun()

	err := a.client.Create(a.context, pipelineRun)
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
		if err != nil {
			return err
		}
	}

	a.logger.Info("Successfully finalized Release")

	return nil
}

// getApplicationSnapshot returns the ApplicationSnapshot referenced by the Release being processed. If the
// ApplicationSnapshot is not found or the Get operation failed, an error will be returned.
func (a *Adapter) getApplicationSnapshot() (*v1alpha1.ApplicationSnapshot, error) {
	applicationSnapshot := &v1alpha1.ApplicationSnapshot{}
	err := a.client.Get(a.context, types.NamespacedName{
		Name:      a.release.Spec.ApplicationSnapshot,
		Namespace: a.release.Namespace,
	}, applicationSnapshot)

	if err != nil {
		return nil, err
	}

	return applicationSnapshot, nil
}

// getReleaseLink returns the ReleaseLink referenced by the Release being processed. If the ReleaseLink is not found or
// the Get operation failed, an error will be returned.
func (a *Adapter) getReleaseLink() (*v1alpha1.ReleaseLink, error) {
	releaseLink := &v1alpha1.ReleaseLink{}
	err := a.client.Get(a.context, types.NamespacedName{
		Namespace: a.release.Namespace,
		Name:      a.release.Spec.ReleaseLink,
	}, releaseLink)

	if err != nil {
		return nil, err
	}

	return releaseLink, nil
}

// getReleasePipelineRun returns the PipelineRun referenced by the Release being processed or nil if it's not found.
// In the case the List operation fails, an error will be returned.
func (a *Adapter) getReleasePipelineRun() (*v1beta1.PipelineRun, error) {
	pipelineRuns := &v1beta1.PipelineRunList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			tekton.ReleaseNameLabel:      a.release.Name,
			tekton.ReleaseWorkspaceLabel: a.release.Namespace,
		},
	}

	err := a.client.List(a.context, pipelineRuns, opts...)
	if err != nil {
		return nil, err
	} else if len(pipelineRuns.Items) == 0 {
		return nil, nil
	}

	return &pipelineRuns.Items[0], nil
}

// getReleaseStrategy returns the ReleaseStrategy referenced by the given ReleaseLink. If the ReleaseStrategy is
// not found or the Get operation fails, an error will be returned.
func (a *Adapter) getReleaseStrategy(releaseLink *v1alpha1.ReleaseLink) (*v1alpha1.ReleaseStrategy, error) {
	releaseStrategy := &v1alpha1.ReleaseStrategy{}
	err := a.client.Get(a.context, types.NamespacedName{
		Name:      releaseLink.Spec.ReleaseStrategy,
		Namespace: releaseLink.Namespace,
	}, releaseStrategy)

	if err != nil {
		return nil, err
	}

	return releaseStrategy, nil
}

// getTargetReleaseLink returns the ReleaseLink targeted by ReleaseLink in the Release being processed. If a matching
// ReleaseLink is not found or the List operation fails, an error will be returned.
func (a *Adapter) getTargetReleaseLink() (*v1alpha1.ReleaseLink, error) {
	releaseLink, err := a.getReleaseLink()
	if err != nil {
		return nil, err
	}

	releaseLinks := &v1alpha1.ReleaseLinkList{}
	opts := []client.ListOption{
		client.InNamespace(releaseLink.Spec.Target),
		client.MatchingFields{"spec.target": releaseLink.Namespace},
	}

	err = a.client.List(a.context, releaseLinks, opts...)
	if err != nil {
		return nil, err
	}

	for _, foundReleaseLink := range releaseLinks.Items {
		if foundReleaseLink.Spec.Application == releaseLink.Spec.Application {
			return &foundReleaseLink, nil
		}
	}

	return nil, fmt.Errorf("no ReleaseLink found in target workspace '%s' with target '%s' and application '%s'",
		releaseLink.Spec.Target, releaseLink.Namespace, releaseLink.Spec.Application)
}

// registerReleasePipelineRunStatus updates the status of the Release being processed by monitoring the status of the
// associated release PipelineRun and setting the appropriate state in the Release. If the PipelineRun hasn't
// started/succeeded, no action will be taken.
func (a *Adapter) registerReleasePipelineRunStatus(pipelineRun *v1beta1.PipelineRun) error {
	if pipelineRun != nil && pipelineRun.IsDone() {
		a.release.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		condition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
		if condition.IsTrue() {
			a.release.Status.MarkSucceeded()
		} else {
			a.release.Status.MarkFailed(v1alpha1.ReleaseReasonPipelineFailed, condition.Message)
		}

		return a.updateStatus()
	}

	return nil
}

// updateStatus updates the status of the Release being processed.
func (a *Adapter) updateStatus() error {
	return a.client.Status().Update(a.context, a.release)
}
