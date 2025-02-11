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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	"github.com/konflux-ci/release-service/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crtHandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ crtHandler.EventHandler = &EnqueueRequestForMatchedResource[client.Object]{}

// EnqueueRequestForMatchedResource enqueues Request containing the Name and Namespace of the resource(s) specified in the
// Status of the ReleasePlans and ReleasePlanAdmissions that are the source of the Event. The source of the event
// triggers reconciliation of the parent resource.
type EnqueueRequestForMatchedResource[object client.Object] struct{}

// Create implements EventHandler.
func (e *EnqueueRequestForMatchedResource[T]) Create(_ context.Context, _ event.TypedCreateEvent[T], _ workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	// A freshly created resource won't have any resources in its status
}

// Update implements EventHandler.
func (e *EnqueueRequestForMatchedResource[T]) Update(_ context.Context, updateEvent event.TypedUpdateEvent[T], rateLimitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	switch object := any(updateEvent.ObjectOld).(type) {
	case *v1alpha1.ReleasePlan:
		enqueueRequest(object.Status.ReleasePlanAdmission.Name, rateLimitingInterface)
	case *v1alpha1.ReleasePlanAdmission:
		for _, releasePlan := range object.Status.ReleasePlans {
			enqueueRequest(releasePlan.Name, rateLimitingInterface)
		}
	}

	switch object := any(updateEvent.ObjectNew).(type) {
	case *v1alpha1.ReleasePlan:
		enqueueRequest(object.Status.ReleasePlanAdmission.Name, rateLimitingInterface)
	case *v1alpha1.ReleasePlanAdmission:
		for _, releasePlan := range object.Status.ReleasePlans {
			enqueueRequest(releasePlan.Name, rateLimitingInterface)
		}
	}
}

// Delete implements EventHandler.
func (e *EnqueueRequestForMatchedResource[T]) Delete(_ context.Context, deleteEvent event.TypedDeleteEvent[T], rateLimitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	switch object := any(deleteEvent.Object).(type) {
	case *v1alpha1.ReleasePlan:
		enqueueRequest(object.Status.ReleasePlanAdmission.Name, rateLimitingInterface)
	case *v1alpha1.ReleasePlanAdmission:
		for _, releasePlan := range object.Status.ReleasePlans {
			enqueueRequest(releasePlan.Name, rateLimitingInterface)
		}
	}
}

// Generic implements EventHandler.
func (e *EnqueueRequestForMatchedResource[T]) Generic(_ context.Context, genericEvent event.TypedGenericEvent[T], rateLimitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	switch object := any(genericEvent.Object).(type) {
	case *v1alpha1.ReleasePlan:
		enqueueRequest(object.Status.ReleasePlanAdmission.Name, rateLimitingInterface)
	case *v1alpha1.ReleasePlanAdmission:
		for _, releasePlan := range object.Status.ReleasePlans {
			enqueueRequest(releasePlan.Name, rateLimitingInterface)
		}
	}
}

// enqueueRequest parses the provided string to extract the namespace and name into a
// types.NamespacedName and adds a request to the RateLimitingInterface with it.
func enqueueRequest(namespacedNameString string, rateLimitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	values := strings.SplitN(namespacedNameString, "/", 2)

	if len(values) < 2 {
		return
	}

	namespacedName := types.NamespacedName{Namespace: values[0], Name: values[1]}
	rateLimitingInterface.Add(reconcile.Request{NamespacedName: namespacedName})
}
