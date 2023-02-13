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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type conditionValues struct {
	status  metav1.ConditionStatus
	reason  ReleaseReason
	message string
}

var _ = Describe("Release type", func() {

	var r *Release

	BeforeEach(func() {
		r = &Release{}
		r.Status.StartTime = &metav1.Time{Time: time.Now().Add(-time.Hour * 1)}
		r.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		r.Status.Conditions = []metav1.Condition{{}}
	})

	Context("When ReleaseReason.String method is called", func() {
		It("should return the string representation", func() {
			var ReleaseReasonFake ReleaseReason = "Fake"
			Expect(ReleaseReasonFake.String()).To(Equal("Fake"))
		})

		It("should return an empty string when the ReleaseReason is empty", func() {
			var ReleaseEmptyReason ReleaseReason = ""
			Expect(ReleaseEmptyReason.String()).To(Equal(""))
		})
	})

	Context("When HasStarted method is called", func() {
		It("should return false when Status.startTime is nil", func() {
			r.Status.StartTime = nil
			Expect(r.HasStarted()).To(BeFalse())
		})

		It("should return false when Status.startTime is zero", func() {
			r.Status.StartTime = &metav1.Time{}
			Expect(r.HasStarted()).To(BeFalse())
		})

		It("should return true when Status.startTime is not nil or zero", func() {
			Expect(r.HasStarted()).To(BeTrue())
		})
	})

	Context("When HasSucceeded method is called", func() {
		It("should return false when condition.Status is ConditionUnknown", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:   releaseConditionType,
				Status: metav1.ConditionUnknown,
			}
			Expect(r.HasSucceeded()).To(BeFalse())
		})

		It("should return true when condition.Status is ConditionTrue", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:   releaseConditionType,
				Status: metav1.ConditionTrue,
			}
			Expect(r.HasSucceeded()).To(BeTrue())
		})
	})

	Context("When IsDeployed method is called", func() {
		It("should return true when AllComponentsDeployed condition status is True", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:   applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
				Status: metav1.ConditionTrue,
			}
			Expect(r.IsDeployed()).To(BeTrue())
		})

		It("should return false when AllComponentsDeployed condition status is False", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:   applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
				Status: metav1.ConditionFalse,
			}
			Expect(r.IsDeployed()).To(BeFalse())
		})

		It("should return false when AllComponentsDeployed condition status is Unknown", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:   applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
				Status: metav1.ConditionUnknown,
			}
			Expect(r.IsDeployed()).To(BeFalse())
		})
	})

	Context("When IsDeploying method is called", func() {
		It("should return false when Status.deploymentStartTime is nil", func() {
			r.Status.DeploymentStartTime = nil
			Expect(r.IsDeploying()).To(BeFalse())
		})

		It("should return false when Status.deploymentStartTime is zero", func() {
			r.Status.DeploymentStartTime = &metav1.Time{}
			Expect(r.IsDeploying()).To(BeFalse())
		})

		It("should return true when Status.deploymentStartTime is not nil or zero", func() {
			r.Status.DeploymentStartTime = &metav1.Time{Time: time.Now().Add(-time.Hour * 1)}
			Expect(r.IsDeploying()).To(BeTrue())
		})
	})

	Context("When IsDone method is called", func() {
		It("should return false when condition struct is nil", func() {
			Expect(r.IsDone()).To(BeFalse())
		})

		It("should return true when Release status is finished", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:   releaseConditionType,
				Status: metav1.ConditionTrue,
			}
			Expect(r.IsDone()).To(BeTrue())
		})

		It("should return false when condition.Status is ConditionUnknown", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:   releaseConditionType,
				Status: metav1.ConditionUnknown,
			}
			Expect(r.IsDone()).To(BeFalse())
		})
	})

	Context("When MarkDeployed method is called", func() {
		It("should do nothing if the Release is already deployed", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:    applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed,
				Status:  metav1.ConditionTrue,
				Reason:  "test",
				Message: "abc",
			}
			r.MarkDeployed("foo", "bar")
			Expect(len(r.Status.Conditions)).To(Equal(1))
			Expect(r.Status.Conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionTrue),
				"Type":    Equal(applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed),
				"Reason":  Equal("test"),
				"Message": Equal("abc"),
			}))
		})

		It("should properly register the status if passed status is true", func() {
			r.MarkDeploying(metav1.ConditionFalse, "CommitsUnsynced", "1 of 3 components deployed")
			r.MarkDeployed("CommitsSynced", "3 of 3 components deployed")
			statusCondition := meta.FindStatusCondition(r.Status.Conditions,
				applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed)
			Expect(*statusCondition).To(MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionTrue),
				"Reason":  Equal("CommitsSynced"),
				"Message": Equal("3 of 3 components deployed"),
			}))
		})
	})

	Context("When MarkDeploying method is called", func() {
		It("should properly register the status to the release if passed status is false", func() {
			r.MarkDeploying(metav1.ConditionFalse, "CommitsUnsynced", "1 of 3 components deployed")
			statusCondition := meta.FindStatusCondition(r.Status.Conditions,
				applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed)
			Expect(*statusCondition).To(MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionFalse),
				"Reason":  Equal("CommitsUnsynced"),
				"Message": Equal("1 of 3 components deployed"),
			}))
		})

		It("should properly register the status to the release if passed status is unknown", func() {
			r.MarkDeploying(metav1.ConditionUnknown, "CommitsUnsynced", "0 of 3 components deployed")
			statusCondition := meta.FindStatusCondition(r.Status.Conditions,
				applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed)
			Expect(*statusCondition).To(MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionUnknown),
				"Reason":  Equal("CommitsUnsynced"),
				"Message": Equal("0 of 3 components deployed"),
			}))
		})

		It("should not change the release status if passed status that is true", func() {
			r.MarkDeploying(metav1.ConditionTrue, "CommitsSynced", "3 of 3 components deployed")
			statusCondition := meta.FindStatusCondition(r.Status.Conditions,
				applicationapiv1alpha1.ComponentDeploymentConditionAllComponentsDeployed)
			Expect(statusCondition).To(BeNil())
		})
	})

	Context("When MarkFailed method is called", func() {
		It("should do nothing if the Release is finished", func() {
			args := conditionValues{
				reason:  ReleaseReasonValidationError,
				message: "cow say m00",
			}
			r.Status.Conditions[0] = metav1.Condition{
				Type:    releaseConditionType,
				Status:  metav1.ConditionTrue,
				Reason:  ReleaseReasonTargetDisabledError.String(),
				Message: "Testcase one message string",
			}
			r.MarkFailed(args.reason, args.message)
			Expect(len(r.Status.Conditions)).To(Equal(1))
			Expect(r.Status.Conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionTrue),
				"Type":    Equal(releaseConditionType),
				"Reason":  Equal(ReleaseReasonTargetDisabledError.String()),
				"Message": Equal("Testcase one message string"),
			}))
		})

		It("should register the Failed condition when the Release is not complete", func() {
			args := conditionValues{
				reason:  ReleaseReasonValidationError,
				message: "what does the fox say",
			}
			r.Status.CompletionTime = nil
			r.Status.Conditions[0] = metav1.Condition{
				Type:    releaseConditionType,
				Status:  metav1.ConditionUnknown,
				Reason:  ReleaseReasonTargetDisabledError.String(),
				Message: "Testcase two message string",
			}
			r.MarkFailed(args.reason, args.message)
			Expect(len(r.Status.Conditions)).To(Equal(1))
			Expect(r.Status.Conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionFalse),
				"Type":    Equal(releaseConditionType),
				"Reason":  Equal(ReleaseReasonValidationError.String()),
				"Message": Equal("what does the fox say"),
			}))
		})
	})

	Context("When MarkInvalid method is called", func() {
		It("should register the Invalid status when the Release is not finished", func() {
			args := conditionValues{
				reason:  ReleaseReasonPipelineFailed,
				message: "what does the fox say",
			}
			r.Status.Conditions[0] = metav1.Condition{
				Type:    "Fail",
				Status:  metav1.ConditionUnknown,
				Reason:  ReleaseReasonReleasePlanValidationError.String(),
				Message: "message string",
			}
			r.MarkInvalid(args.reason, args.message)
			Expect(len(r.Status.Conditions)).To(Equal(2))
			Expect(r.Status.Conditions[1]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionFalse),
				"Type":    Equal(releaseConditionType),
				"Reason":  Equal(ReleaseReasonPipelineFailed.String()),
				"Message": Equal("what does the fox say"),
			}))
			Expect(r.Status.Conditions[0].Message).To(Equal("message string"))
		})

		It("should not register the Invalid status when the Release status is already finished", func() {
			args := conditionValues{
				reason:  ReleaseReasonTargetDisabledError,
				message: "how now brown cow",
			}
			r.Status.Conditions[0] = metav1.Condition{
				Type:    releaseConditionType,
				Status:  metav1.ConditionTrue,
				Reason:  ReleaseReasonValidationError.String(),
				Message: "message string",
			}
			r.MarkInvalid(args.reason, args.message)
			Expect(len(r.Status.Conditions)).To(Equal(1))
			Expect(r.Status.Conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionTrue),
				"Type":    Equal(releaseConditionType),
				"Reason":  Equal(ReleaseReasonValidationError.String()),
				"Message": Equal("message string"),
			}))
		})
	})

	Context("When MarkRunning method is called", func() {
		It("should do nothing when the Release is already running", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:    "fake type",
				Status:  metav1.ConditionFalse,
				Reason:  ReleaseReasonPipelineFailed.String(),
				Message: "fake message",
			}
			r.MarkRunning()
			Expect(r.Status.StartTime).ToNot(Equal(time.Time{}))
			Expect(r.Status.StartTime).ToNot(BeNil())
			Expect(len(r.Status.Conditions)).To(Equal(1))
			Expect(r.Status.Conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionFalse),
				"Type":    Equal("fake type"),
				"Reason":  Equal(ReleaseReasonPipelineFailed.String()),
				"Message": Equal("fake message"),
			}))
		})

		It("should register the Running status when the Release is not running", func() {
			r.Status.StartTime = nil
			r.Status.StartTime = &metav1.Time{
				Time: time.Time{},
			}
			r.MarkRunning()
			Expect(r.Status.StartTime.Time).ToNot(Equal(time.Time{}))
			Expect(r.Status.StartTime).ToNot(BeNil())
			Expect(len(r.Status.Conditions)).To(Equal(2))
			Expect(r.Status.Conditions[1]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionUnknown),
				"Reason":  Equal(ReleaseReasonRunning.String()),
				"Message": Equal(""),
			}))
		})

		It("should register the Running status when the Release is not running", func() {
			r.Status.StartTime = nil
			r.MarkRunning()
			Expect(r.Status.StartTime.Time).ToNot(Equal(time.Time{}))
			Expect(r.Status.StartTime).ToNot(BeNil())
			Expect(len(r.Status.Conditions)).To(Equal(2))
			Expect(r.Status.Conditions[1]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionUnknown),
				"Reason":  Equal(ReleaseReasonRunning.String()),
				"Message": Equal(""),
			}))
		})
	})

	Context("When MarkSucceeded method is called", func() {
		It("should do nothing when when the Release is already successful", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:    releaseConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  ReleaseReasonTargetDisabledError.String(),
				Message: "lucy in the sky with diamonds",
			}
			rr := r.DeepCopy()
			r.MarkSucceeded()
			Expect(r.Status.StartTime).ToNot(BeNil())
			Expect(r.Status.CompletionTime).ToNot(BeNil())
			Expect(r.Status.CompletionTime.Time).To(BeTemporally(">=", rr.Status.CompletionTime.Time))
			Expect(r.Status.CompletionTime.Time).To(BeTemporally(">", r.Status.StartTime.Time))
			Expect(len(r.Status.Conditions)).To(Equal(1))
			Expect(r.Status.Conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionFalse),
				"Type":    Equal(releaseConditionType),
				"Reason":  Equal(ReleaseReasonTargetDisabledError.String()),
				"Message": Equal("lucy in the sky with diamonds"),
			}))
			rr = nil
		})

		It("register the Succeeded status when the Release is completed", func() {
			r.Status.Conditions[0] = metav1.Condition{
				Type:    "Fail",
				Status:  metav1.ConditionUnknown,
				Reason:  ReleaseReasonValidationError.String(),
				Message: "all your base belong to us",
			}
			rr := r.DeepCopy()
			r.MarkSucceeded()
			Expect(r.Status.CompletionTime).ToNot(BeNil())
			Expect(r.Status.CompletionTime.Time).To(BeTemporally(">=", rr.Status.CompletionTime.Time))
			Expect(r.Status.CompletionTime.Time).To(BeTemporally(">", r.Status.StartTime.Time))
			Expect(len(r.Status.Conditions)).To(Equal(2))
			Expect(r.Status.Conditions[1]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(metav1.ConditionTrue),
				"Type":    Equal(releaseConditionType),
				"Reason":  Equal(ReleaseReasonSucceeded.String()),
				"Message": Equal(""),
			}))
			Expect(r.Status.Conditions[0].Message).To(Equal("all your base belong to us"))
			rr = nil
		})
	})

	Context("When setStatusCondition method is called", func() {
		It("should update condition with provided arguments, and empty message", func() {
			args := conditionValues{
				status: metav1.ConditionStatus("fake"),
				reason: ReleaseReasonPipelineFailed,
			}
			r.Status.Conditions[0] = metav1.Condition{
				Type:    releaseConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  ReleaseReasonTargetDisabledError.String(),
				Message: "lucy in the sky with diamonds",
			}
			r.setStatusCondition(releaseConditionType, args.status, args.reason)
			Expect(len(r.Status.Conditions)).To(Equal(1))
			Expect(r.Status.Conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(args.status),
				"Reason":  Equal(args.reason.String()),
				"Message": Equal(""),
			}))
		})
	})

	Context("When setStatusConditionWithMessage method is called", func() {
		It("should update condition with provided arguments", func() {
			args := conditionValues{
				status:  metav1.ConditionStatus("fake"),
				reason:  ReleaseReasonValidationError,
				message: "fake",
			}
			r.Status.Conditions[0] = metav1.Condition{
				Type:    releaseConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  ReleaseReasonPipelineFailed.String(),
				Message: "lucy in the sky with diamonds",
			}
			r.setStatusConditionWithMessage(releaseConditionType, args.status, args.reason, args.message)
			Expect(len(r.Status.Conditions)).To(Equal(1))
			Expect(r.Status.Conditions[0]).To(MatchFields(IgnoreMissing|IgnoreExtras, Fields{
				"Status":  Equal(args.status),
				"Reason":  Equal(args.reason.String()),
				"Message": Equal(args.message),
			}))
		})
	})
})
