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
	"github.com/redhat-appstudio/operator-toolkit/controller"
	"github.com/redhat-appstudio/operator-toolkit/predicates"
	"github.com/redhat-appstudio/release-service/cache"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/go-logr/logr"
	libhandler "github.com/operator-framework/operator-lib/handler"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/gitops"
	"github.com/redhat-appstudio/release-service/loader"
	"github.com/redhat-appstudio/release-service/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Reconciler reconciles a Release object
type Reconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// NewReleaseReconciler creates and returns a Reconciler.
func NewReleaseReconciler(client client.Client, logger *logr.Logger, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Client: client,
		Log:    logger.WithName("release"),
		Scheme: scheme,
	}
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applications/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=enterprisecontractpolicies,verbs=get;list;watch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=enterprisecontractpolicies/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("Release", req.NamespacedName)

	release := &v1alpha1.Release{}
	err := r.Get(ctx, req.NamespacedName, release)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	adapter := NewAdapter(ctx, r.Client, release, loader.NewLoader(), logger)

	return controller.ReconcileHandler([]controller.Operation{
		adapter.EnsureFinalizersAreCalled,
		adapter.EnsureReleaseIsRunning,
		adapter.EnsureReleaseIsValid,
		adapter.EnsureFinalizerIsAdded,
		adapter.EnsureReleaseIsProcessed,
		adapter.EnsureReleaseProcessingIsTracked,
		adapter.EnsureReleaseIsDeployed,
		adapter.EnsureReleaseDeploymentIsTracked,
		adapter.EnsureReleaseIsCompleted,
	})
}

// Register registers the controller with the passed manager and log. This controller ignores Release status updates and
// also watches for PipelineRuns and SnapshotEnvironmentBindings that are created by the adapter and owned by the
// Releases so the owner gets reconciled on changes.
func (r *Reconciler) Register(manager ctrl.Manager, log *logr.Logger, _ cluster.Cluster) error {
	r.Client = manager.GetClient()
	r.Log = log.WithName("release")

	return ctrl.NewControllerManagedBy(manager).
		For(&v1alpha1.Release{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &applicationapiv1alpha1.SnapshotEnvironmentBinding{}}, &libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Kind:  "Release",
				Group: "appstudio.redhat.com",
			},
		}, builder.WithPredicates(predicates.GenerationUnchangedOnUpdatePredicate{}, gitops.DeploymentFinishedPredicate())).
		Watches(&source.Kind{Type: &v1beta1.PipelineRun{}}, &libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Kind:  "Release",
				Group: "appstudio.redhat.com",
			},
		}, builder.WithPredicates(tekton.ReleasePipelineRunSucceededPredicate())).
		Complete(r)
}

// SetupCache indexes fields for each of the resources used in the release adapter in those cases where filtering by
// field is required.
func (r *Reconciler) SetupCache(mgr ctrl.Manager) error {
	if err := cache.SetupComponentCache(mgr); err != nil {
		return err
	}

	if err := cache.SetupReleasePlanAdmissionCache(mgr); err != nil {
		return err
	}

	return cache.SetupSnapshotEnvironmentBindingCache(mgr)
}
