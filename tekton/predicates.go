package tekton

import (
	"github.com/redhat-appstudio/release-service/helpers"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// BuildOrReleasePipelineRunPredicate returns a predicate which filters out all objects except
// PipelineRuns from the Build service which have just succeeded or PipelineRuns from the Release
// service which status has changed.
func BuildOrReleasePipelineRunPredicate() predicate.Predicate {
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
			switch {
			case IsBuildPipelineRun(e.ObjectNew) && hasPipelineSucceeded(e.ObjectOld, e.ObjectNew):
				return true
			case IsReleasePipelineRun(e.ObjectNew) && helpers.HasStatusChanged(e.ObjectOld, e.ObjectNew):
				return true
			default:
				return false
			}
		},
	}
}
