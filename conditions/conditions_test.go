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

package conditions

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Release type", func() {

	When("setStatusCondition method is called", func() {
		It("should update the condition with provided the arguments and an empty message", func() {
			var conditions []metav1.Condition
			SetCondition(&conditions, "conditionType", metav1.ConditionTrue, "conditionReason")
			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionTrue),
				"Reason":  Equal("conditionReason"),
				"Message": Equal(""),
			}))
		})
	})

	When("setStatusConditionWithMessage method is called", func() {
		It("should update condition with provided arguments", func() {
			var conditions []metav1.Condition
			SetConditionWithMessage(&conditions, "conditionType", metav1.ConditionTrue, "conditionReason", "message")
			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionTrue),
				"Reason":  Equal("conditionReason"),
				"Message": Equal("message"),
			}))
		})
	})

})
