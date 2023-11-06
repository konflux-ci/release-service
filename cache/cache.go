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

package cache

import (
	"context"

	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupComponentCache adds a new index field to be able to search Components by application.
func SetupComponentCache(mgr ctrl.Manager) error {
	componentIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*applicationapiv1alpha1.Component).Spec.Application}
	}

	return mgr.GetCache().IndexField(context.Background(), &applicationapiv1alpha1.Component{},
		"spec.application", componentIndexFunc)
}

// SetupReleasePlanCache adds a new index field to be able to search ReleasePlans by target.
func SetupReleasePlanCache(mgr ctrl.Manager) error {
	releasePlanIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*v1alpha1.ReleasePlan).Spec.Target}
	}

	return mgr.GetCache().IndexField(context.Background(), &v1alpha1.ReleasePlan{},
		"spec.target", releasePlanIndexFunc)
}

// SetupReleasePlanAdmissionCache adds a new index field to be able to search ReleasePlanAdmissions by origin.
func SetupReleasePlanAdmissionCache(mgr ctrl.Manager) error {
	releasePlanAdmissionIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*v1alpha1.ReleasePlanAdmission).Spec.Origin}
	}

	return mgr.GetCache().IndexField(context.Background(), &v1alpha1.ReleasePlanAdmission{},
		"spec.origin", releasePlanAdmissionIndexFunc)
}

// SetupSnapshotEnvironmentBindingCache adds a new index field to be able to search SnapshotEnvironmentBindings by environment.
func SetupSnapshotEnvironmentBindingCache(mgr ctrl.Manager) error {
	snapshotEnvironmentBindingIndexFunc := func(obj client.Object) []string {
		return []string{obj.(*applicationapiv1alpha1.SnapshotEnvironmentBinding).Spec.Environment}
	}

	return mgr.GetCache().IndexField(context.Background(), &applicationapiv1alpha1.SnapshotEnvironmentBinding{},
		"spec.environment", snapshotEnvironmentBindingIndexFunc)
}
