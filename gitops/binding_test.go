/*
Copyright 2022 Red Hat Inc.

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"reflect"
)

var _ = Describe("Binding", func() {
	var replicas int = 2

	components := []applicationapiv1alpha1.Component{
		{
			Spec: applicationapiv1alpha1.ComponentSpec{
				Application:   "app",
				ComponentName: "foo",
				Source: applicationapiv1alpha1.ComponentSource{
					ComponentSourceUnion: applicationapiv1alpha1.ComponentSourceUnion{
						GitSource: &applicationapiv1alpha1.GitSource{
							URL: "https://foo",
						},
					},
				},
			},
		},
		{
			Spec: applicationapiv1alpha1.ComponentSpec{
				Application:   "app",
				ComponentName: "bar",
				Replicas:      &replicas,
				Source: applicationapiv1alpha1.ComponentSource{
					ComponentSourceUnion: applicationapiv1alpha1.ComponentSourceUnion{
						GitSource: &applicationapiv1alpha1.GitSource{
							URL: "https://foo",
						},
					},
				},
			},
		},
	}

	environment := &applicationapiv1alpha1.Environment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "environment",
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.EnvironmentSpec{
			DeploymentStrategy: applicationapiv1alpha1.DeploymentStrategy_Manual,
			DisplayName:        "production",
			Type:               applicationapiv1alpha1.EnvironmentType_POC,
		},
	}

	snapshot := &applicationapiv1alpha1.Snapshot{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "snapshot-",
			Namespace:    "default",
		},
		Spec: applicationapiv1alpha1.SnapshotSpec{
			Application: "app",
			Components:  []applicationapiv1alpha1.SnapshotComponent{},
		},
	}

	When("calling getComponentBindings with a list of Components", func() {
		bindingComponents := getComponentBindings(components)

		It("can create and return a new BindingComponent slice", func() {
			Expect(reflect.TypeOf(bindingComponents)).To(Equal(reflect.TypeOf([]applicationapiv1alpha1.BindingComponent{})))
			Expect(len(bindingComponents)).To(Equal(len(components)))
		})

		It("respect the number of replicas defined in the components", func() {
			for i, component := range components {
				var replicas int
				if component.Spec.Replicas != nil {
					replicas = int(math.Max(1, float64(*component.Spec.Replicas)))
				} else {
					replicas = 1
				}
				Expect(bindingComponents[i].Configuration.Replicas).To(Equal(&replicas))
			}
		})
	})

	When("calling NewSnapshotEnvironmentBinding", func() {
		It("can create and return a new SnapshotEnvironmentBinding", func() {
			binding := NewSnapshotEnvironmentBinding(components, snapshot, environment)
			Expect(reflect.TypeOf(binding)).To(Equal(reflect.TypeOf(&applicationapiv1alpha1.SnapshotEnvironmentBinding{})))
			Expect(*binding).To(MatchFields(IgnoreExtras, Fields{
				"ObjectMeta": MatchFields(IgnoreExtras, Fields{
					"GenerateName": Equal(environment.Name + "-"),
					"Namespace":    Equal(environment.Namespace),
				}),
				"Spec": MatchFields(IgnoreExtras, Fields{
					"Application": Equal(snapshot.Spec.Application),
					"Environment": Equal(environment.Name),
					"Snapshot":    Equal(snapshot.Name),
					"Components":  HaveLen(len(components)),
				}),
			}))
		})
	})
})
