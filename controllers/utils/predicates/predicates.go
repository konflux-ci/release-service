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

package predicates

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/metadata"
)

// MatchPredicate returns a predicate which returns true when a ReleasePlan or ReleasePlanAdmission
// is created, deleted, or when the auto-release label, target, application, or the matched
// resource of one changes.
func MatchPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return haveApplicationsChanged(e.ObjectOld, e.ObjectNew) ||
				hasBehaviorLabelChanged(e.ObjectOld, e.ObjectNew) ||
				hasMatchConditionChanged(e.ObjectOld, e.ObjectNew) ||
				hasSourceChanged(e.ObjectOld, e.ObjectNew)
		},
	}
}

// RetryInfoPredicate returns a predicate which returns true when pipeline or data
// specifications change (needed for retry info updates).
func RetryInfoPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return hasPipelineChanged(e.ObjectOld, e.ObjectNew) ||
				hasDataChanged(e.ObjectOld, e.ObjectNew)
		},
	}
}

// hasConditionChanged returns true if one, but not both, of the conditions
// are nil or if both are not nil and have different lastTransitionTimes.
func hasConditionChanged(conditionOld, conditionNew *metav1.Condition) bool {
	if conditionOld == nil || conditionNew == nil {
		return conditionOld != conditionNew
	}
	// both not nil, check lastTransitionTime for equality
	return !conditionOld.LastTransitionTime.Equal(&conditionNew.LastTransitionTime)
}

// hasBehaviorLabelChanged returns true if the auto-release or block-releases label value is
// different between the two objects.
func hasBehaviorLabelChanged(objectOld, objectNew client.Object) bool {
	if objectOld.GetLabels()[metadata.AutoReleaseLabel] != objectNew.GetLabels()[metadata.AutoReleaseLabel] {
		return true
	}
	if objectOld.GetLabels()[metadata.BlockReleasesLabel] != objectNew.GetLabels()[metadata.BlockReleasesLabel] {
		return true
	}
	return false
}

// haveApplicationsChanged returns true if passed objects are of the same kind and the
// Spec.Application/ComponentGroup or Spec.Applications/ComponentGroups values between them is different.
func haveApplicationsChanged(objectOld, objectNew client.Object) bool {
	if releasePlanOld, ok := objectOld.(*v1alpha1.ReleasePlan); ok {
		if releasePlanNew, ok := objectNew.(*v1alpha1.ReleasePlan); ok {
			return releasePlanOld.Spec.Application != releasePlanNew.Spec.Application ||
				releasePlanOld.Spec.ComponentGroup != releasePlanNew.Spec.ComponentGroup
		}
	}

	if releasePlanAdmissionOld, ok := objectOld.(*v1alpha1.ReleasePlanAdmission); ok {
		if releasePlanAdmissionNew, ok := objectNew.(*v1alpha1.ReleasePlanAdmission); ok {
			return !reflect.DeepEqual(releasePlanAdmissionOld.Spec.Applications, releasePlanAdmissionNew.Spec.Applications) ||
				!reflect.DeepEqual(releasePlanAdmissionOld.Spec.ComponentGroups, releasePlanAdmissionNew.Spec.ComponentGroups)
		}
	}

	return false
}

// hasMatchConditionChanged returns true if the lastTransitionTime of the Matched condition
// is different between the two objects or if one (but not both) of the objects is missing
// the Matched condition.
func hasMatchConditionChanged(objectOld, objectNew client.Object) bool {
	if releasePlanOld, ok := objectOld.(*v1alpha1.ReleasePlan); ok {
		if releasePlanNew, ok := objectNew.(*v1alpha1.ReleasePlan); ok {
			oldCondition := meta.FindStatusCondition(releasePlanOld.Status.Conditions,
				v1alpha1.MatchedConditionType.String())
			newCondition := meta.FindStatusCondition(releasePlanNew.Status.Conditions,
				v1alpha1.MatchedConditionType.String())
			return hasConditionChanged(oldCondition, newCondition)
		}
	} else if releasePlanAdmissionOld, ok := objectOld.(*v1alpha1.ReleasePlanAdmission); ok {
		if releasePlanAdmissionNew, ok := objectNew.(*v1alpha1.ReleasePlanAdmission); ok {
			oldCondition := meta.FindStatusCondition(releasePlanAdmissionOld.Status.Conditions,
				v1alpha1.MatchedConditionType.String())
			newCondition := meta.FindStatusCondition(releasePlanAdmissionNew.Status.Conditions,
				v1alpha1.MatchedConditionType.String())
			return hasConditionChanged(oldCondition, newCondition)
		}
	}
	return false
}

