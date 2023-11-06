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

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/operator-toolkit/conditions"
	"github.com/redhat-appstudio/release-service/metadata"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ReleasePlan type", func() {
	When("IsMatched method is called", func() {
		var releasePlan *ReleasePlan

		BeforeEach(func() {
			releasePlan = &ReleasePlan{}
		})

		It("should return true when the matched condition status is True", func() {
			conditions.SetCondition(&releasePlan.Status.Conditions, MatchedConditionType, metav1.ConditionTrue, "")
			Expect(releasePlan.IsMatched()).To(BeTrue())
		})

		It("should return false when the matched condition status is False", func() {
			conditions.SetCondition(&releasePlan.Status.Conditions, MatchedConditionType, metav1.ConditionFalse, "")
			Expect(releasePlan.IsMatched()).To(BeFalse())
		})

		It("should return false when the matched condition status is Unknown", func() {
			conditions.SetCondition(&releasePlan.Status.Conditions, MatchedConditionType, metav1.ConditionUnknown, "")
			Expect(releasePlan.IsMatched()).To(BeFalse())
		})

		It("should return false when the matched condition is missing", func() {
			Expect(releasePlan.IsMatched()).To(BeFalse())
		})
	})

	When("MarkMatched method is called", func() {
		var releasePlan *ReleasePlan
		var releasePlanAdmission *ReleasePlanAdmission

		BeforeEach(func() {
			releasePlan = &ReleasePlan{}
			releasePlanAdmission = &ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rpa",
					Namespace: "default",
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "false",
					},
				},
			}
		})

		It("should mark the ReleasePlan as matched", func() {
			releasePlan.MarkMatched(releasePlanAdmission)
			Expect(releasePlan.Status.ReleasePlanAdmission.Name).To(Equal("default/rpa"))
			Expect(releasePlan.Status.ReleasePlanAdmission.Active).To(BeFalse())
			condition := meta.FindStatusCondition(releasePlan.Status.Conditions, MatchedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	When("MarkUnmatched method is called", func() {
		var releasePlan *ReleasePlan
		var releasePlanAdmission *ReleasePlanAdmission

		BeforeEach(func() {
			releasePlan = &ReleasePlan{}
			releasePlanAdmission = &ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rpa",
					Namespace: "default",
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "false",
					},
				},
			}
		})

		It("should do nothing if the ReleasePlan is not matched", func() {
			releasePlan.setMatchedStatus(releasePlanAdmission, metav1.ConditionFalse) // IsMatched relies on the condition, not value of RPA
			releasePlan.MarkUnmatched()
			Expect(releasePlan.Status.ReleasePlanAdmission.Name).To(Equal("default/rpa"))
		})

		It("should mark the ReleasePlan as unmatched", func() {
			releasePlan.MarkMatched(releasePlanAdmission)
			releasePlan.MarkUnmatched()
			Expect(releasePlan.Status.ReleasePlanAdmission).To(Equal(MatchedReleasePlanAdmission{}))
			condition := meta.FindStatusCondition(releasePlan.Status.Conditions, MatchedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		})
	})

	When("setMatchedStatus method is called", func() {
		var releasePlan *ReleasePlan
		var releasePlanAdmission *ReleasePlanAdmission

		BeforeEach(func() {
			releasePlan = &ReleasePlan{}
			releasePlanAdmission = &ReleasePlanAdmission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rpa",
					Namespace: "default",
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
			}
		})

		It("should set the ReleasePlanAdmission and matched condition", func() {
			releasePlan.setMatchedStatus(releasePlanAdmission, metav1.ConditionUnknown)
			Expect(releasePlan.Status.ReleasePlanAdmission.Name).To(Equal("default/rpa"))
			Expect(releasePlan.Status.ReleasePlanAdmission.Active).To(BeTrue())
			condition := meta.FindStatusCondition(releasePlan.Status.Conditions, MatchedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionUnknown))
		})
	})
})
