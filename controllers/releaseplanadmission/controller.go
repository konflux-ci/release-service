/*
Copyright 2023.

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

package releaseplanadmission

import (
	"context"

	"github.com/redhat-appstudio/operator-toolkit/controller"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/cache"
	"github.com/redhat-appstudio/release-service/controllers/utils/handlers"
	"github.com/redhat-appstudio/release-service/controllers/utils/predicates"
	"github.com/redhat-appstudio/release-service/loader"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Controller reconciles a ReleasePlanAdmission object
type Controller struct {
	client client.Client
	log    logr.Logger
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releaseplans,verbs=get;list;watch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releaseplansadmissions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releaseplanadmissions/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := c.log.WithValues("ReleasePlanAdmission", req.NamespacedName)

	releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{}
	err := c.client.Get(ctx, req.NamespacedName, releasePlanAdmission)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	adapter := newAdapter(ctx, c.client, releasePlanAdmission, loader.NewLoader(), &logger)

	return controller.ReconcileHandler([]controller.Operation{
		adapter.EnsureMatchingInformationIsSet,
	})
}

// Register registers the controller with the passed manager and log.
func (c *Controller) Register(mgr ctrl.Manager, log *logr.Logger, _ cluster.Cluster) error {
	c.client = mgr.GetClient()

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ReleasePlanAdmission{}, builder.WithPredicates(predicates.MatchPredicate())).
		Watches(&v1alpha1.ReleasePlan{}, &handlers.EnqueueRequestForMatchedResource{},
			builder.WithPredicates(predicates.MatchPredicate())).
		Complete(c)
}

// SetupCache indexes fields for each of the resources used in the releaseplanadmission adapter in those cases where filtering by
// field is required.
func (c *Controller) SetupCache(mgr ctrl.Manager) error {
	return cache.SetupReleasePlanCache(mgr)
}
