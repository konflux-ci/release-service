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
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *ReleaseLink) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-releaselink,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaselinks,verbs=create;update,versions=v1alpha1,name=mreleaselink.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ReleaseLink{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *ReleaseLink) Default() {
}

// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-releaselink,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaselinks,verbs=create;update,versions=v1alpha1,name=vreleaselink.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ReleaseLink{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ReleaseLink) ValidateCreate() error {
	return r.isTargetValid()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *ReleaseLink) ValidateUpdate(old runtime.Object) error {
	return r.isTargetValid()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ReleaseLink) ValidateDelete() error {
	return nil
}

// isTargetValid checks that the target namespace defined in the ReleaseLink CR is not the same as the namespace the ReleaseLink CR was created in.
func (r *ReleaseLink) isTargetValid() error {
	if r.Spec.Target == r.Namespace {
		return fmt.Errorf("field spec.target and namespace cannot have the same value")
	}

	return nil
}
