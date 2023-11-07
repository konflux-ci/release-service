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
	"fmt"
	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/metadata"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Webhook describes the data structure for the bar webhook
type Webhook struct {
	client client.Client
	log    logr.Logger
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (w *Webhook) Default(ctx context.Context, obj runtime.Object) error {
	releasePlan := obj.(*v1alpha1.ReleasePlan)

	if _, found := releasePlan.GetLabels()[metadata.AutoReleaseLabel]; !found {
		if releasePlan.Labels == nil {
			releasePlan.Labels = map[string]string{
				metadata.AutoReleaseLabel: "true",
			}
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-releaseplan,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplans,verbs=create,versions=v1alpha1,name=mreleaseplan.kb.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-releaseplan,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplans,verbs=create;update,versions=v1alpha1,name=vreleaseplan.kb.io,admissionReviewVersions=v1

// Register registers the webhook with the passed manager and log.
func (w *Webhook) Register(mgr ctrl.Manager, log *logr.Logger) error {
	w.client = mgr.GetClient()
	w.log = log.WithName("releasePlan")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.ReleasePlan{}).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return w.validateAutoReleaseLabel(obj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	return w.validateAutoReleaseLabel(newObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

// validateAutoReleaseLabel throws an error if the auto-release label value is set to anything besides true or false.
func (w *Webhook) validateAutoReleaseLabel(obj runtime.Object) (warnings admission.Warnings, err error) {
	releasePlan := obj.(*v1alpha1.ReleasePlan)

	if value, found := releasePlan.GetLabels()[metadata.AutoReleaseLabel]; found {
		if value != "true" && value != "false" {
			return nil, fmt.Errorf("'%s' label can only be set to true or false", metadata.AutoReleaseLabel)
		}
	}
	return nil, nil
}
