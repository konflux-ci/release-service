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
	"errors"
	"github.com/go-logr/logr"
	hasv1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/tekton"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	log := r.Log.WithValues("PipelineRun", req.NamespacedName)

	pipelineRun := &tektonv1beta1.PipelineRun{}
	err := r.Get(ctx, req.NamespacedName, pipelineRun)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Starting reconcile for new succeeded build")

	component, err := r.getComponent(ctx, pipelineRun)
	if err != nil {
		log.Error(err, "Component could not be loaded")
		return ctrl.Result{}, nil
	}

	r.triggerReleases(ctx, component)

	return ctrl.Result{}, nil
}

// getComponent loads from the cluster the Component referenced in the PipelineRun. It the PipelineRun doesn't specify
// a component or this is not found in the cluster, an error will be returned.
func (r *ReleaseReconciler) getComponent(ctx context.Context, pipelineRun *tektonv1beta1.PipelineRun) (*hasv1alpha1.Component, error) {
	if componentName, found := pipelineRun.Annotations["build.appstudio.openshift.io/component"]; found {
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

	return nil, errors.New("the pipeline had no component associated with it")
}

// getReleaseStrategies parses releases strategies linked to a given component and returns a slice with all
// the objects loaded from the cluster.
func (r *ReleaseReconciler) getReleaseStrategies(ctx context.Context, component *hasv1alpha1.Component) []*v1alpha1.ReleaseStrategy {
	var strategies []*v1alpha1.ReleaseStrategy

	for _, strategyName := range component.Spec.ReleaseStrategies {
		releaseStrategy := &v1alpha1.ReleaseStrategy{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: "redhat",
			Name:      strategyName,
		}, releaseStrategy)

		if err == nil {
			strategies = append(strategies, releaseStrategy)
		}
	}

	return strategies
}

// triggerReleases triggers a new release for each strategy referenced in the given component.
func (r *ReleaseReconciler) triggerReleases(ctx context.Context, component *hasv1alpha1.Component) {
	log := r.Log.WithValues("Component.Name", component.Name, "Component.Namespace", component.Namespace)

	log.Info("Fetching release strategies from component")
	if len(component.Spec.ReleaseStrategies) == 0 {
		log.Info("Component has not declared any release strategy",
			"Component.Spec.ReleaseStrategies", component.Spec.ReleaseStrategies)
		return
	}

	strategies := r.getReleaseStrategies(ctx, component)
	if len(strategies) == 0 {
		log.Info("No release strategy found in the cluster",
			"Component.Spec.ReleaseStrategies", component.Spec.ReleaseStrategies)
		return
	}

	for _, strategy := range strategies {
		log.Info("Triggering release", "ReleaseStrategy", strategy.Name)
		pipelineRun := tekton.CreatePipelineRunFromReleaseStrategy(strategy, component)
		err := r.Create(ctx, pipelineRun)
		if err != nil {
			log.Error(err, "Unable to trigger a release", "ReleaseStrategy.Name", strategy.Name)
		}
		log.Info("Release triggered",
			"PipelineRun.Name", pipelineRun.Name,
			"PipelineRun.Namespace", pipelineRun.Namespace)
	}
}

// SetupWithManager sets up the controller with the Manager which monitors PipelineRuns and filter them
// using a predicate to filter out everything but just succeeded Build PipelineRuns.
func (r *ReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tektonv1beta1.PipelineRun{}).
		WithEventFilter(tekton.BuildPipelineRunSucceededPredicate()).
		Complete(r)
}
