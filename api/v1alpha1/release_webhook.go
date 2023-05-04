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
	"encoding/json"
	"reflect"
	"strings"

	"github.com/redhat-appstudio/release-service/metadata"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// releaseHandler describes the handler for the Release low level webhook
type releaseHandler struct {
	Client client.Client
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-release,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releases,verbs=create;update,versions=v1alpha1,name=mrelease.kb.io,admissionReviewVersions=v1

// SetupReleaseLowLevelWebhook registers the Release low level webhook with the passed manager.
func SetupReleaseLowlevelWebhook(mgr ctrl.Manager) {
	mgr.GetWebhookServer().Register("/mutate-appstudio-redhat-com-v1alpha1-release", &webhook.Admission{Handler: &releaseHandler{Client: mgr.GetClient()}})
}

// Handle takes an incoming admission request and returns an admission response. Create requests add
// an author label with the current user. Update requests are rejected if the spec or author label
// are being modified. All other requests are accepted without action.
func (r *releaseHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.AdmissionRequest.Operation {
	case admissionv1.Create:
		return patchAuthorResponse(req)
	case admissionv1.Update:
		var releaseOld, releaseNew Release
		err := json.Unmarshal(req.Object.Raw, &releaseNew)
		if err != nil {
			return failedResponse(req.UID, "could not unmarshal the raw incoming Release object")
		}
		err = json.Unmarshal(req.OldObject.Raw, &releaseOld)
		if err != nil {
			return failedResponse(req.UID, "could not unmarshal the raw old Release object")
		}
		if !reflect.DeepEqual(releaseNew.Spec, releaseOld.Spec) {
			return failedResponse(req.UID, "release resources spec cannot be updated")
		}
		if releaseNew.GetLabels()[metadata.AuthorLabel] != releaseOld.GetLabels()[metadata.AuthorLabel] {
			return failedResponse(req.UID, "release author label cannnot be updated")
		}
	}
	return successfulResponse(req.UID)
}

// successfulResponse returns a success admission.Response that allows the requested operation.
func successfulResponse(uid types.UID) admission.Response {
	return admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			UID:     uid,
			Allowed: true,
			Result: &metav1.Status{
				Status: "Success",
			},
		},
	}
}

// failedResponse returns a failed admission.Response that blocks the requested operation.
func failedResponse(uid types.UID, message string) admission.Response {
	return admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			UID:     uid,
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: message,
			},
		},
	}
}

// patchAuthorResponse returns an admission.Response that modifies the CR author label to be
// the user who issued the request.
func patchAuthorResponse(req admission.Request) admission.Response {
	author := strings.Replace(req.UserInfo.Username, ":", "_", -1)     // Colons disallowed in labels
	author = strings.Replace(author, "system_serviceaccount", "sa", 1) // Effort to shorten long usernames
	if len(author) > 63 {
		author = string(author)[0:63] // Max length of label value is 63 chars
	}
	var patch []jsonpatch.JsonPatchOperation
	patch = append(patch, jsonpatch.JsonPatchOperation{
		Operation: "replace",
		Path:      "/metadata/labels",
		Value: map[string]string{
			metadata.AuthorLabel: author,
		},
	})
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return failedResponse(req.UID, "creation of JSON patch adding author label failed")
	}
	return admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: true,
			Result: &metav1.Status{
				Status: "Success",
			},
			Patch: patchBytes,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		},
	}
}
