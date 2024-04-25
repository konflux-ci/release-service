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

package releaseplan

import (
	"context"

	"github.com/konflux-ci/operator-toolkit/controller"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/go-logr/logr"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/controllers/utils/handlers"
	"github.com/konflux-ci/release-service/controllers/utils/predicates"
	"github.com/konflux-ci/release-service/loader"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Controller reconciles a ReleasePlan object
type Controller struct {
	client client.Client
	log    logr.Logger
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releaseplanadmissions,verbs=get;list;watch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releaseplans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releaseplans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releaseplans/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.WithValues("ReleasePlan", req.NamespacedName)

	releasePlan := &v1alpha1.ReleasePlan{}
	err := c.client.Get(ctx, req.NamespacedName, releasePlan)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	adapter := newAdapter(ctx, c.client, releasePlan, loader.NewLoader(), &logger)

	return controller.ReconcileHandler([]controller.Operation{
		adapter.EnsureMatchingInformationIsSet,
		adapter.EnsureOwnerReferenceIsSet,
	})
}

// Register registers the controller with the passed manager and log.
func (c *Controller) Register(mgr ctrl.Manager, log *logr.Logger, _ cluster.Cluster) error {
	c.client = mgr.GetClient()

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ReleasePlan{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}, predicates.MatchPredicate())).
		Watches(&v1alpha1.ReleasePlanAdmission{}, &handlers.EnqueueRequestForMatchedResource{},
			builder.WithPredicates(predicates.MatchPredicate())).
		Complete(c)
}

// SetupCache indexes fields for each of the resources used in the releaseplan adapter in those cases where filtering by
// field is required.
// NOTE: Both the release and releaseplan controller need this ReleasePlanAdmission cache. However, it only needs to be added
// once to the manager, so only one controller should add it. If it is removed from the Release controller, it should be added here.
func (c *Controller) SetupCache(mgr ctrl.Manager) error {
	return nil
}
