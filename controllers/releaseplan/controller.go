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
	"github.com/redhat-appstudio/operator-toolkit/controller"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
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

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releasePlans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releasePlans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=releasePlans/finalizers,verbs=update

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

	adapter := NewAdapter(ctx, c.client, releasePlan, loader.NewLoader(), &logger)

	return controller.ReconcileHandler([]controller.Operation{
		adapter.EnsureOwnerReferenceIsSet,
	})
}

// Register registers the controller with the passed manager and log.
func (c *Controller) Register(mgr ctrl.Manager, log *logr.Logger, _ cluster.Cluster) error {
	c.client = mgr.GetClient()
	c.log = log.WithName("releasePlan")

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ReleasePlan{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(c)
}
