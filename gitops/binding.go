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
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
)

// NewSnapshotEnvironmentBinding creates a new SnapshotEnvironmentBinding.
func NewSnapshotEnvironmentBinding(components []applicationapiv1alpha1.Component, snapshot *applicationapiv1alpha1.Snapshot, environment *applicationapiv1alpha1.Environment) *applicationapiv1alpha1.SnapshotEnvironmentBinding {
	return &applicationapiv1alpha1.SnapshotEnvironmentBinding{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: environment.Name + "-",
			Namespace:    environment.Namespace,
		},
		Spec: applicationapiv1alpha1.SnapshotEnvironmentBindingSpec{
			Application: snapshot.Spec.Application,
			Environment: environment.Name,
			Snapshot:    snapshot.Name,
			Components:  getComponentBindings(components),
		},
	}
}

// getComponentBindings returns a list of BindingComponents created by using the information of the given Components.
func getComponentBindings(components []applicationapiv1alpha1.Component) []applicationapiv1alpha1.BindingComponent {
	var bindingComponents []applicationapiv1alpha1.BindingComponent

	for _, component := range components {
		var replicas int
		if component.Spec.Replicas != nil {
			replicas = int(math.Max(1, float64(*component.Spec.Replicas)))
		} else {
			replicas = 1
		}
		bindingComponents = append(bindingComponents, applicationapiv1alpha1.BindingComponent{
			Name: component.Name,
			Configuration: applicationapiv1alpha1.BindingComponentConfiguration{
				Replicas: &replicas,
			},
		})
	}

	return bindingComponents
}
