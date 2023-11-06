/*
Copyright 2023 Red Hat Inc.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/metadata"
	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Predicates", Ordered, func() {

	const (
		namespace       = "default"
		namespace2      = "other"
		applicationName = "test-application"
	)

	Context("Working with ReleasePlans and ReleasePlanAdmissions", func() {
		var releasePlan, releasePlanDiffApp, releasePlanDiffLabel,
			releasePlanDiffTarget, releasePlanDiffStatus *v1alpha1.ReleasePlan
		var releasePlanAdmission, releasePlanAdmissionDiffApps, releasePlanAdmissionDiffOrigin,
			releasePlanAdmissionDiffStatus *v1alpha1.ReleasePlanAdmission
		var instance predicate.Predicate

		BeforeAll(func() {
			releasePlan = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplan",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application: applicationName,
					Target:      namespace2,
				},
			}
			releasePlanDiffApp = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplan-app",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application: "diff",
					Target:      namespace2,
				},
			}
			releasePlanDiffLabel = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplan-label",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "false",
					},
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application: applicationName,
					Target:      namespace2,
				},
			}
			releasePlanDiffTarget = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplan-target",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application: applicationName,
					Target:      "diff",
				},
			}
			releasePlanDiffStatus = &v1alpha1.ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplan-status",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
				Spec: v1alpha1.ReleasePlanSpec{
					Application: applicationName,
					Target:      namespace2,
				},
			}
			releasePlanDiffStatus.MarkMatched(&v1alpha1.ReleasePlanAdmission{})

			releasePlanAdmission = &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplanadmission",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Applications: []string{
						applicationName,
					},
					Origin: namespace2,
					Policy: "policy",
					PipelineRef: &tektonutils.PipelineRef{
						Resolver: "bundles",
						Params: []tektonutils.Param{
							{Name: "bundle", Value: "quay.io/some/bundle"},
							{Name: "name", Value: "release-pipeline"},
							{Name: "kind", Value: "pipeline"},
						},
					},
				},
			}
			releasePlanAdmissionDiffApps = &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplanadmission-app",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Applications: []string{
						"diff",
					},
					Origin: namespace2,
					Policy: "policy",
					PipelineRef: &tektonutils.PipelineRef{
						Resolver: "bundles",
						Params: []tektonutils.Param{
							{Name: "bundle", Value: "quay.io/some/bundle"},
							{Name: "name", Value: "release-pipeline"},
							{Name: "kind", Value: "pipeline"},
						},
					},
				},
			}
			releasePlanAdmissionDiffOrigin = &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplanadmission-origin",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Applications: []string{
						applicationName,
					},
					Origin: "diff",
					Policy: "policy",
					PipelineRef: &tektonutils.PipelineRef{
						Resolver: "bundles",
						Params: []tektonutils.Param{
							{Name: "bundle", Value: "quay.io/some/bundle"},
							{Name: "name", Value: "release-pipeline"},
							{Name: "kind", Value: "pipeline"},
						},
					},
				},
			}
			releasePlanAdmissionDiffStatus = &v1alpha1.ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "releaseplanadmission-status",
					Namespace: namespace,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
				Spec: v1alpha1.ReleasePlanAdmissionSpec{
					Applications: []string{
						applicationName,
					},
					Origin: namespace2,
					Policy: "policy",
					PipelineRef: &tektonutils.PipelineRef{
						Resolver: "bundles",
						Params: []tektonutils.Param{
							{Name: "bundle", Value: "quay.io/some/bundle"},
							{Name: "name", Value: "release-pipeline"},
							{Name: "kind", Value: "pipeline"},
						},
					},
				},
			}
			releasePlanAdmissionDiffStatus.MarkMatched(&v1alpha1.ReleasePlan{})

			instance = MatchPredicate()
		})

		When("calling MatchPredicate", func() {
			It("returns true for creating events", func() {
				contextEvent := event.CreateEvent{
					Object: releasePlan,
				}
				Expect(instance.Create(contextEvent)).To(BeTrue())
			})

			It("returns true for deleting events", func() {
				contextEvent := event.DeleteEvent{
					Object: releasePlan,
				}
				Expect(instance.Delete(contextEvent)).To(BeTrue())
			})

			It("should ignore generic events", func() {
				contextEvent := event.GenericEvent{
					Object: releasePlan,
				}
				Expect(instance.Generic(contextEvent)).To(BeFalse())
			})

			It("returns true when the application changes between ReleasePlans", func() {
				contextEvent := event.UpdateEvent{
					ObjectOld: releasePlan,
					ObjectNew: releasePlanDiffApp,
				}
				Expect(instance.Update(contextEvent)).To(BeTrue())
			})

			It("returns true when the applications change between ReleasePlanAdmissions", func() {
				contextEvent := event.UpdateEvent{
					ObjectOld: releasePlanAdmission,
					ObjectNew: releasePlanAdmissionDiffApps,
				}
				Expect(instance.Update(contextEvent)).To(BeTrue())
			})

			It("returns true when the auto-release label changes", func() {
				contextEvent := event.UpdateEvent{
					ObjectOld: releasePlan,
					ObjectNew: releasePlanDiffLabel,
				}
				Expect(instance.Update(contextEvent)).To(BeTrue())
			})

			It("returns true when the target changes between ReleasePlans", func() {
				contextEvent := event.UpdateEvent{
					ObjectOld: releasePlan,
					ObjectNew: releasePlanDiffTarget,
				}
				Expect(instance.Update(contextEvent)).To(BeTrue())
			})

			It("returns true when the origin changes between ReleasePlanAdmissions", func() {
				contextEvent := event.UpdateEvent{
					ObjectOld: releasePlanAdmission,
					ObjectNew: releasePlanAdmissionDiffOrigin,
				}
				Expect(instance.Update(contextEvent)).To(BeTrue())
			})

			It("returns true when the matched condition in the status changes", func() {
				contextEvent := event.UpdateEvent{
					ObjectOld: releasePlan,
					ObjectNew: releasePlanDiffStatus,
				}
				Expect(instance.Update(contextEvent)).To(BeTrue())
			})
		})

		When("calling haveApplicationsChanged", func() {
			It("returns true when the application has changed between ReleasePlans", func() {
				Expect(haveApplicationsChanged(releasePlan, releasePlanDiffApp)).To(BeTrue())
			})

			It("returns true when the applications have changed between ReleasePlanAdmissions", func() {
				Expect(haveApplicationsChanged(releasePlanAdmission, releasePlanAdmissionDiffApps)).To(BeTrue())
			})

			It("returns false when the application has not changed between ReleasePlans", func() {
				Expect(haveApplicationsChanged(releasePlan, releasePlanDiffTarget)).To(BeFalse())
			})

			It("returns false when the applications have not changed between ReleasePlanAdmissions", func() {
				Expect(haveApplicationsChanged(releasePlanAdmission, releasePlanAdmissionDiffOrigin)).To(BeFalse())
			})
		})

		When("calling hasSourceChanged", func() {
			It("returns true when the target has changed between ReleasePlans", func() {
				Expect(hasSourceChanged(releasePlan, releasePlanDiffTarget)).To(BeTrue())
			})

			It("returns true when the origin has changed between ReleasePlanAdmissions", func() {
				Expect(hasSourceChanged(releasePlanAdmission, releasePlanAdmissionDiffOrigin)).To(BeTrue())
			})

			It("returns false when the target has not changed between ReleasePlans", func() {
				Expect(hasSourceChanged(releasePlan, releasePlanDiffApp)).To(BeFalse())
			})

			It("returns false when the target has not changed between ReleasePlanAdmissions", func() {
				Expect(hasSourceChanged(releasePlanAdmission, releasePlanAdmissionDiffApps)).To(BeFalse())
			})
		})

		When("calling hasMatchConditionChanged", func() {
			It("returns true when the ReleasePlans with differing lastTransitionTimes are passed", func() {
				Expect(hasMatchConditionChanged(releasePlan, releasePlanDiffStatus)).To(BeTrue())
			})

			It("returns true when the ReleasePlanAdmissions with differing lastTransitionTimes are passed", func() {
				Expect(hasMatchConditionChanged(releasePlanAdmission, releasePlanAdmissionDiffStatus)).To(BeTrue())
			})

			It("returns false when the ReleasePlans with the same lastTransitionTimes are passed", func() {
				Expect(hasMatchConditionChanged(releasePlanDiffStatus, releasePlanDiffStatus)).To(BeFalse())
			})

			It("returns false when the ReleasePlanAdmissions with the same lastTransitionTimes are passed", func() {
				Expect(hasMatchConditionChanged(releasePlanAdmissionDiffStatus, releasePlanAdmissionDiffStatus)).To(BeFalse())
			})

			It("returns false when objects of different types are passed", func() {
				Expect(hasMatchConditionChanged(releasePlanDiffStatus, releasePlanAdmissionDiffStatus)).To(BeFalse())
			})
		})
	})

	When("calling hasConditionChanged", func() {
		It("returns false when both conditions are nil", func() {
			Expect(hasConditionChanged(nil, nil)).To(BeFalse())
		})

		It("returns true when just the first condition is nil", func() {
			condition := &metav1.Condition{}
			Expect(hasConditionChanged(condition, nil)).To(BeTrue())
		})

		It("returns true when just the second condition is nil", func() {
			condition := &metav1.Condition{}
			Expect(hasConditionChanged(nil, condition)).To(BeTrue())
		})

		It("returns false when the conditions have the same lastTransitionTime", func() {
			transitionTime := metav1.Time{Time: time.Now()}
			condition1 := &metav1.Condition{LastTransitionTime: transitionTime}
			condition2 := &metav1.Condition{LastTransitionTime: transitionTime}
			Expect(hasConditionChanged(condition1, condition2)).To(BeFalse())
		})

		It("returns true when the conditions have different lastTransitionTimes", func() {
			transitionTime := metav1.Time{Time: time.Now()}
			condition1 := &metav1.Condition{LastTransitionTime: transitionTime}
			condition2 := &metav1.Condition{LastTransitionTime: metav1.Time{Time: transitionTime.Add(time.Minute)}}
			Expect(hasConditionChanged(condition1, condition2)).To(BeTrue())
		})
	})

	When("calling hasAutoReleaseLabelChanged", func() {
		var podTrue, podFalse, podMissing *corev1.Pod

		BeforeAll(func() {
			podTrue = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
			}
			podFalse = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "false",
					},
				},
			}
			podMissing = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
					Labels:      nil,
				},
			}
		})

		It("returns true when the first object has no labels", func() {
			Expect(hasAutoReleaseLabelChanged(podMissing, podTrue)).To(BeTrue())
		})

		It("returns true when the second object has no labels", func() {
			Expect(hasAutoReleaseLabelChanged(podTrue, podFalse)).To(BeTrue())
		})

		It("returns true when the first object has a true label and second has a false label", func() {
			Expect(hasAutoReleaseLabelChanged(podTrue, podFalse)).To(BeTrue())
		})

		It("returns true when the first object has a false label and second has a true label", func() {
			Expect(hasAutoReleaseLabelChanged(podFalse, podTrue)).To(BeTrue())
		})

		It("returns false when the both objects have the label set to false", func() {
			Expect(hasAutoReleaseLabelChanged(podFalse, podFalse)).To(BeFalse())
		})

		It("returns false when the both objects have the label set to true", func() {
			Expect(hasAutoReleaseLabelChanged(podTrue, podTrue)).To(BeFalse())
		})

		It("returns false when the both objects are missing the label", func() {
			Expect(hasAutoReleaseLabelChanged(podMissing, podMissing)).To(BeFalse())
		})
	})
})
