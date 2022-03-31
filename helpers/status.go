package helpers

import (
	"context"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HasStatusChanged returns a boolean indicating whether the status of a given object has changed or not.
func HasStatusChanged(objectOld, objectNew client.Object) bool {
	return objectOld.GetGeneration() == objectNew.GetGeneration()
}

// UpdateStatus updates the status of a given object and returns the result.
func UpdateStatus(client client.Client, ctx context.Context, object client.Object) (ctrl.Result, error) {
	return ctrl.Result{}, client.Status().Update(ctx, object)
}
