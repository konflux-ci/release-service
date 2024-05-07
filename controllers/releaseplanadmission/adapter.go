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
	"reflect"

	"github.com/go-logr/logr"
	"github.com/konflux-ci/operator-toolkit/controller"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/loader"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// adapter holds the objects needed to reconcile a ReleasePlanAdmission.
type adapter struct {
	client               client.Client
	ctx                  context.Context
	loader               loader.ObjectLoader
	logger               *logr.Logger
	releasePlanAdmission *v1alpha1.ReleasePlanAdmission
}

// newAdapter creates and returns an adapter instance.
func newAdapter(ctx context.Context, client client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission, loader loader.ObjectLoader, logger *logr.Logger) *adapter {
	return &adapter{
		client:               client,
		ctx:                  ctx,
		loader:               loader,
		logger:               logger,
		releasePlanAdmission: releasePlanAdmission,
	}
}

// EnsureMatchingInformationIsSet is an operation that will ensure that the ReleasePlanAdmission has updated matching
// information in its status.
func (a *adapter) EnsureMatchingInformationIsSet() (controller.OperationResult, error) {
	releasePlans, err := a.loader.GetMatchingReleasePlans(a.ctx, a.client, a.releasePlanAdmission)
	if err != nil {
		return controller.RequeueWithError(err)
	}

	copiedReleasePlanAdmission := a.releasePlanAdmission.DeepCopy()
	patch := client.MergeFrom(copiedReleasePlanAdmission)

	a.releasePlanAdmission.ClearMatchingInfo()
	for i := range releasePlans.Items {
		a.releasePlanAdmission.MarkMatched(&releasePlans.Items[i])
	}

	// If there is no change in the matched ReleasePlans and the Matched condition is present
	// (in case it is a new ReleasePlanAdmission going from matched to nil -> matched to nil), do not patch
	if reflect.DeepEqual(copiedReleasePlanAdmission.Status.ReleasePlans, a.releasePlanAdmission.Status.ReleasePlans) &&
		meta.FindStatusCondition(copiedReleasePlanAdmission.Status.Conditions,
			v1alpha1.MatchedConditionType.String()) != nil {
		return controller.ContinueProcessing()
	}

	return controller.RequeueOnErrorOrContinue(a.client.Status().Patch(a.ctx, a.releasePlanAdmission, patch))
}
