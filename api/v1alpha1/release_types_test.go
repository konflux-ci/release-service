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
	. "github.com/onsi/gomega/gstruct"
	"github.com/redhat-appstudio/operator-toolkit/conditions"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Release type", func() {

	When("HasDeploymentFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the deployed condition is missing", func() {
			Expect(release.HasDeploymentFinished()).To(BeFalse())
		})

		It("should return true when the deployed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasDeploymentFinished()).To(BeTrue())
		})

		It("should return false when the deployed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasDeploymentFinished()).To(BeFalse())
		})

		It("should return true when the deployed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasDeploymentFinished()).To(BeTrue())
		})

		It("should return false when the deployed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasDeploymentFinished()).To(BeFalse())
		})
	})

	When("HasEveryPostActionExecutionFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the post-actions executed condition is missing", func() {
			Expect(release.HasEveryPostActionExecutionFinished()).To(BeFalse())
		})

		It("should return true when the post-actions executed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasEveryPostActionExecutionFinished()).To(BeTrue())
		})

		It("should return false when the post-actions executed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasEveryPostActionExecutionFinished()).To(BeFalse())
		})

		It("should return true when the post-actions executed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasEveryPostActionExecutionFinished()).To(BeTrue())
		})

		It("should return false when the post-actions executed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasEveryPostActionExecutionFinished()).To(BeFalse())
		})
	})

	When("HasProcessingFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the deployed condition is missing", func() {
			Expect(release.HasProcessingFinished()).To(BeFalse())
		})

		It("should return true when the processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasProcessingFinished()).To(BeTrue())
		})

		It("should return false when the processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasProcessingFinished()).To(BeFalse())
		})

		It("should return true when the processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasProcessingFinished()).To(BeTrue())
		})

		It("should return false when the processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasProcessingFinished()).To(BeFalse())
		})
	})

	When("HasReleaseFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the released condition is missing", func() {
			Expect(release.HasReleaseFinished()).To(BeFalse())
		})

		It("should return true when the released condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasReleaseFinished()).To(BeTrue())
		})

		It("should return false when the released condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasReleaseFinished()).To(BeFalse())
		})

		It("should return true when the released condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasReleaseFinished()).To(BeTrue())
		})

		It("should return false when the released condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasReleaseFinished()).To(BeFalse())
		})
	})

	When("IsAttributed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when there is an author in the release status", func() {
			release.Status.Attribution.Author = "user"
			Expect(release.IsAttributed()).To(BeTrue())
		})

		It("should return false when there is no author in the release status", func() {
			Expect(release.IsAttributed()).To(BeFalse())
		})
	})

	When("IsAutomated method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the automated field in the status is True", func() {
			release.SetAutomated()
			Expect(release.IsAutomated()).To(BeTrue())
		})

		It("should return false when the automated field in the status is False", func() {
			release.Status.Automated = false
			Expect(release.IsAutomated()).To(BeFalse())
		})

		It("should return false when the automated field in the status is missing", func() {
			Expect(release.IsAutomated()).To(BeFalse())
		})
	})

	When("IsDeployed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the deployed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsDeployed()).To(BeTrue())
		})

		It("should return false when the deployed condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsDeployed()).To(BeFalse())
		})

		It("should return false when the deployed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsDeployed()).To(BeFalse())
		})

		It("should return false when the deployed condition is missing", func() {
			Expect(release.IsDeployed()).To(BeFalse())
		})
	})

	When("IsDeploying method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the deployed condition is missing", func() {
			Expect(release.IsDeploying()).To(BeFalse())
		})

		It("should return false when the deployed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsDeploying()).To(BeFalse())
		})

		It("should return true when the deployed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsDeploying()).To(BeTrue())
		})

		It("should return false when the deployed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsDeploying()).To(BeFalse())
		})

		It("should return false when the deployed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.IsDeploying()).To(BeFalse())
		})
	})

	When("IsEveryPostActionExecuted method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the post-actions executed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsEveryPostActionExecuted()).To(BeTrue())
		})

		It("should return false when the post-actions executed condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsEveryPostActionExecuted()).To(BeFalse())
		})

		It("should return false when the post-actions executed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsEveryPostActionExecuted()).To(BeFalse())
		})

		It("should return false when the post-actions executed condition is missing", func() {
			Expect(release.IsEveryPostActionExecuted()).To(BeFalse())
		})
	})

	When("IsEachPostActionExecuting method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the post-actions executed condition is missing", func() {
			Expect(release.IsEachPostActionExecuting()).To(BeFalse())
		})

		It("should return false when the post-actions executed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsEachPostActionExecuting()).To(BeFalse())
		})

		It("should return true when the post-actions executed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsEachPostActionExecuting()).To(BeTrue())
		})

		It("should return false when the post-actions executed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsEachPostActionExecuting()).To(BeFalse())
		})

		It("should return false when the post-actions executed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, postActionsExecutedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.IsEachPostActionExecuting()).To(BeFalse())
		})
	})

	When("IsProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsProcessed()).To(BeTrue())
		})

		It("should return false when the processed condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsProcessed()).To(BeFalse())
		})

		It("should return false when the processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsProcessed()).To(BeFalse())
		})

		It("should return false when the processed condition is missing", func() {
			Expect(release.IsProcessed()).To(BeFalse())
		})
	})

	When("IsProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the processed condition is missing", func() {
			Expect(release.IsProcessing()).To(BeFalse())
		})

		It("should return false when the processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsProcessing()).To(BeFalse())
		})

		It("should return true when the processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsProcessing()).To(BeTrue())
		})

		It("should return false when the processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsProcessing()).To(BeFalse())
		})

		It("should return false when the processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, processedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.IsProcessing()).To(BeFalse())
		})
	})

	When("IsReleased method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the released condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsReleased()).To(BeTrue())
		})

		It("should return false when the released condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsReleased()).To(BeFalse())
		})

		It("should return false when the released condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsReleased()).To(BeFalse())
		})

		It("should return false when the released condition is missing", func() {
			Expect(release.IsReleased()).To(BeFalse())
		})
	})

	When("IsReleasing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the released condition is missing", func() {
			Expect(release.IsReleasing()).To(BeFalse())
		})

		It("should return false when the released condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsReleasing()).To(BeFalse())
		})

		It("should return true when the released condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsReleasing()).To(BeTrue())
		})

		It("should return false when the released condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsReleasing()).To(BeFalse())
		})

		It("should return false when the released condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.IsReleasing()).To(BeFalse())
		})
	})

	When("IsValid method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the validated condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsValid()).To(BeTrue())
		})

		It("should return false when the validated condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsValid()).To(BeFalse())
		})

		It("should return false when the validated condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsValid()).To(BeFalse())
		})

		It("should return false when the validated condition is missing", func() {
			Expect(release.IsValid()).To(BeFalse())
		})
	})

	When("MarkDeployed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release deployment has not started", func() {
			release.MarkDeployed()
			Expect(release.Status.Deployment.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release deployment finished", func() {
			release.MarkDeploying("")
			release.MarkDeployed()
			Expect(release.Status.Deployment.CompletionTime.IsZero()).To(BeFalse())
			release.Status.Deployment.CompletionTime = &metav1.Time{}
			release.MarkDeployed()
			Expect(release.Status.Deployment.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkDeploying("")
			Expect(release.Status.Deployment.CompletionTime.IsZero()).To(BeTrue())
			release.MarkDeployed()
			Expect(release.Status.Deployment.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkDeploying("")
			release.MarkDeployed()

			condition := meta.FindStatusCondition(release.Status.Conditions, deployedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkDeploying method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release deployment finished", func() {
			release.MarkDeploying("")
			release.MarkDeployed()
			Expect(release.IsDeploying()).To(BeFalse())
			release.MarkDeploying("")
			Expect(release.IsDeploying()).To(BeFalse())
		})

		It("should register the start time if it's not deploying", func() {
			Expect(release.Status.Deployment.StartTime).To(BeNil())
			release.MarkDeploying("")
			Expect(release.Status.Deployment.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if it's deploying already", func() {
			Expect(release.Status.Deployment.StartTime).To(BeNil())
			release.MarkDeploying("")
			release.Status.Deployment.StartTime = &metav1.Time{}
			Expect(release.Status.Deployment.StartTime.IsZero()).To(BeTrue())
			release.MarkDeploying("")
			Expect(release.Status.Deployment.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkDeploying("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, deployedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(ProgressingReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkDeploymentFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release deployment has not started", func() {
			release.MarkDeploymentFailed("")
			Expect(release.Status.Deployment.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release deployment finished", func() {
			release.MarkDeploying("")
			release.MarkDeployed()
			Expect(release.Status.Deployment.CompletionTime.IsZero()).To(BeFalse())
			release.Status.Deployment.CompletionTime = &metav1.Time{}
			release.MarkDeploymentFailed("")
			Expect(release.Status.Deployment.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkDeploying("")
			Expect(release.Status.Deployment.CompletionTime.IsZero()).To(BeTrue())
			release.MarkDeploymentFailed("")
			Expect(release.Status.Deployment.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkDeploying("")
			release.MarkDeploymentFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, deployedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release processing has not started", func() {
			release.MarkProcessed()
			Expect(release.Status.Processing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release processing finished", func() {
			release.MarkProcessing("")
			release.MarkProcessed()
			Expect(release.Status.Processing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.Processing.CompletionTime = &metav1.Time{}
			release.MarkProcessed()
			Expect(release.Status.Processing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkProcessing("")
			Expect(release.Status.Processing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkProcessed()
			Expect(release.Status.Processing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkProcessing("")
			release.MarkProcessed()

			condition := meta.FindStatusCondition(release.Status.Conditions, processedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release processing finished", func() {
			release.MarkProcessing("")
			release.MarkProcessed()
			Expect(release.IsProcessing()).To(BeFalse())
			release.MarkProcessing("")
			Expect(release.IsProcessing()).To(BeFalse())
		})

		It("should register the start time if it's not processing", func() {
			Expect(release.Status.Processing.StartTime).To(BeNil())
			release.MarkProcessing("")
			Expect(release.Status.Processing.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if it's processing already", func() {
			Expect(release.Status.Processing.StartTime).To(BeNil())
			release.MarkProcessing("")
			release.Status.Processing.StartTime = &metav1.Time{}
			Expect(release.Status.Processing.StartTime.IsZero()).To(BeTrue())
			release.MarkProcessing("")
			Expect(release.Status.Processing.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkProcessing("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, processedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(ProgressingReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkProcessingFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release processing has not started", func() {
			release.MarkProcessingFailed("")
			Expect(release.Status.Processing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release processing finished", func() {
			release.MarkProcessing("")
			release.MarkProcessed()
			Expect(release.Status.Processing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.Processing.CompletionTime = &metav1.Time{}
			release.MarkProcessingFailed("")
			Expect(release.Status.Processing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkProcessing("")
			Expect(release.Status.Processing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkProcessingFailed("")
			Expect(release.Status.Processing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkProcessing("")
			release.MarkProcessingFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, processedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkPostActionsExecuted method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release post-actions execution has not started", func() {
			release.MarkPostActionsExecuted()
			Expect(release.Status.PostActionsExecution.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release post-actions execution finished", func() {
			release.MarkPostActionsExecuting("")
			release.MarkPostActionsExecuted()
			Expect(release.Status.PostActionsExecution.CompletionTime.IsZero()).To(BeFalse())
			release.Status.PostActionsExecution.CompletionTime = &metav1.Time{}
			release.MarkPostActionsExecuted()
			Expect(release.Status.PostActionsExecution.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkPostActionsExecuting("")
			Expect(release.Status.PostActionsExecution.CompletionTime.IsZero()).To(BeTrue())
			release.MarkPostActionsExecuted()
			Expect(release.Status.PostActionsExecution.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkPostActionsExecuting("")
			release.MarkPostActionsExecuted()

			condition := meta.FindStatusCondition(release.Status.Conditions, postActionsExecutedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkPostActionsExecuting method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release post-actions execution finished", func() {
			release.MarkPostActionsExecuting("")
			release.MarkPostActionsExecuted()
			Expect(release.IsEachPostActionExecuting()).To(BeFalse())
			release.MarkPostActionsExecuting("")
			Expect(release.IsEachPostActionExecuting()).To(BeFalse())
		})

		It("should register the start time if it's not executing post-actions", func() {
			Expect(release.Status.PostActionsExecution.StartTime).To(BeNil())
			release.MarkPostActionsExecuting("")
			Expect(release.Status.PostActionsExecution.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if it's executing post-actions already", func() {
			Expect(release.Status.PostActionsExecution.StartTime).To(BeNil())
			release.MarkPostActionsExecuting("")
			release.Status.PostActionsExecution.StartTime = &metav1.Time{}
			Expect(release.Status.PostActionsExecution.StartTime.IsZero()).To(BeTrue())
			release.MarkPostActionsExecuting("")
			Expect(release.Status.PostActionsExecution.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkPostActionsExecuting("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, postActionsExecutedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(ProgressingReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkPostActionsExecutionFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release post-actions execution has not started", func() {
			release.MarkPostActionsExecutionFailed("")
			Expect(release.Status.PostActionsExecution.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release post-actions execution finished", func() {
			release.MarkPostActionsExecuting("")
			release.MarkPostActionsExecuted()
			Expect(release.Status.PostActionsExecution.CompletionTime.IsZero()).To(BeFalse())
			release.Status.PostActionsExecution.CompletionTime = &metav1.Time{}
			release.MarkPostActionsExecutionFailed("")
			Expect(release.Status.PostActionsExecution.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkPostActionsExecuting("")
			Expect(release.Status.PostActionsExecution.CompletionTime.IsZero()).To(BeTrue())
			release.MarkPostActionsExecutionFailed("")
			Expect(release.Status.PostActionsExecution.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkPostActionsExecuting("")
			release.MarkPostActionsExecutionFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, postActionsExecutedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkReleased method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release has not started", func() {
			release.MarkReleased()
			Expect(release.Status.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release has finished", func() {
			release.MarkReleasing("")
			release.MarkReleased()
			Expect(release.Status.CompletionTime.IsZero()).To(BeFalse())
			release.Status.CompletionTime = &metav1.Time{}
			release.MarkReleased()
			Expect(release.Status.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkReleasing("")
			Expect(release.Status.CompletionTime.IsZero()).To(BeTrue())
			release.MarkReleased()
			Expect(release.Status.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkReleasing("")
			release.MarkReleased()

			condition := meta.FindStatusCondition(release.Status.Conditions, releasedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkReleasing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release finished", func() {
			release.MarkReleasing("")
			release.MarkReleased()
			Expect(release.IsReleasing()).To(BeFalse())
			release.MarkReleasing("")
			Expect(release.IsReleasing()).To(BeFalse())
		})

		It("should register the start time if it's not releasing", func() {
			Expect(release.Status.StartTime).To(BeNil())
			release.MarkReleasing("")
			Expect(release.Status.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if it's releasing already", func() {
			Expect(release.Status.StartTime).To(BeNil())
			release.MarkReleasing("")
			release.Status.StartTime = &metav1.Time{}
			Expect(release.Status.StartTime.IsZero()).To(BeTrue())
			release.MarkReleasing("")
			Expect(release.Status.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkReleasing("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, releasedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(ProgressingReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkReleaseFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release has not started", func() {
			release.MarkReleaseFailed("")
			Expect(release.Status.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release has finished", func() {
			release.MarkReleasing("")
			release.MarkReleased()
			Expect(release.Status.CompletionTime.IsZero()).To(BeFalse())
			release.Status.CompletionTime = &metav1.Time{}
			release.MarkReleaseFailed("")
			Expect(release.Status.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkReleasing("")
			Expect(release.Status.CompletionTime.IsZero()).To(BeTrue())
			release.MarkReleaseFailed("")
			Expect(release.Status.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkReleasing("")
			release.MarkReleaseFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, releasedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkValidated method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release is valid", func() {
			release.MarkValidated()
			Expect(release.Status.Validation.Time.IsZero()).To(BeFalse())
			release.Status.Validation.Time = &metav1.Time{}
			release.MarkValidated()
			Expect(release.Status.Validation.Time.IsZero()).To(BeTrue())
		})

		It("should register the validation time", func() {
			Expect(release.Status.Validation.Time.IsZero()).To(BeTrue())
			release.MarkValidated()
			Expect(release.Status.Validation.Time.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkValidated()

			condition := meta.FindStatusCondition(release.Status.Conditions, validatedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkValidationFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should not register the post-validation failure if it was not marked as valid before", func() {
			release.MarkValidationFailed("")
			Expect(release.Status.Validation.FailedPostValidation).To(BeFalse())
		})

		It("should register the post-validation failure if it was marked as valid before", func() {
			release.MarkValidated()
			release.MarkValidationFailed("")
			Expect(release.Status.Validation.FailedPostValidation).To(BeTrue())
		})

		It("should register the validation time", func() {
			Expect(release.Status.Validation.Time).To(BeNil())
			release.MarkValidationFailed("")
			Expect(release.Status.Validation.Time.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkValidationFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, validatedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("SetAutomated method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should set the automated field in the status to True", func() {
			release.SetAutomated()
			Expect(release.Status.Automated).To(BeTrue())
		})
	})

	When("getPhaseReason method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("returns the reason associated with the condition type", func() {
			release.MarkValidated()
			Expect(release.getPhaseReason(validatedConditionType)).To(Equal(SucceededReason.String()))
		})

		It("returns an empty string if the condition is not found", func() {
			Expect(release.getPhaseReason(validatedConditionType)).To(Equal(""))
		})
	})

	When("hasPhaseFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the condition is missing", func() {
			Expect(release.hasPhaseFinished(deployedConditionType)).To(BeFalse())
		})

		It("should return true when the condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.hasPhaseFinished(deployedConditionType)).To(BeTrue())
		})

		It("should return false when the condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.hasPhaseFinished(deployedConditionType)).To(BeFalse())
		})

		It("should return true when the condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.hasPhaseFinished(deployedConditionType)).To(BeTrue())
		})

		It("should return false when the condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.hasPhaseFinished(deployedConditionType)).To(BeFalse())
		})
	})

	When("isPhaseProgressing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the condition is missing", func() {
			Expect(release.isPhaseProgressing(deployedConditionType)).To(BeFalse())
		})

		It("should return false when the condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.isPhaseProgressing(deployedConditionType)).To(BeFalse())
		})

		It("should return true when the condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.isPhaseProgressing(deployedConditionType)).To(BeTrue())
		})

		It("should return false when the condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.isPhaseProgressing(deployedConditionType)).To(BeFalse())
		})

		It("should return false when the condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, deployedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.isPhaseProgressing(deployedConditionType)).To(BeFalse())
		})
	})

})
