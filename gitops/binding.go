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

package gitops

import (
	hasv1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
)

// NewSnapshotEnvironmentBinding creates a new SnapshotEnvironmentBinding.
func NewSnapshotEnvironmentBinding(components []hasv1alpha1.Component, snapshot *appstudioshared.ApplicationSnapshot, environment *appstudioshared.Environment) *appstudioshared.ApplicationSnapshotEnvironmentBinding {
	return &appstudioshared.ApplicationSnapshotEnvironmentBinding{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: environment.Name + "-",
			Namespace:    environment.Namespace,
		},
		Spec: appstudioshared.ApplicationSnapshotEnvironmentBindingSpec{
			Application: snapshot.Spec.Application,
			Environment: environment.Name,
			Snapshot:    snapshot.Name,
			Components:  getComponentBindings(components),
		},
	}
}

// getComponentBindings returns a list of BindingComponents created by using the information of the given Components.
func getComponentBindings(components []hasv1alpha1.Component) []appstudioshared.BindingComponent {
	var bindingComponents []appstudioshared.BindingComponent

	for _, component := range components {
		bindingComponents = append(bindingComponents, appstudioshared.BindingComponent{
			Name: component.Name,
			Configuration: appstudioshared.BindingComponentConfiguration{
				Replicas: int(math.Max(1, float64(component.Spec.Replicas))),
			},
		})
	}

	return bindingComponents
}
