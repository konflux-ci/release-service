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
	"net/http"
	"sort"
	"strings"

	"github.com/konflux-ci/release-service/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlWebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/go-logr/logr"
	"github.com/konflux-ci/release-service/metadata"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
// attribution label is set to true, the author label is set to a string containing all users in the
// namespace who can create Release CRs. If the attribution label is false, the author label is removed.
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
			// Get all users who can create Release CRs in this namespace
			users, err := w.getUsersWithReleaseCreatePermission(context.Background(), releasePlan.Namespace)
			if err != nil {
				w.log.Error(err, "failed to get users with Release create permission", "namespace", releasePlan.Namespace)
				// Fall back to current user on error
				w.setAuthorLabel(req.UserInfo.Username, releasePlan)
			} else {
				// Set author label to all users who can create releases
				authorString := w.formatUsersAsAuthor(users)
				w.setAuthorLabel(authorString, releasePlan)
			}
		}
	case admissionv1.Update:
		oldReleasePlan := &v1alpha1.ReleasePlan{}
		err := json.Unmarshal(req.OldObject.Raw, oldReleasePlan)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, errors.Wrap(err, "error decoding object"))
		}

		if releasePlan.GetLabels()[metadata.AttributionLabel] == "true" {
			// For updates, always refresh the author list to ensure it's current
			users, err := w.getUsersWithReleaseCreatePermission(context.Background(), releasePlan.Namespace)
			if err != nil {
				w.log.Error(err, "failed to get users with Release create permission", "namespace", releasePlan.Namespace)
				// Fall back to current user on error to ensure author is always refreshed
				w.setAuthorLabel(req.UserInfo.Username, releasePlan)
			} else {
				// Set author label to all users who can create releases
				authorString := w.formatUsersAsAuthor(users)
				w.setAuthorLabel(authorString, releasePlan)
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
	author = strings.Replace(author, "@", ".", 1)     // At sign is disallowed. Support usernames that uses email address.

	if len(author) > metadata.MaxLabelLength {
		author = string(author)[0:metadata.MaxLabelLength]
	}

	return author
}

// getUsersWithReleaseCreatePermission retrieves all users in the specified namespace
// who have permission to create Release CRs by examining RoleBindings in that namespace.
func (w *Webhook) getUsersWithReleaseCreatePermission(ctx context.Context, namespace string) ([]string, error) {
	var users []string
	userSet := make(map[string]bool) // Use a set to avoid duplicates

	// Function to check if a role has create permission on releases
	hasCreateReleasePermission := func(rules []rbacv1.PolicyRule) bool {
		for _, rule := range rules {
			// Check if the rule applies to the release resource
			for _, apiGroupStr := range rule.APIGroups {
				if apiGroupStr == "appstudio.redhat.com" || apiGroupStr == "*" {
					for _, resource := range rule.Resources {
						if resource == "releases" || resource == "*" {
							// Check if create verb is allowed
							for _, verb := range rule.Verbs {
								if verb == "create" || verb == "*" {
									return true
								}
							}
						}
					}
				}
			}
		}
		return false
	}

	// Get RoleBindings in the namespace
	roleBindings := &rbacv1.RoleBindingList{}
	if err := w.client.List(ctx, roleBindings, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list RoleBindings in namespace %s: %v", namespace, err)
	}

	// Process RoleBindings
	for _, rb := range roleBindings.Items {
		if rb.RoleRef.Kind == "Role" {
			// Get the Role
			role := &rbacv1.Role{}
			if err := w.client.Get(ctx, client.ObjectKey{Name: rb.RoleRef.Name, Namespace: namespace}, role); err != nil {
				w.log.Error(err, "failed to get Role", "role", rb.RoleRef.Name, "namespace", namespace)
				continue
			}

			if hasCreateReleasePermission(role.Rules) {
				// Add users from this RoleBinding
				for _, subject := range rb.Subjects {
					if subject.Kind == "User" {
						userSet[subject.Name] = true
					}
				}
			}
		} else if rb.RoleRef.Kind == "ClusterRole" {
			// Get the ClusterRole (RoleBinding can reference ClusterRole)
			clusterRole := &rbacv1.ClusterRole{}
			if err := w.client.Get(ctx, client.ObjectKey{Name: rb.RoleRef.Name}, clusterRole); err != nil {
				w.log.Error(err, "failed to get ClusterRole", "clusterRole", rb.RoleRef.Name)
				continue
			}

			if hasCreateReleasePermission(clusterRole.Rules) {
				// Add users from this RoleBinding
				for _, subject := range rb.Subjects {
					if subject.Kind == "User" {
						userSet[subject.Name] = true
					}
				}
			}
		}
	}

	// Convert set to slice
	for user := range userSet {
		users = append(users, user)
	}

	// Sort for consistent output
	sort.Strings(users)

	return users, nil
}

// formatUsersAsAuthor takes a list of users and formats them as a single string
// suitable for use as an author label value, respecting label length limits.
func (w *Webhook) formatUsersAsAuthor(users []string) string {
	if len(users) == 0 {
		return "unknown"
	}

	// Join users with underscores (allowed in label values), but sanitize each username first
	var sanitizedUsers []string
	for _, user := range users {
		sanitizedUsers = append(sanitizedUsers, w.sanitizeLabelValue(user))
	}

	author := strings.Join(sanitizedUsers, "_")

	// Ensure we don't exceed the label length limit
	if len(author) > metadata.MaxLabelLength {
		author = author[:metadata.MaxLabelLength]
		// If we truncated in the middle of a username, find the last underscore and truncate there
		if lastUnderscore := strings.LastIndex(author, "_"); lastUnderscore > 0 {
			author = author[:lastUnderscore]
		}
	}

	return author
}
