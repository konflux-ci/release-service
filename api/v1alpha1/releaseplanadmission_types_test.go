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

	"github.com/konflux-ci/release-service/metadata"

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

	When("IsRetryEnabled method is called", func() {
		It("should return false when RetryInfo is nil", func() {
			rpa := &ReleasePlanAdmission{}
			Expect(rpa.IsRetryEnabled()).To(BeFalse())
		})

		It("should return false when retries are not enabled", func() {
			rpa := &ReleasePlanAdmission{}
			rpa.Status.RetryInfo = &RetryInfo{Enabled: false}
			Expect(rpa.IsRetryEnabled()).To(BeFalse())
		})

		It("should return false when MaxRetries is nil", func() {
			rpa := &ReleasePlanAdmission{}
			rpa.Status.RetryInfo = &RetryInfo{Enabled: true}
			Expect(rpa.IsRetryEnabled()).To(BeFalse())
		})

		It("should return true when enabled with a valid MaxRetries", func() {
			maxRetries := 3
			rpa := &ReleasePlanAdmission{}
			rpa.Status.RetryInfo = &RetryInfo{Enabled: true, MaxRetries: &maxRetries}
			Expect(rpa.IsRetryEnabled()).To(BeTrue())
		})
	})

	When("GetMitigations method is called", func() {
		It("should return nil when RetryInfo is nil", func() {
			rpa := &ReleasePlanAdmission{}
			Expect(rpa.GetMitigations()).To(BeNil())
		})

		It("should return nil when no mitigations are configured", func() {
			rpa := &ReleasePlanAdmission{}
			rpa.Status.RetryInfo = &RetryInfo{Enabled: true}
			Expect(rpa.GetMitigations()).To(BeNil())
		})

		It("should return the mitigations when configured", func() {
			rpa := &ReleasePlanAdmission{}
			rpa.Status.RetryInfo = &RetryInfo{
				Mitigations: &Mitigations{
					OOMKill: &MemoryMitigation{Multiplier: "2"},
				},
			}
			mitigations := rpa.GetMitigations()
			Expect(mitigations).NotTo(BeNil())
			Expect(mitigations.OOMKill.Multiplier).To(Equal("2"))
		})
	})

	When("MatchesReleasePlan method is called", func() {
		It("should return true when RP Application is in RPA Applications", func() {
			releasePlan := &ReleasePlan{
				Spec: ReleasePlanSpec{
					Application: "my-app",
				},
			}
			releasePlanAdmission := &ReleasePlanAdmission{
				Spec: ReleasePlanAdmissionSpec{
					Applications: []string{"my-app", "other-app"},
				},
			}
			Expect(releasePlanAdmission.MatchesReleasePlan(releasePlan)).To(BeTrue())
		})

		It("should return true when RP ComponentGroup is in RPA ComponentGroups", func() {
			releasePlan := &ReleasePlan{
				Spec: ReleasePlanSpec{
					ComponentGroup: "my-group",
				},
			}
			releasePlanAdmission := &ReleasePlanAdmission{
				Spec: ReleasePlanAdmissionSpec{
					ComponentGroups: []string{"my-group", "other-group"},
				},
			}
			Expect(releasePlanAdmission.MatchesReleasePlan(releasePlan)).To(BeTrue())
		})

		It("should return true when RP Application is in RPA ComponentGroups (backward compatible)", func() {
			releasePlan := &ReleasePlan{
				Spec: ReleasePlanSpec{
					Application: "my-app",
				},
			}
			releasePlanAdmission := &ReleasePlanAdmission{
				Spec: ReleasePlanAdmissionSpec{
					ComponentGroups: []string{"my-app"},
				},
			}
			Expect(releasePlanAdmission.MatchesReleasePlan(releasePlan)).To(BeTrue())
		})

		It("should return true when RP ComponentGroup is in RPA Applications (backward compatible)", func() {
			releasePlan := &ReleasePlan{
				Spec: ReleasePlanSpec{
					ComponentGroup: "my-group",
				},
			}
			releasePlanAdmission := &ReleasePlanAdmission{
				Spec: ReleasePlanAdmissionSpec{
					Applications: []string{"my-group"},
				},
			}
			Expect(releasePlanAdmission.MatchesReleasePlan(releasePlan)).To(BeTrue())
		})

		It("should return false when RP Application is not in any RPA list", func() {
			releasePlan := &ReleasePlan{
				Spec: ReleasePlanSpec{
					Application: "my-app",
				},
			}
			releasePlanAdmission := &ReleasePlanAdmission{
				Spec: ReleasePlanAdmissionSpec{
					Applications: []string{"other-app"},
				},
			}
			Expect(releasePlanAdmission.MatchesReleasePlan(releasePlan)).To(BeFalse())
		})

		It("should return false when RP ComponentGroup is not in any RPA list", func() {
			releasePlan := &ReleasePlan{
				Spec: ReleasePlanSpec{
					ComponentGroup: "my-group",
				},
			}
			releasePlanAdmission := &ReleasePlanAdmission{
				Spec: ReleasePlanAdmissionSpec{
					ComponentGroups: []string{"other-group"},
				},
			}
			Expect(releasePlanAdmission.MatchesReleasePlan(releasePlan)).To(BeFalse())
		})
	})
})
