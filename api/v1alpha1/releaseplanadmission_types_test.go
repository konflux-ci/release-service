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

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/release-service/metadata"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ReleasePlanAdmission type", func() {
	When("ClearMatchingInfo method is called", func() {
		var releasePlan *ReleasePlan
		var releasePlanAdmission *ReleasePlanAdmission

		BeforeEach(func() {
			releasePlan = &ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rp",
					Namespace: "default",
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
			}
			releasePlanAdmission = &ReleasePlanAdmission{}
		})

		It("should wipe all releasePlans from the status and add matched condition as false", func() {
			condition := meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, MatchedConditionType.String())
			Expect(condition).To(BeNil())
			releasePlanAdmission.Status.ReleasePlans = []MatchedReleasePlan{{Name: "rp"}}

			releasePlanAdmission.ClearMatchingInfo()

			Expect(releasePlanAdmission.Status.ReleasePlans).To(Equal([]MatchedReleasePlan{}))
			condition = meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, MatchedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		})

		It("should switch the condition from true to false", func() {
			releasePlanAdmission.MarkMatched(releasePlan)
			condition := meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, MatchedConditionType.String())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))

			releasePlanAdmission.ClearMatchingInfo()
			condition = meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, MatchedConditionType.String())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		})
	})

	When("MarkMatched method is called", func() {
		var releasePlan *ReleasePlan
		var releasePlanAdmission *ReleasePlanAdmission

		BeforeEach(func() {
			releasePlan = &ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rp",
					Namespace: "default",
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
			}
			releasePlanAdmission = &ReleasePlanAdmission{}
		})

		It("should add matched condition as true and add the releasePlan to the status", func() {
			condition := meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, MatchedConditionType.String())
			Expect(condition).To(BeNil())

			releasePlanAdmission.MarkMatched(releasePlan)

			Expect(releasePlanAdmission.Status.ReleasePlans).To(Equal([]MatchedReleasePlan{{Name: "default/rp", Active: true}}))
			condition = meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, MatchedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		})

		It("should switch the condition from false to true", func() {
			releasePlanAdmission.ClearMatchingInfo()
			condition := meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, MatchedConditionType.String())
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))

			releasePlanAdmission.MarkMatched(releasePlan)
			condition = meta.FindStatusCondition(releasePlanAdmission.Status.Conditions, MatchedConditionType.String())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		})

		It("should store the ReleasePlans in sorted order by Name", func() {
			releasePlan2 := &ReleasePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "r",
					Namespace: "default",
					Labels: map[string]string{
						metadata.AutoReleaseLabel: "true",
					},
				},
			}

			releasePlanAdmission.MarkMatched(releasePlan)
			releasePlanAdmission.MarkMatched(releasePlan2)

			Expect(releasePlanAdmission.Status.ReleasePlans).To(Equal([]MatchedReleasePlan{
				{Name: "default/r", Active: true},
				{Name: "default/rp", Active: true},
			}))
		})
	})
})
