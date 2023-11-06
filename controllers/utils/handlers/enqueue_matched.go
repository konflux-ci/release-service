/*
Copyright 2023.

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

package handlers

import (
	"context"
	"strings"

	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crtHandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EnqueueRequestForMatchedResource enqueues Request containing the Name and Namespace of the resource(s) specified in the
// Status of the ReleasePlans and ReleasePlanAdmissions that are the source of the Event. The source of the event
// triggers reconciliation of the parent resource.
type EnqueueRequestForMatchedResource struct{}

var _ crtHandler.EventHandler = &EnqueueRequestForMatchedResource{}

// Create implements EventHandler.
func (e *EnqueueRequestForMatchedResource) Create(_ context.Context, _ event.CreateEvent, _ workqueue.RateLimitingInterface) {
	// A freshly created resource won't have any resources in its status
}

// Update implements EventHandler.
func (e *EnqueueRequestForMatchedResource) Update(_ context.Context, updateEvent event.UpdateEvent, rateLimitingInterface workqueue.RateLimitingInterface) {
	if releasePlan, ok := updateEvent.ObjectOld.(*v1alpha1.ReleasePlan); ok {
		enqueueRequest(releasePlan.Status.ReleasePlanAdmission.Name, rateLimitingInterface)
	} else if releasePlanAdmission, ok := updateEvent.ObjectOld.(*v1alpha1.ReleasePlanAdmission); ok {
		for _, releasePlan := range releasePlanAdmission.Status.ReleasePlans {
			enqueueRequest(releasePlan.Name, rateLimitingInterface)
		}
	}

	if releasePlan, ok := updateEvent.ObjectNew.(*v1alpha1.ReleasePlan); ok {
		enqueueRequest(releasePlan.Status.ReleasePlanAdmission.Name, rateLimitingInterface)
	} else if releasePlanAdmission, ok := updateEvent.ObjectNew.(*v1alpha1.ReleasePlanAdmission); ok {
		for _, releasePlan := range releasePlanAdmission.Status.ReleasePlans {
			enqueueRequest(releasePlan.Name, rateLimitingInterface)
		}
	}
}

// Delete implements EventHandler.
func (e *EnqueueRequestForMatchedResource) Delete(_ context.Context, deleteEvent event.DeleteEvent, rateLimitingInterface workqueue.RateLimitingInterface) {
	if releasePlan, ok := deleteEvent.Object.(*v1alpha1.ReleasePlan); ok {
		enqueueRequest(releasePlan.Status.ReleasePlanAdmission.Name, rateLimitingInterface)
	} else if releasePlanAdmission, ok := deleteEvent.Object.(*v1alpha1.ReleasePlanAdmission); ok {
		for _, releasePlan := range releasePlanAdmission.Status.ReleasePlans {
			enqueueRequest(releasePlan.Name, rateLimitingInterface)
		}
	}
}

// Generic implements EventHandler.
func (e *EnqueueRequestForMatchedResource) Generic(_ context.Context, genericEvent event.GenericEvent, rateLimitingInterface workqueue.RateLimitingInterface) {
	if releasePlan, ok := genericEvent.Object.(*v1alpha1.ReleasePlan); ok {
		enqueueRequest(releasePlan.Status.ReleasePlanAdmission.Name, rateLimitingInterface)
	} else if releasePlanAdmission, ok := genericEvent.Object.(*v1alpha1.ReleasePlanAdmission); ok {
		for _, releasePlan := range releasePlanAdmission.Status.ReleasePlans {
			enqueueRequest(releasePlan.Name, rateLimitingInterface)
		}
	}
}

// enqueueRequest parses the provided string to extract the namespace and name into a
// types.NamespacedName and adds a request to the RateLimitingInterface with it.
func enqueueRequest(namespacedNameString string, rateLimitingInterface workqueue.RateLimitingInterface) {
	values := strings.SplitN(namespacedNameString, "/", 2)

	if len(values) < 2 {
		return
	}

	namespacedName := types.NamespacedName{Namespace: values[0], Name: values[1]}
	rateLimitingInterface.Add(reconcile.Request{NamespacedName: namespacedName})
}
