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
	hasv1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/tekton"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PipelineRunReconciler reconciles a PipelineRun object
type PipelineRunReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=pipelineruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=pipelineruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=pipelineruns/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PipelineRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("PipelineRun", req.NamespacedName)

	pipelineRun := &tektonv1beta1.PipelineRun{}
	err := r.Get(ctx, req.NamespacedName, pipelineRun)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("PipelineRun resource not found")

			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get PipelineRun")

		return ctrl.Result{}, err
	}

	return r.createRelease(ctx, pipelineRun)
}

// createRelease creates a new Release by using the information provided in the Build PipelineRun.
func (r *PipelineRunReconciler) createRelease(ctx context.Context, pipelineRun *tektonv1beta1.PipelineRun) (ctrl.Result, error) {
	log := r.Log.WithValues()

	log.Info("Creating new Release",
		"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)

	component, err := r.getComponent(ctx, pipelineRun)
	if err != nil {
		log.Error(err, "Failed to get Component",
			"PipelineRun.Name", pipelineRun.Name, "PipelineRun.Namespace", pipelineRun.Namespace)

		return ctrl.Result{}, nil
	}

	releaseLink, err := r.getReleaseLink(ctx, component)
	if err != nil {
		log.Error(err, "Failed to get ReleaseLink",
			"Component.Name", component.Name, "Component.Namespace", component.Namespace)

		return ctrl.Result{}, nil
	}

	release := v1alpha1.NewRelease(component, releaseLink)
	err = r.Create(ctx, release)
	if err != nil {
		log.Error(err, "Failed to create Release")

		// This is the only returned error so idempotency is achieved. If we need to return more errors,
		// we need to check if the release exists already.
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// getComponent loads from the cluster the Component referenced in the given PipelineRun. If the PipelineRun doesn't specify
// a Component or this is not found in the cluster, an error will be returned.
func (r *PipelineRunReconciler) getComponent(ctx context.Context, pipelineRun *tektonv1beta1.PipelineRun) (*hasv1alpha1.Component, error) {
	if componentName, found := pipelineRun.Labels["build.appstudio.openshift.io/component"]; found {
		component := &hasv1alpha1.Component{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: pipelineRun.Namespace,
			Name:      componentName,
		}, component)

		if err != nil {
			return nil, err
		}

		return component, nil
	}

	return nil, fmt.Errorf("the pipeline had no component associated with it")
}

// getReleaseLink is a dummy implementation of the function to get the ReleaseLink from the object that triggered an
// automated Release. This implementation should be updated as soon the object triggering the Release contains
// a reference to a ReleaseLink.
func (r *PipelineRunReconciler) getReleaseLink(ctx context.Context, component *hasv1alpha1.Component) (*v1alpha1.ReleaseLink, error) {
	releaseLink := &v1alpha1.ReleaseLink{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: component.Namespace,
		Name:      component.Namespace,
	}, releaseLink)

	if err != nil {
		return nil, err
	}

	return releaseLink, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tektonv1beta1.PipelineRun{}).
		WithEventFilter(tekton.BuildPipelineRunSucceededPredicate()).
		Complete(r)
}
