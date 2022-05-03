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

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/helpers"
	"github.com/redhat-appstudio/release-service/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const releaseFinalizer = "appstudio.redhat.com/finalizer"

// ReleaseReconciler reconciles a Release object
type ReleaseReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Release", req.NamespacedName)

	release := &v1alpha1.Release{}
	err := r.Get(ctx, req.NamespacedName, release)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Release resource not found")

			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get Release")

		return ctrl.Result{}, err
	}

	isReleaseMarkedToBeDeleted := release.GetDeletionTimestamp() != nil
	if isReleaseMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(release, releaseFinalizer) {
			if err := r.finalizeRelease(ctx, log, release); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(release, releaseFinalizer)
			err := r.Update(ctx, release)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(release, releaseFinalizer) {
		controllerutil.AddFinalizer(release, releaseFinalizer)
		err = r.Update(ctx, release)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if release.Status.StartTime == nil {
		release.Status.SetCreatedCondition()

		_, err = helpers.UpdateStatus(r.Client, ctx, release)
		if err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Created", "Conditions", release.Status.Conditions)
	}

	pipelineRuns := &v1beta1.PipelineRunList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			"release.appstudio.openshift.io/release":   release.Name,
			"release.appstudio.openshift.io/workspace": release.Namespace,
		},
	}

	err = r.List(ctx, pipelineRuns, opts...)
	if err != nil {
		return ctrl.Result{}, err
	} else if len(pipelineRuns.Items) == 0 {
		return r.createReleasePipelineRun(ctx, release)
	}

	release.Status.TrackReleasePipelineStatus(&pipelineRuns.Items[0])

	return helpers.UpdateStatus(r.Client, ctx, release)
}

// createReleasePipelineRun triggers a new Release Pipeline using the information provided in the given Release.
func (r *ReleaseReconciler) createReleasePipelineRun(ctx context.Context, release *v1alpha1.Release) (ctrl.Result, error) {
	log := r.Log.WithValues("Release.Name", release.Name, "Release.Namespace", release.Namespace)

	log.Info("Creating release pipeline run")

	releaseLink, err := r.getReleaseLink(ctx, release)
	if err != nil {
		log.Error(err, "Failed to get ReleaseLink")
		release.Status.SetErrorCondition(err)

		return helpers.UpdateStatus(r.Client, ctx, release)
	}

	targetReleaseLink, err := r.getTargetReleaseLink(ctx, releaseLink)
	if err != nil {
		log.Error(err, "Failed to find a matching ReleaseLink in target workspace",
			"ReleaseLink.Target", releaseLink.Spec.Target)
		release.Status.SetErrorCondition(err)

		return helpers.UpdateStatus(r.Client, ctx, release)
	}

	releaseStrategy, err := r.getReleaseStrategy(ctx, targetReleaseLink)
	if err != nil {
		log.Error(err, "Failed to get ReleaseStrategy")
		release.Status.SetErrorCondition(err)

		return helpers.UpdateStatus(r.Client, ctx, release)
	}

	log.Info("Triggering release", "ReleaseStrategy", releaseStrategy.Name)

	pipelineRun := tekton.CreatePipelineRunFromReleaseStrategy(releaseStrategy, releaseLink.Spec.Target, release)
	err = libhandler.SetOwnerAnnotations(release, pipelineRun)
	if err != nil {
		log.Error(err, "Failed to set Release Pipeline ownership",
			"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)
		return ctrl.Result{}, err
	}
	err = r.Create(ctx, pipelineRun)
	if err != nil {
		log.Error(err, "Unable to trigger a Release Pipeline", "ReleaseStrategy.Name", releaseStrategy.Name)
		release.Status.SetErrorCondition(err)

		return helpers.UpdateStatus(r.Client, ctx, release)
	}

	log.Info("Release triggered",
		"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)

	return ctrl.Result{}, nil
}

// getReleaseLink loads and returns the ReleaseLink referenced in the given Release.
func (r *ReleaseReconciler) getReleaseLink(ctx context.Context, release *v1alpha1.Release) (*v1alpha1.ReleaseLink, error) {
	releaseLink := &v1alpha1.ReleaseLink{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: release.Namespace,
		Name:      release.Spec.ReleaseLink,
	}, releaseLink)

	if err != nil {
		return nil, err
	}

	return releaseLink, nil
}

// getReleaseStrategy loads and returns the ReleaseStrategy referenced in the given ReleaseLink.
func (r *ReleaseReconciler) getReleaseStrategy(ctx context.Context, releaseLink *v1alpha1.ReleaseLink) (*v1alpha1.ReleaseStrategy, error) {
	releaseStrategy := &v1alpha1.ReleaseStrategy{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      releaseLink.Spec.ReleaseStrategy,
		Namespace: releaseLink.Namespace,
	}, releaseStrategy)

	if err != nil {
		return nil, err
	}

	return releaseStrategy, nil
}

// getTargetReleaseLink gets a ReleaseLink by following the spec.target field in a given ReleaseLink and returning
// it only if it targets the passed ReleaseLink and they both reference the same Application.
func (r *ReleaseReconciler) getTargetReleaseLink(ctx context.Context, releaseLink *v1alpha1.ReleaseLink) (*v1alpha1.ReleaseLink, error) {
	releaseLinks := &v1alpha1.ReleaseLinkList{}
	opts := []client.ListOption{
		client.InNamespace(releaseLink.Spec.Target),
		client.MatchingFields{"spec.target": releaseLink.Namespace},
	}

	if err := r.List(ctx, releaseLinks, opts...); err != nil {
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

// finalizeRelease removes all the CRs associated with the Release once the release is marked to be deleted.
func (r *ReleaseReconciler) finalizeRelease(ctx context.Context, log logr.Logger, release *v1alpha1.Release) error {
	pipelineRuns := &v1beta1.PipelineRunList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			"release.appstudio.openshift.io/release":   release.Name,
			"release.appstudio.openshift.io/workspace": release.Namespace,
		},
	}

	err := r.List(ctx, pipelineRuns, opts...)
	if err != nil {
		return err
	} else if len(pipelineRuns.Items) != 0 {
		return r.Delete(ctx, &pipelineRuns.Items[0])
	}

	log.Info("Successfully finalized memcached")

	return nil
}

// SetupWithManager sets up the controller with the Manager which monitors new Releases and filters out status updates.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Add a cache to be able to search for ReleaseLinks by target
	releaseLinkTargetIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*v1alpha1.ReleaseLink).Spec.Target}
	}
	if err := mgr.GetCache().IndexField(context.Background(), &v1alpha1.ReleaseLink{}, "spec.target", releaseLinkTargetIndexFunc); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Release{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &v1beta1.PipelineRun{}}, &libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Kind:  "Release",
				Group: "appstudio.redhat.com",
			},
		}).
		Complete(r)
}
