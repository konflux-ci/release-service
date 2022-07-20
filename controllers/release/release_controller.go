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

	"github.com/go-logr/logr"
	"github.com/kcp-dev/logicalcluster/v2"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/controllers/results"
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("Release", req.NamespacedName).WithValues("clusterName", req.ClusterName)

	if req.ClusterName != "" {
		ctx = logicalcluster.WithCluster(ctx, logicalcluster.New(req.ClusterName))
	}

	release := &v1alpha1.Release{}
	err := r.Get(ctx, req.NamespacedName, release)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	adapter := NewAdapter(release, logger, r.Client, ctx, nil)

	return r.ReconcileHandler(adapter)
}

// AdapterInterface is an interface defining all the operations that should be defined in a Release adapter.
type AdapterInterface interface {
	EnsureFinalizersAreCalled() (results.OperationResult, error)
	EnsureFinalizerIsAdded() (results.OperationResult, error)
	EnsureReleasePipelineRunExists() (results.OperationResult, error)
	EnsureReleasePipelineStatusIsTracked() (results.OperationResult, error)
	EnsureTargetContextIsSet() (results.OperationResult, error)
}

// ReconcileOperation defines the syntax of functions invoked by the ReconcileHandler
type ReconcileOperation func() (results.OperationResult, error)

// ReconcileHandler will invoke all the operations to be performed as part of a Release reconcile, managing the queue
// based on the operations' results.
func (r *Reconciler) ReconcileHandler(adapter AdapterInterface) (ctrl.Result, error) {
	operations := []ReconcileOperation{
		adapter.EnsureTargetContextIsSet,
		adapter.EnsureFinalizersAreCalled,
		adapter.EnsureFinalizerIsAdded,
		adapter.EnsureReleasePipelineRunExists,
		adapter.EnsureReleasePipelineStatusIsTracked,
	}

	for _, operation := range operations {
		result, err := operation()
		if err != nil || result.RequeueRequest {
			return ctrl.Result{RequeueAfter: result.RequeueDelay}, err
		}
		if result.CancelRequest {
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

// SetupController creates a new Release reconciler and adds it to the Manager.
func SetupController(manager ctrl.Manager, log *logr.Logger) error {
	return setupControllerWithManager(manager, NewReleaseReconciler(manager.GetClient(), log, manager.GetScheme()))
}

// setupCache adds a new index field to be able to search ReleaseLinks by target namespace.
func setupCache(mgr ctrl.Manager) error {
	releaseLinkTargetIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*v1alpha1.ReleaseLink).Spec.Target.Namespace}
	}

	return mgr.GetCache().IndexField(context.Background(), &v1alpha1.ReleaseLink{},
		"spec.target.namespace", releaseLinkTargetIndexFunc)
}

// setupControllerWithManager sets up the controller with the Manager which monitors new Releases and filters out
// status updates. This controller also watches for PipelineRuns created by this controller and owned by the Releases so
// the owner gets reconciled on PipelineRun changes.
func setupControllerWithManager(manager ctrl.Manager, reconciler *Reconciler) error {
	err := setupCache(manager)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(manager).
		For(&v1alpha1.Release{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &v1beta1.PipelineRun{}}, &libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Kind:  "Release",
				Group: "appstudio.redhat.com",
			},
		}, builder.WithPredicates(tekton.ReleasePipelineRunSucceededPredicate())).
		Complete(reconciler)
}
