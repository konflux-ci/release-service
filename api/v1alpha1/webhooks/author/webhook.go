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

package author

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlWebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/redhat-appstudio/release-service/metadata"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Webhook describes the data structure for the author webhook
type Webhook struct {
	client client.Client
	log    logr.Logger
}

// Handle creates an admission response for Release and ReleasePlan requests.
func (w *Webhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Kind.Kind {
	case "Release":
		return w.handleRelease(req)
	case "ReleasePlan":
		return w.handleReleasePlan(req)
	default:
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("webhook tried to handle an unsupported resource: %s", req.Kind.Kind))
	}
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-author,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releases;releaseplans,verbs=create;update,versions=v1alpha1,name=mauthor.kb.io,admissionReviewVersions=v1

// Register registers the webhook with the passed manager and log.
func (w *Webhook) Register(mgr ctrl.Manager, log *logr.Logger) error {
	w.client = mgr.GetClient()
	w.log = log.WithName("author")

	mgr.GetWebhookServer().Register("/mutate-appstudio-redhat-com-v1alpha1-author", &ctrlWebhook.Admission{Handler: w})

	return nil
}

// handleRelease takes an incoming admission request and returns an admission response. Create requests
// add an author label with the current user. Update requests are rejected if the author label is being
// modified. All other requests are accepted without action.
func (w *Webhook) handleRelease(req admission.Request) admission.Response {
	release := &v1alpha1.Release{}
	err := json.Unmarshal(req.Object.Raw, release)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrap(err, "error decoding object"))
	}

	switch req.AdmissionRequest.Operation {
	case admissionv1.Create:
		if release.GetLabels()[metadata.AutomatedLabel] != "true" {
			w.setAuthorLabel(req.UserInfo.Username, release)
		}

		return w.patchResponse(req.Object.Raw, release)
	case admissionv1.Update:
		oldRelease := &v1alpha1.Release{}
		err := json.Unmarshal(req.OldObject.Raw, oldRelease)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, errors.Wrap(err, "error decoding object"))
		}

		if release.GetLabels()[metadata.AuthorLabel] != oldRelease.GetLabels()[metadata.AuthorLabel] {
			return admission.Errored(http.StatusBadRequest, errors.New("release author label cannnot be updated"))
		}
	}
	return admission.Allowed("Success")
}

// handleReleasePlan takes an incoming admission request and returns an admission response. If the
// attribution label is set to true, the current user is set as the author. If the attribution label
// is false, the author label is removed. The only exception is if the attribution label remains true
// during an update and the author value is not modified, the previous author label remains.
func (w *Webhook) handleReleasePlan(req admission.Request) admission.Response {
	releasePlan := &v1alpha1.ReleasePlan{}
	err := json.Unmarshal(req.Object.Raw, releasePlan)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrap(err, "error decoding object"))
	}
	// Author label should not exist in any case if attribution is not true
	if releasePlan.GetLabels()[metadata.AttributionLabel] != "true" {
		delete(releasePlan.GetLabels(), metadata.AuthorLabel)
	}

	switch req.AdmissionRequest.Operation {
	case admissionv1.Create:
		if releasePlan.GetLabels()[metadata.AttributionLabel] == "true" {
			w.setAuthorLabel(req.UserInfo.Username, releasePlan)
		}
	case admissionv1.Update:
		oldReleasePlan := &v1alpha1.ReleasePlan{}
		err := json.Unmarshal(req.OldObject.Raw, oldReleasePlan)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, errors.Wrap(err, "error decoding object"))
		}

		if releasePlan.GetLabels()[metadata.AttributionLabel] == "true" {
			author := releasePlan.GetLabels()[metadata.AuthorLabel]

			if oldReleasePlan.GetLabels()[metadata.AttributionLabel] != "true" || author == w.sanitizeLabelValue(req.UserInfo.Username) {
				w.setAuthorLabel(req.UserInfo.Username, releasePlan)
			} else {
				// Preserve previous author if the new author does not match the user making the change
				w.setAuthorLabel(oldReleasePlan.GetLabels()[metadata.AuthorLabel], releasePlan)
			}
		}
	}

	return w.patchResponse(req.Object.Raw, releasePlan)
}

// patchResponse returns an admission response that patches the passed raw object to be the passed object.
func (w *Webhook) patchResponse(raw []byte, object client.Object) admission.Response {
	marshalledObject, err := json.Marshal(object)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, errors.Wrap(err, "error encoding object"))
	}

	return admission.PatchResponseFromRaw(raw, marshalledObject)
}

// setAuthorLabel returns the passed object with the author label added.
func (w *Webhook) setAuthorLabel(username string, obj client.Object) {
	labels := make(map[string]string)
	if obj.GetLabels() != nil {
		labels = obj.GetLabels()
	}

	labels[metadata.AuthorLabel] = w.sanitizeLabelValue(username)
	obj.SetLabels(labels)
}

// sanitizeLabelValue takes a username and returns it in a form appropriate to use as a label value.
func (w *Webhook) sanitizeLabelValue(username string) string {
	author := strings.Replace(username, ":", "_", -1) // Colons disallowed in labels

	if len(author) > metadata.MaxLabelLength {
		author = string(author)[0:metadata.MaxLabelLength]
	}

	return author
}
