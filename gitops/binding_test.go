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
	"github.com/redhat-appstudio/application-service/api/v1alpha1"
	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"reflect"
)

var _ = Describe("Binding", func() {
	components := []v1alpha1.Component{
		{
			Spec: v1alpha1.ComponentSpec{
				Application:   "app",
				ComponentName: "foo",
				Source: v1alpha1.ComponentSource{
					ComponentSourceUnion: v1alpha1.ComponentSourceUnion{
						GitSource: &v1alpha1.GitSource{
							URL: "https://foo",
						},
					},
				},
			},
		},
		{
			Spec: v1alpha1.ComponentSpec{
				Application:   "app",
				ComponentName: "bar",
				Replicas:      2,
				Source: v1alpha1.ComponentSource{
					ComponentSourceUnion: v1alpha1.ComponentSourceUnion{
						GitSource: &v1alpha1.GitSource{
							URL: "https://foo",
						},
					},
				},
			},
		},
	}

	environment := &appstudioshared.Environment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "environment",
			Namespace: "default",
		},
		Spec: appstudioshared.EnvironmentSpec{
			DeploymentStrategy: appstudioshared.DeploymentStrategy_Manual,
			DisplayName:        "production",
			Type:               appstudioshared.EnvironmentType_POC,
		},
	}

	snapshot := &appstudioshared.ApplicationSnapshot{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "snapshot-",
			Namespace:    "default",
		},
		Spec: appstudioshared.ApplicationSnapshotSpec{
			Application: "app",
			Components:  []appstudioshared.ApplicationSnapshotComponent{},
		},
	}

	Context("When calling getComponentBindings with a list of Components", func() {
		bindingComponents := getComponentBindings(components)

		It("can create and return a new BindingComponent slice", func() {
			Expect(reflect.TypeOf(bindingComponents)).To(Equal(reflect.TypeOf([]appstudioshared.BindingComponent{})))
			Expect(len(bindingComponents)).To(Equal(len(components)))
		})

		It("respect the number of replicas defined in the components", func() {
			for i, component := range components {
				Expect(bindingComponents[i].Configuration.Replicas).To(
					Equal(int(math.Max(1, float64(component.Spec.Replicas)))),
				)
			}
		})
	})

	Context("When calling NewSnapshotEnvironmentBinding", func() {
		It("can create and return a new SnapshotEnvironmentBinding", func() {
			binding := NewSnapshotEnvironmentBinding(components, snapshot, environment)
			Expect(reflect.TypeOf(binding)).To(Equal(reflect.TypeOf(&appstudioshared.ApplicationSnapshotEnvironmentBinding{})))
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