// hasSourceChanged returns true if the objects are ReleasePlans and the Spec.Target value is
// different between the two objects or if the objects are ReleasePlanAdmissions and the
// Spec.Origin value is different between the two.
func hasSourceChanged(objectOld, objectNew client.Object) bool {
	if releasePlanOld, ok := objectOld.(*v1alpha1.ReleasePlan); ok {
		if releasePlanNew, ok := objectNew.(*v1alpha1.ReleasePlan); ok {
			return releasePlanOld.Spec.Target != releasePlanNew.Spec.Target
		}
	}

	if releasePlanAdmissionOld, ok := objectOld.(*v1alpha1.ReleasePlanAdmission); ok {
		if releasePlanAdmissionNew, ok := objectNew.(*v1alpha1.ReleasePlanAdmission); ok {
			return releasePlanAdmissionOld.Spec.Origin != releasePlanAdmissionNew.Spec.Origin
		}
	}

	return false
}

// hasPipelineChanged returns true if the objects are ReleasePlanAdmissions or ReleasePlans
// and their pipeline specifications have changed.
func hasPipelineChanged(objectOld, objectNew client.Object) bool {
	if releasePlanAdmissionOld, ok := objectOld.(*v1alpha1.ReleasePlanAdmission); ok {
		if releasePlanAdmissionNew, ok := objectNew.(*v1alpha1.ReleasePlanAdmission); ok {
			return !reflect.DeepEqual(releasePlanAdmissionOld.Spec.Pipeline, releasePlanAdmissionNew.Spec.Pipeline)
		}
	}

	if releasePlanOld, ok := objectOld.(*v1alpha1.ReleasePlan); ok {
		if releasePlanNew, ok := objectNew.(*v1alpha1.ReleasePlan); ok {
			return !reflect.DeepEqual(releasePlanOld.Spec.TenantPipeline, releasePlanNew.Spec.TenantPipeline) ||
				!reflect.DeepEqual(releasePlanOld.Spec.FinalPipeline, releasePlanNew.Spec.FinalPipeline)
		}
	}

	return false
}

// hasDataChanged returns true if the objects are ReleasePlanAdmissions or ReleasePlans
// and their data specifications have changed.
func hasDataChanged(objectOld, objectNew client.Object) bool {
	if releasePlanAdmissionOld, ok := objectOld.(*v1alpha1.ReleasePlanAdmission); ok {
		if releasePlanAdmissionNew, ok := objectNew.(*v1alpha1.ReleasePlanAdmission); ok {
			return !reflect.DeepEqual(releasePlanAdmissionOld.Spec.Data, releasePlanAdmissionNew.Spec.Data)
		}
	}

	if releasePlanOld, ok := objectOld.(*v1alpha1.ReleasePlan); ok {
		if releasePlanNew, ok := objectNew.(*v1alpha1.ReleasePlan); ok {
			return !reflect.DeepEqual(releasePlanOld.Spec.Data, releasePlanNew.Spec.Data)
		}
	}

	return false
}

// ReleaseServiceConfigPredicate returns a predicate which returns true when a ReleaseServiceConfig
// is created, deleted, or when its RetryablePipelines specification changes.
func ReleaseServiceConfigPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			// Reconcile on deletion to update RPA status to "no retry configuration available"
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			rscOld, okOld := e.ObjectOld.(*v1alpha1.ReleaseServiceConfig)
			rscNew, okNew := e.ObjectNew.(*v1alpha1.ReleaseServiceConfig)

			if !okOld || !okNew {
				return false
			}

			return !reflect.DeepEqual(rscOld.Spec.RetryablePipelines, rscNew.Spec.RetryablePipelines)
		},
	}
}
