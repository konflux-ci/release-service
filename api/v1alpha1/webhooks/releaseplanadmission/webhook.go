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

package releaseplanadmission

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/metadata"
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
func (w *Webhook) Default(ctx context.Context, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) error {
	if _, found := releasePlanAdmission.GetLabels()[metadata.BlockReleasesLabel]; !found {
		if releasePlanAdmission.Labels == nil {
			releasePlanAdmission.Labels = map[string]string{
				metadata.BlockReleasesLabel: "false",
			}
		} else {
			releasePlanAdmission.Labels[metadata.BlockReleasesLabel] = "false"
		}
	}

	return nil
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-releaseplanadmission,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplanadmissions,verbs=create,versions=v1alpha1,name=mreleaseplanadmission.kb.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-releaseplanadmission,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplanadmissions,verbs=create;update,versions=v1alpha1,name=vreleaseplanadmission.kb.io,admissionReviewVersions=v1

// Register registers the webhook with the passed manager and log.
func (w *Webhook) Register(mgr ctrl.Manager, log *logr.Logger) error {
	w.client = mgr.GetClient()
	w.log = log.WithName("releasePlanAdmission")

	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.ReleasePlanAdmission{}).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateCreate(ctx context.Context, rpa *v1alpha1.ReleasePlanAdmission) (warnings admission.Warnings, err error) {
	if err := w.validateSpec(rpa); err != nil {
		return nil, err
	}
	return w.validateBlockReleasesLabel(rpa)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateUpdate(ctx context.Context, oldRpa, newRpa *v1alpha1.ReleasePlanAdmission) (warnings admission.Warnings, err error) {
	if err := w.validateSpec(newRpa); err != nil {
		return nil, err
	}
	return w.validateBlockReleasesLabel(newRpa)
}

// validateSpec validates the length of application and componentGroup names.
// Mutual exclusivity is handled by CRD CEL rules.
func (w *Webhook) validateSpec(rpa *v1alpha1.ReleasePlanAdmission) error {
	for _, app := range rpa.Spec.Applications {
		if len(app) > 63 {
			return fmt.Errorf("application name '%s' must be no more than 63 characters, got %d characters", app, len(app))
		}
	}

	for _, cg := range rpa.Spec.ComponentGroups {
		if len(cg) > 63 {
			return fmt.Errorf("componentGroup name '%s' must be no more than 63 characters, got %d characters", cg, len(cg))
		}
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateDelete(ctx context.Context, rpa *v1alpha1.ReleasePlanAdmission) (warnings admission.Warnings, err error) {
	return nil, nil
}

// validateBlockReleasesLabel throws an error if the block-releases label value is set to anything besides true or false.
func (w *Webhook) validateBlockReleasesLabel(rpa *v1alpha1.ReleasePlanAdmission) (warnings admission.Warnings, err error) {
	if value, found := rpa.GetLabels()[metadata.BlockReleasesLabel]; found {
		if value != "true" && value != "false" {
			return nil, fmt.Errorf("'%s' label can only be set to true or false", metadata.BlockReleasesLabel)
		}
	}
	return nil, nil
}
