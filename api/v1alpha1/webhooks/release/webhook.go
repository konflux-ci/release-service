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
	"fmt"
	"reflect"

	"github.com/konflux-ci/release-service/loader"
	"github.com/konflux-ci/release-service/metadata"

	"github.com/go-logr/logr"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Webhook describes the data structure for the release webhook
type Webhook struct {
	client client.Client
	loader loader.ObjectLoader
	log    logr.Logger
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (w *Webhook) Default(ctx context.Context, obj runtime.Object) error {
	release := obj.(*v1alpha1.Release)

	// Initialize labels map if nil
	if release.Labels == nil {
		release.Labels = make(map[string]string)
	}

	// Set snapshot and releasePlan labels from spec fields
	release.Labels[metadata.SnapshotLabel] = release.Spec.Snapshot
	release.Labels[metadata.ReleasePlanLabel] = release.Spec.ReleasePlan

	if release.Spec.GracePeriodDays != 0 {
		return nil
	}

	releasePlan, err := w.loader.GetReleasePlan(ctx, w.client, release)
	if err != nil {
		if errors.IsNotFound(err) {
			w.log.Info("releasePlan not found. Not setting ReleaseGracePeriodDays")
			return nil
		} else {
			return err
		}
	}

	release.Spec.GracePeriodDays = releasePlan.Spec.ReleaseGracePeriodDays

	return nil
}

//+kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-release,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releases,verbs=create,versions=v1alpha1,name=mrelease.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-release,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releases,verbs=create;update,versions=v1alpha1,name=vrelease.kb.io,admissionReviewVersions=v1

func (w *Webhook) Register(mgr ctrl.Manager, log *logr.Logger) error {
	w.client = mgr.GetClient()
	w.loader = loader.NewLoader()
	w.log = log.WithName("release")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.Release{}).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *Webhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	release := obj.(*v1alpha1.Release)

	// Validate that resource names used as labels don't exceed Kubernetes label value limit (63 characters)
	if len(release.Name) > 63 {
		return nil, fmt.Errorf("release name must be no more than 63 characters, got %d characters", len(release.Name))
	}
	if len(release.Spec.Snapshot) > 63 {
		return nil, fmt.Errorf("snapshot name must be no more than 63 characters, got %d characters", len(release.Spec.Snapshot))
	}
	if len(release.Spec.ReleasePlan) > 63 {
		return nil, fmt.Errorf("releasePlan name must be no more than 63 characters, got %d characters", len(release.Spec.ReleasePlan))
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *Webhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	oldRelease := oldObj.(*v1alpha1.Release)
	newRelease := newObj.(*v1alpha1.Release)

	if !reflect.DeepEqual(newRelease.Spec, oldRelease.Spec) {
		return nil, fmt.Errorf("release resources spec cannot be updated")
	}

	// Validate snapshot label immutability
	if oldRelease.Labels[metadata.SnapshotLabel] != newRelease.Labels[metadata.SnapshotLabel] {
		return nil, fmt.Errorf("release snapshot label cannot be updated")
	}

	// Validate releasePlan label immutability
	if oldRelease.Labels[metadata.ReleasePlanLabel] != newRelease.Labels[metadata.ReleasePlanLabel] {
		return nil, fmt.Errorf("release releasePlan label cannot be updated")
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *Webhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
