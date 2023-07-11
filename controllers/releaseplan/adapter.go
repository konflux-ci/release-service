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
	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/operator-goodies/reconciler"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"github.com/redhat-appstudio/release-service/syncer"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Adapter holds the objects needed to reconcile a ReleasePlan.
type Adapter struct {
	client      client.Client
	ctx         context.Context
	loader      loader.ObjectLoader
	logger      logr.Logger
	releasePlan *v1alpha1.ReleasePlan
	syncer      *syncer.Syncer
}

// NewAdapter creates and returns an Adapter instance.
func NewAdapter(ctx context.Context, client client.Client, releasePlan *v1alpha1.ReleasePlan, loader loader.ObjectLoader, logger logr.Logger) *Adapter {
	return &Adapter{
		client:      client,
		ctx:         ctx,
		loader:      loader,
		logger:      logger,
		releasePlan: releasePlan,
		syncer:      syncer.NewSyncerWithContext(client, logger, ctx),
	}
}

// EnsureOwnerReferenceIsSet is an operation that will ensure that the owner reference is set.
func (a *Adapter) EnsureOwnerReferenceIsSet() (reconciler.OperationResult, error) {
	if len(a.releasePlan.OwnerReferences) > 0 {
		return reconciler.ContinueProcessing()
	}

	application, err := a.loader.GetApplication(a.ctx, a.client, a.releasePlan)
	if err != nil {
		return reconciler.RequeueWithError(err)
	}

	patch := client.MergeFrom(a.releasePlan.DeepCopy())
	err = ctrl.SetControllerReference(application, a.releasePlan, a.client.Scheme())
	if err != nil {
		return reconciler.RequeueWithError(err)
	}

	err = a.client.Patch(a.ctx, a.releasePlan, patch)
	if err != nil && !errors.IsNotFound(err) {
		return reconciler.RequeueWithError(err)
	}

	return reconciler.ContinueProcessing()
}
