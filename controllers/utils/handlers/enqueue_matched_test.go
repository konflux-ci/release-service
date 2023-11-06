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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/client-go/util/workqueue"
)

var _ = Describe("EnqueueRequestForMatchedResource", func() {
	var ctx = context.TODO()

	var rateLimitingInterface workqueue.RateLimitingInterface
	var instance EnqueueRequestForMatchedResource
	var releasePlan *v1alpha1.ReleasePlan
	var releasePlanAdmission *v1alpha1.ReleasePlanAdmission

	BeforeEach(func() {
		rateLimitingInterface = &controllertest.Queue{Interface: workqueue.New()}
		releasePlan = &v1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "rp",
			},
			Spec: v1alpha1.ReleasePlanSpec{
				Application: "app",
				Target:      "default",
			},
			Status: v1alpha1.ReleasePlanStatus{
				ReleasePlanAdmission: v1alpha1.MatchedReleasePlanAdmission{
					Name: "default/rpa",
				},
			},
		}
		releasePlanAdmission = &v1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "rpa",
			},
			Spec: v1alpha1.ReleasePlanAdmissionSpec{
				Applications: []string{"app"},
				Origin:       "default",
			},
			Status: v1alpha1.ReleasePlanAdmissionStatus{
				ReleasePlans: []v1alpha1.MatchedReleasePlan{
					{Name: "default/rp"},
				},
			},
		}
		instance = EnqueueRequestForMatchedResource{}
	})

	Describe("Create", func() {
		It("should not enqueue a request for a ReleasePlan", func() {
			createEvent := event.CreateEvent{
				Object: releasePlan,
			}

			instance.Create(ctx, createEvent, rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(0))
		})

		It("should not enqueue a request for a ReleasePlanAdmission", func() {
			createEvent := event.CreateEvent{
				Object: releasePlanAdmission,
			}

			instance.Create(ctx, createEvent, rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(0))
		})
	})

	Describe("Update", func() {
		It("should enqueue a request for both the objectOld and objectNew with ReleasePlans", func() {
			newReleasePlan := releasePlan.DeepCopy()
			newReleasePlan.Status.ReleasePlanAdmission.Name = "default/new-rpa"

			updateEvent := event.UpdateEvent{
				ObjectOld: releasePlan,
				ObjectNew: newReleasePlan,
			}

			instance.Update(ctx, updateEvent, rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(2))
		})

		It("should enqueue a request for both the objectOld and objectNew with ReleasePlanAdmissions", func() {
			newReleasePlanAdmission := releasePlanAdmission.DeepCopy()
			newReleasePlanAdmission.Status.ReleasePlans = []v1alpha1.MatchedReleasePlan{
				{Name: "default/new-rp"},
			}

			updateEvent := event.UpdateEvent{
				ObjectOld: releasePlanAdmission,
				ObjectNew: newReleasePlanAdmission,
			}

			instance.Update(ctx, updateEvent, rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(2))
		})
	})

	Describe("Delete", func() {
		It("should enqueue a request for a ReleasePlan", func() {
			deleteEvent := event.DeleteEvent{
				Object: releasePlan,
			}

			instance.Delete(ctx, deleteEvent, rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(1))

			i, _ := rateLimitingInterface.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      "rpa",
				},
			}))
		})

		It("should enqueue a request for a ReleasePlanAdmission", func() {
			deleteEvent := event.DeleteEvent{
				Object: releasePlanAdmission,
			}

			instance.Delete(ctx, deleteEvent, rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(1))

			i, _ := rateLimitingInterface.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      "rp",
				},
			}))
		})
	})

	Describe("Generic", func() {
		It("should enqueue a request for a ReleasePlan", func() {
			genericEvent := event.GenericEvent{
				Object: releasePlan,
			}

			instance.Generic(ctx, genericEvent, rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(1))

			i, _ := rateLimitingInterface.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      "rpa",
				},
			}))
		})

		It("should enqueue a request for a ReleasePlanAdmission", func() {
			genericEvent := event.GenericEvent{
				Object: releasePlanAdmission,
			}

			instance.Generic(ctx, genericEvent, rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(1))

			i, _ := rateLimitingInterface.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      "rp",
				},
			}))
		})
	})

	Describe("enqueueRequest", func() {
		It("should enqueue a request for a proper namespaced name", func() {
			enqueueRequest("foo/bar", rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(1))

			i, _ := rateLimitingInterface.Get()
			Expect(i).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "foo",
					Name:      "bar",
				},
			}))
		})

		It("should not enqueue a request for an invalid namespaced name", func() {
			enqueueRequest("bar", rateLimitingInterface)
			Expect(rateLimitingInterface.Len()).To(Equal(0))
		})
	})
})
