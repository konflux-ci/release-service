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

	"github.com/konflux-ci/operator-toolkit/controller"
	"github.com/konflux-ci/operator-toolkit/predicates"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/go-logr/logr"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/cache"
	"github.com/konflux-ci/release-service/loader"
	"github.com/konflux-ci/release-service/tekton"
	libhandler "github.com/operator-framework/operator-lib/handler"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Controller reconciles a Release object
type Controller struct {
	client client.Client
	log    logr.Logger
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releases/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=applications/finalizers,verbs=update
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=enterprisecontractpolicies,verbs=get;list;watch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=enterprisecontractpolicies/status,verbs=get
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releaseserviceconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=internalrequests,verbs=create;delete;get;list;watch
//InternalRequests RBAC is required to prevent `forbidden: user system:serviceaccount:release-service:release-service-controller-manager
//is attempting to grant RBAC permissions not currently held`

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.WithValues("Release", req.NamespacedName)

	release := &v1alpha1.Release{}
	err := c.client.Get(ctx, req.NamespacedName, release)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	adapter := newAdapter(ctx, c.client, release, loader.NewLoader(), &logger)

	return controller.ReconcileHandler([]controller.Operation{
		adapter.EnsureFinalizersAreCalled,
		adapter.EnsureConfigIsLoaded, // This operation sets the config in the adapter to be used in other operations.
		adapter.EnsureReleaseIsRunning,
		adapter.EnsureReleaseIsValid,
		adapter.EnsureFinalizerIsAdded,
		adapter.EnsureReleaseExpirationTimeIsAdded,
		adapter.EnsureTenantPipelineIsProcessed,
		adapter.EnsureTenantPipelineProcessingIsTracked,
		adapter.EnsureManagedPipelineIsProcessed,
		adapter.EnsureManagedPipelineProcessingIsTracked,
		adapter.EnsureReleaseProcessingResourcesAreCleanedUp,
		adapter.EnsureReleaseIsCompleted,
	})
}

// Register registers the controller with the passed manager and log. This controller ignores Release status updates and
// also watches for PipelineRuns and SnapshotEnvironmentBindings that are created by the adapter and owned by the
// Releases so the owner gets reconciled on changes.
func (c *Controller) Register(mgr ctrl.Manager, log *logr.Logger, _ cluster.Cluster) error {
	c.client = mgr.GetClient()
	c.log = log.WithName("release")

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Release{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}, predicates.IgnoreBackups{})).
		Watches(&tektonv1.PipelineRun{}, &libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Kind:  "Release",
				Group: "appstudio.redhat.com",
			},
		}, builder.WithPredicates(tekton.ReleasePipelineRunSucceededPredicate())).
		Complete(c)
}

// SetupCache indexes fields for each of the resources used in the release adapter in those cases where filtering by
// field is required.
func (c *Controller) SetupCache(mgr ctrl.Manager) error {
	if err := cache.SetupComponentCache(mgr); err != nil {
		return err
	}
	if err := cache.SetupReleaseCache(mgr); err != nil {
		return err
	}

	// NOTE: Both the release and releaseplan controller need this ReleasePlanAdmission cache. However, it only needs to be added
	// once to the manager, so only one controller should add it. If it is removed here, it should be added to the ReleasePlan controller.
	return cache.SetupReleasePlanAdmissionCache(mgr)
}
