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

package v1alpha1

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/release-service/metadata"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReleasePlanWebhook describes the data structure for the bar webhook
type ReleasePlanWebhook struct{}

// Register registers the webhook with the passed manager and log.
func (w *ReleasePlanWebhook) Register(mgr ctrl.Manager, log *logr.Logger) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ReleasePlan{}).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-releaseplan,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplans,verbs=create,versions=v1alpha1,name=mreleaseplan.kb.io,admissionReviewVersions=v1

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (w *ReleasePlanWebhook) Default(ctx context.Context, obj runtime.Object) error {
	rp := obj.(*ReleasePlan)

	if _, found := rp.GetLabels()[metadata.AutoReleaseLabel]; !found {
		if rp.Labels == nil {
			rp.Labels = map[string]string{
				metadata.AutoReleaseLabel: "true",
			}
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-releaseplan,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplans,verbs=create;update,versions=v1alpha1,name=vreleaseplan.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (w *ReleasePlanWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return w.validateAutoReleaseLabel(obj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (w *ReleasePlanWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return w.validateAutoReleaseLabel(newObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (w *ReleasePlanWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// validateAutoReleaseLabel throws an error if the auto-release label value is set to anything besides true or false.
func (w *ReleasePlanWebhook) validateAutoReleaseLabel(obj runtime.Object) error {
	rp := obj.(*ReleasePlan)

	if value, found := rp.GetLabels()[metadata.AutoReleaseLabel]; found {
		if value != "true" && value != "false" {
			return fmt.Errorf("'%s' label can only be set to true or false", metadata.AutoReleaseLabel)
		}
	}
	return nil
}
