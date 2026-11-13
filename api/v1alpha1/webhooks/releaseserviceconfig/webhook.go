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

package releaseserviceconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/konflux-ci/release-service/api/v1alpha1"
)

// Webhook describes the data structure for the releaseserviceconfig webhook
type Webhook struct {
	client client.Client
	log    logr.Logger
}

// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-releaseserviceconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseserviceconfigs,verbs=create;update,versions=v1alpha1,name=vreleaseserviceconfig.kb.io,admissionReviewVersions=v1

// Register registers the webhook with the passed manager and log.
func (w *Webhook) Register(mgr ctrl.Manager, log *logr.Logger) error {
	w.client = mgr.GetClient()
	w.log = log.WithName("releaseServiceConfig")

	return ctrl.NewWebhookManagedBy(mgr, &v1alpha1.ReleaseServiceConfig{}).
		WithValidator(w).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateCreate(ctx context.Context, rsc *v1alpha1.ReleaseServiceConfig) (warnings admission.Warnings, err error) {
	return w.validateAllowedGitResolvers(rsc)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateUpdate(ctx context.Context, oldRSC, newRSC *v1alpha1.ReleaseServiceConfig) (warnings admission.Warnings, err error) {
	return w.validateAllowedGitResolvers(newRSC)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (w *Webhook) ValidateDelete(ctx context.Context, rsc *v1alpha1.ReleaseServiceConfig) (warnings admission.Warnings, err error) {
	return nil, nil
}

// validateAllowedGitResolvers validates the AllowedGitResolvers field by checking all regex patterns.
func (w *Webhook) validateAllowedGitResolvers(rsc *v1alpha1.ReleaseServiceConfig) (warnings admission.Warnings, err error) {
	validationErrors := rsc.ValidateGitResolverPatterns()
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("invalid regex patterns in AllowedGitResolvers: %s", strings.Join(validationErrors, "; "))
	}
	return nil, nil
}
