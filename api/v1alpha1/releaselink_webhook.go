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

const (
	// autoReleaseLabel is the label name for the auto-release setting. This will probably be moved elsewhere eventually.
	autoReleaseLabel = "release.appstudio.openshift.io/auto-release"
)

func (r *ReleaseLink) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-releaselink,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaselinks,verbs=create,versions=v1alpha1,name=mreleaselink.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ReleaseLink{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *ReleaseLink) Default() {
	if _, found := r.GetLabels()[autoReleaseLabel]; !found {
		if r.Labels == nil {
			r.Labels = map[string]string{
				autoReleaseLabel: "true",
			}
		}
	}
}

// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-releaselink,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaselinks,verbs=create;update,versions=v1alpha1,name=vreleaselink.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ReleaseLink{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *ReleaseLink) ValidateCreate() error {
	if err := r.validateTarget(); err != nil {
		return err
	}
	return r.validateAutoReleaseLabel()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *ReleaseLink) ValidateUpdate(old runtime.Object) error {
	if err := r.validateTarget(); err != nil {
		return err
	}
	return r.validateAutoReleaseLabel()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *ReleaseLink) ValidateDelete() error {
	return nil
}

// validateTarget checks that the target namespace defined in the ReleaseLink CR is not the same as the namespace the ReleaseLink CR was created in.
func (r *ReleaseLink) validateTarget() error {
	if r.Spec.Target.Namespace == r.Namespace {
		return fmt.Errorf("field spec.target.namespace and namespace cannot have the same value")
	}

	return nil
}

// validateAutoReleaseLabel throws an error if the auto-release label value is set to anything besides true and false.
func (r *ReleaseLink) validateAutoReleaseLabel() error {
	if value, found := r.GetLabels()[autoReleaseLabel]; found {
		if value != "true" && value != "false" {
			return fmt.Errorf("%s label can only be set to true or false", autoReleaseLabel)
		}
	}
	return nil
}
