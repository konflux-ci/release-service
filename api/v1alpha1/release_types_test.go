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
	"time"

	"github.com/konflux-ci/operator-toolkit/conditions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Release type", func() {

	When("HasFinalPipelineProcessingFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the final pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasFinalPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the final pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasFinalPipelineProcessingFinished()).To(BeFalse())
		})

		It("Should return true when the final pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasFinalPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the final pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasFinalPipelineProcessingFinished()).To(BeFalse())
		})
	})

	When("HasManagedCollectorsPipelineProcessingFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the managed collectors pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasManagedCollectorsPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the managed collectors pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasManagedCollectorsPipelineProcessingFinished()).To(BeFalse())
		})

		It("Should return true when the managed collectors pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasManagedCollectorsPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the managed collectors pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasManagedCollectorsPipelineProcessingFinished()).To(BeFalse())
		})
	})

	When("HasManagedPipelineProcessingFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the managed pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasManagedPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the managed pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasManagedPipelineProcessingFinished()).To(BeFalse())
		})

		It("should return true when the managed pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasManagedPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the managed pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasManagedPipelineProcessingFinished()).To(BeFalse())
		})
	})

	When("HasTenantCollectorsPipelineProcessingFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the tenant collectors pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasTenantCollectorsPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the tenant collectors pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasTenantCollectorsPipelineProcessingFinished()).To(BeFalse())
		})

		It("Should return true when the tenant collectors pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasTenantCollectorsPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the tenant collectors pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasTenantCollectorsPipelineProcessingFinished()).To(BeFalse())
		})
	})

	When("HasTenantPipelineProcessingFinished method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the tenant pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.HasTenantPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the tenant pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.HasTenantPipelineProcessingFinished()).To(BeFalse())
		})

		It("should return true when the tenant pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.HasTenantPipelineProcessingFinished()).To(BeTrue())
		})

		It("should return false when the tenant pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.HasTenantPipelineProcessingFinished()).To(BeFalse())
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
			conditions.SetCondition(&release.Status.Conditions, releasedConditionType, metav1.ConditionUnknown, ProgressingReason)
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

	When("IsFinalPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the final pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsFinalPipelineProcessed()).To(BeTrue())
		})

		It("should return false when the final pipeline processed condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsFinalPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the final pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsFinalPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the final pipeline processed condition is missing", func() {
			Expect(release.IsFinalPipelineProcessed()).To(BeFalse())
		})

	})

	When("IsManagedCollectorsPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the managed collectors pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsManagedCollectorsPipelineProcessed()).To(BeTrue())
		})

		It("should return false when the managed collectors pipeline processed condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsManagedCollectorsPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the managed collectors pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsManagedCollectorsPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the managed collectors pipeline processed condition is missing", func() {
			Expect(release.IsManagedCollectorsPipelineProcessed()).To(BeFalse())
		})
	})

	When("IsManagedPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the managed pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsManagedPipelineProcessed()).To(BeTrue())
		})

		It("should return false when the managed pipeline processed condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsManagedPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the managed pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsManagedPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the managed pipeline processed condition is missing", func() {
			Expect(release.IsManagedPipelineProcessed()).To(BeFalse())
		})
	})

	When("IsTenantCollectorsPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the tenant collectors pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsTenantCollectorsPipelineProcessed()).To(BeTrue())
		})

		It("should return false when the tenant collectors pipeline processed condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsTenantCollectorsPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the tenant collectors pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsTenantCollectorsPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the tenant collectors pipeline processed condition is missing", func() {
			Expect(release.IsTenantCollectorsPipelineProcessed()).To(BeFalse())
		})
	})

	When("IsTenantPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return true when the tenant pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsTenantPipelineProcessed()).To(BeTrue())
		})

		It("should return false when the tenant pipeline processed condition status is False", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionFalse, SucceededReason)
			Expect(release.IsTenantPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the tenant pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionUnknown, SucceededReason)
			Expect(release.IsTenantPipelineProcessed()).To(BeFalse())
		})

		It("should return false when the tenant pipeline processed condition is missing", func() {
			Expect(release.IsTenantPipelineProcessed()).To(BeFalse())
		})
	})

	When("IsFinalPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the final pipeline processed condition is missing", func() {
			Expect(release.IsFinalPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the final pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsFinalPipelineProcessing()).To(BeFalse())
		})

		It("should return true when the final pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsFinalPipelineProcessing()).To(BeTrue())
		})

		It("should return false when the final pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, finalProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsFinalPipelineProcessing()).To(BeFalse())
		})

	})

	When("IsManagedCollectorsPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the managed collectors pipeline processed condition is missing", func() {
			Expect(release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the managed collectors pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should return true when the managed collectors pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsManagedCollectorsPipelineProcessing()).To(BeTrue())
		})

		It("should return false when the managed collectors pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the managed collectors pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, managedCollectorsProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
		})
	})

	When("IsManagedPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the managed pipeline processed condition is missing", func() {
			Expect(release.IsManagedPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the managed pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsManagedPipelineProcessing()).To(BeFalse())
		})

		It("should return true when the managed pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsManagedPipelineProcessing()).To(BeTrue())
		})

		It("should return false when the managed pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsManagedPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the managed pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, managedProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.IsManagedPipelineProcessing()).To(BeFalse())
		})
	})

	When("IsTenantCollectorsPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the tenant collectors pipeline processed condition is missing", func() {
			Expect(release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the tenant collectors pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should return true when the tenant collectors pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsTenantCollectorsPipelineProcessing()).To(BeTrue())
		})

		It("should return false when the tenant collectors pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the tenant collectors pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantCollectorsProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
		})
	})

	When("IsTenantPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the tenant pipeline processed condition is missing", func() {
			Expect(release.IsTenantPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the tenant pipeline processed condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.IsTenantPipelineProcessing()).To(BeFalse())
		})

		It("should return true when the tenant pipeline processed condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.IsTenantPipelineProcessing()).To(BeTrue())
		})

		It("should return false when the tenant pipeline processed condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.IsTenantPipelineProcessing()).To(BeFalse())
		})

		It("should return false when the tenant pipeline processed condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, tenantProcessedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.IsTenantPipelineProcessing()).To(BeFalse())
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

	When("MarkFinalPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release final pipeline processing has not started", func() {
			release.MarkFinalPipelineProcessed()
			Expect(release.Status.FinalProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release final pipeline processing finished", func() {
			release.MarkFinalPipelineProcessing()
			release.MarkFinalPipelineProcessed()
			Expect(release.Status.FinalProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.FinalProcessing.CompletionTime = &metav1.Time{}
			release.MarkFinalPipelineProcessed()
			Expect(release.Status.FinalProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkFinalPipelineProcessing()
			Expect(release.Status.FinalProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkFinalPipelineProcessed()
			Expect(release.Status.FinalProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkFinalPipelineProcessing()
			release.MarkFinalPipelineProcessed()

			condition := meta.FindStatusCondition(release.Status.Conditions, finalProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkManagedCollectorsPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release managed collectors pipeline processing has not started", func() {
			release.MarkManagedCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release managed collectors pipeline processing finished", func() {
			release.MarkManagedCollectorsPipelineProcessing()
			release.MarkManagedCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime = &metav1.Time{}
			release.MarkManagedCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkManagedCollectorsPipelineProcessing()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkManagedCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkManagedCollectorsPipelineProcessing()
			release.MarkManagedCollectorsPipelineProcessed()

			condition := meta.FindStatusCondition(release.Status.Conditions, managedCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkManagedPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release managed pipeline processing has not started", func() {
			release.MarkManagedPipelineProcessed()
			Expect(release.Status.ManagedProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release managed pipeline processing finished", func() {
			release.MarkManagedPipelineProcessing()
			release.MarkManagedPipelineProcessed()
			Expect(release.Status.ManagedProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.ManagedProcessing.CompletionTime = &metav1.Time{}
			release.MarkManagedPipelineProcessed()
			Expect(release.Status.ManagedProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkManagedPipelineProcessing()
			Expect(release.Status.ManagedProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkManagedPipelineProcessed()
			Expect(release.Status.ManagedProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkManagedPipelineProcessing()
			release.MarkManagedPipelineProcessed()

			condition := meta.FindStatusCondition(release.Status.Conditions, managedProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkTenantCollectorsPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release tenant collectors pipeline processing has not started", func() {
			release.MarkTenantCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release tenant collectors pipeline processing finished", func() {
			release.MarkTenantCollectorsPipelineProcessing()
			release.MarkTenantCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime = &metav1.Time{}
			release.MarkTenantCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkTenantCollectorsPipelineProcessing()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkTenantCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkTenantCollectorsPipelineProcessing()
			release.MarkTenantCollectorsPipelineProcessed()

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkTenantPipelineProcessed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release tenant pipeline processing has not started", func() {
			release.MarkTenantPipelineProcessed()
			Expect(release.Status.TenantProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release tenant pipeline processing finished", func() {
			release.MarkTenantPipelineProcessing()
			release.MarkTenantPipelineProcessed()
			Expect(release.Status.TenantProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.TenantProcessing.CompletionTime = &metav1.Time{}
			release.MarkTenantPipelineProcessed()
			Expect(release.Status.TenantProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkTenantPipelineProcessing()
			Expect(release.Status.TenantProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkTenantPipelineProcessed()
			Expect(release.Status.TenantProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkTenantPipelineProcessing()
			release.MarkTenantPipelineProcessed()

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SucceededReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkFinalPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release final pipeline processing finished", func() {
			release.MarkFinalPipelineProcessing()
			release.MarkFinalPipelineProcessed()
			Expect(release.IsFinalPipelineProcessing()).To(BeFalse())
			release.MarkFinalPipelineProcessing()
			Expect(release.IsFinalPipelineProcessing()).To(BeFalse())
		})

		It("should register the start time if the final pipeline is not processing", func() {
			Expect(release.Status.FinalProcessing.StartTime).To(BeNil())
			release.MarkFinalPipelineProcessing()
			Expect(release.Status.FinalProcessing.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if the final pipeline is processing already", func() {
			Expect(release.Status.FinalProcessing.StartTime).To(BeNil())
			release.MarkFinalPipelineProcessing()
			release.Status.FinalProcessing.StartTime = &metav1.Time{}
			Expect(release.Status.FinalProcessing.StartTime.IsZero()).To(BeTrue())
			release.MarkFinalPipelineProcessing()
			Expect(release.Status.FinalProcessing.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkFinalPipelineProcessing()

			condition := meta.FindStatusCondition(release.Status.Conditions, finalProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(ProgressingReason.String()),
				"Status": Equal(metav1.ConditionFalse),
			}))
		})

	})

	When("MarkManagedCollectorsPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release managed collectors pipeline processing finished", func() {
			release.MarkManagedCollectorsPipelineProcessing()
			release.MarkManagedCollectorsPipelineProcessed()
			Expect(release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
			release.MarkManagedCollectorsPipelineProcessing()
			Expect(release.IsManagedCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should register the start time if the managed collectors pipeline is not processing", func() {
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.StartTime).To(BeNil())
			release.MarkManagedCollectorsPipelineProcessing()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if the managed collectors pipeline is processing already", func() {
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.StartTime).To(BeNil())
			release.MarkManagedCollectorsPipelineProcessing()
			release.Status.CollectorsProcessing.ManagedCollectorsProcessing.StartTime = &metav1.Time{}
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.StartTime.IsZero()).To(BeTrue())
			release.MarkManagedCollectorsPipelineProcessing()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkManagedCollectorsPipelineProcessing()

			condition := meta.FindStatusCondition(release.Status.Conditions, managedCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(ProgressingReason.String()),
				"Status": Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkManagedPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release managed pipeline processing finished", func() {
			release.MarkManagedPipelineProcessing()
			release.MarkManagedPipelineProcessed()
			Expect(release.IsManagedPipelineProcessing()).To(BeFalse())
			release.MarkManagedPipelineProcessing()
			Expect(release.IsManagedPipelineProcessing()).To(BeFalse())
		})

		It("should register the start time if the managed pipeline is not processing", func() {
			Expect(release.Status.ManagedProcessing.StartTime).To(BeNil())
			release.MarkManagedPipelineProcessing()
			Expect(release.Status.ManagedProcessing.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if the managed pipeline is processing already", func() {
			Expect(release.Status.ManagedProcessing.StartTime).To(BeNil())
			release.MarkManagedPipelineProcessing()
			release.Status.ManagedProcessing.StartTime = &metav1.Time{}
			Expect(release.Status.ManagedProcessing.StartTime.IsZero()).To(BeTrue())
			release.MarkManagedPipelineProcessing()
			Expect(release.Status.ManagedProcessing.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkManagedPipelineProcessing()

			condition := meta.FindStatusCondition(release.Status.Conditions, managedProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(ProgressingReason.String()),
				"Status": Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkTenantCollectorsPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release tenant collectors pipeline processing finished", func() {
			release.MarkTenantCollectorsPipelineProcessing()
			release.MarkTenantCollectorsPipelineProcessed()
			Expect(release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
			release.MarkTenantCollectorsPipelineProcessing()
			Expect(release.IsTenantCollectorsPipelineProcessing()).To(BeFalse())
		})

		It("should register the start time if the tenant collectors pipeline is not processing", func() {
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.StartTime).To(BeNil())
			release.MarkTenantCollectorsPipelineProcessing()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if the tenant collectors pipeline is processing already", func() {
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.StartTime).To(BeNil())
			release.MarkTenantCollectorsPipelineProcessing()
			release.Status.CollectorsProcessing.TenantCollectorsProcessing.StartTime = &metav1.Time{}
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.StartTime.IsZero()).To(BeTrue())
			release.MarkTenantCollectorsPipelineProcessing()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkTenantCollectorsPipelineProcessing()

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(ProgressingReason.String()),
				"Status": Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkTenantPipelineProcessing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release tenant pipeline processing finished", func() {
			release.MarkTenantPipelineProcessing()
			release.MarkTenantPipelineProcessed()
			Expect(release.IsTenantPipelineProcessing()).To(BeFalse())
			release.MarkTenantPipelineProcessing()
			Expect(release.IsTenantPipelineProcessing()).To(BeFalse())
		})

		It("should register the start time if the tenant pipeline is not processing", func() {
			Expect(release.Status.TenantProcessing.StartTime).To(BeNil())
			release.MarkTenantPipelineProcessing()
			Expect(release.Status.TenantProcessing.StartTime).NotTo(BeNil())
		})

		It("should not register the start time if the tenant pipeline is processing already", func() {
			Expect(release.Status.TenantProcessing.StartTime).To(BeNil())
			release.MarkTenantPipelineProcessing()
			release.Status.TenantProcessing.StartTime = &metav1.Time{}
			Expect(release.Status.TenantProcessing.StartTime.IsZero()).To(BeTrue())
			release.MarkTenantPipelineProcessing()
			Expect(release.Status.TenantProcessing.StartTime.IsZero()).To(BeTrue())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkTenantPipelineProcessing()

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(ProgressingReason.String()),
				"Status": Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkFinalPipelineProcessingFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release final pipeline processing has not started", func() {
			release.MarkFinalPipelineProcessingFailed("")
			Expect(release.Status.FinalProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release final pipeline processing finished", func() {
			release.MarkFinalPipelineProcessing()
			release.MarkFinalPipelineProcessed()
			Expect(release.Status.FinalProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.FinalProcessing.CompletionTime = &metav1.Time{}
			release.MarkFinalPipelineProcessingFailed("")
			Expect(release.Status.FinalProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkFinalPipelineProcessing()
			Expect(release.Status.FinalProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkFinalPipelineProcessingFailed("")
			Expect(release.Status.FinalProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkFinalPipelineProcessing()
			release.MarkFinalPipelineProcessingFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, finalProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})

	})

	When("MarkManagedCollectorsPipelineProcessingFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release managed collectors pipeline processing has not started", func() {
			release.MarkManagedCollectorsPipelineProcessingFailed("")
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release managed collectors pipeline processing finished", func() {
			release.MarkManagedCollectorsPipelineProcessing()
			release.MarkManagedCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime = &metav1.Time{}
			release.MarkManagedCollectorsPipelineProcessingFailed("")
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkManagedCollectorsPipelineProcessing()
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkManagedCollectorsPipelineProcessingFailed("")
			Expect(release.Status.CollectorsProcessing.ManagedCollectorsProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkManagedCollectorsPipelineProcessing()
			release.MarkManagedCollectorsPipelineProcessingFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, managedCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkManagedPipelineProcessingFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release managed pipeline processing has not started", func() {
			release.MarkManagedPipelineProcessingFailed("")
			Expect(release.Status.ManagedProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release managed pipeline processing finished", func() {
			release.MarkManagedPipelineProcessing()
			release.MarkManagedPipelineProcessed()
			Expect(release.Status.ManagedProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.ManagedProcessing.CompletionTime = &metav1.Time{}
			release.MarkManagedPipelineProcessingFailed("")
			Expect(release.Status.ManagedProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkManagedPipelineProcessing()
			Expect(release.Status.ManagedProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkManagedPipelineProcessingFailed("")
			Expect(release.Status.ManagedProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkManagedPipelineProcessing()
			release.MarkManagedPipelineProcessingFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, managedProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkTenantCollectorsPipelineProcessingFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release tenant collectors pipeline processing has not started", func() {
			release.MarkTenantCollectorsPipelineProcessingFailed("")
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release tenant collectors pipeline processing finished", func() {
			release.MarkTenantCollectorsPipelineProcessing()
			release.MarkTenantCollectorsPipelineProcessed()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime = &metav1.Time{}
			release.MarkTenantCollectorsPipelineProcessingFailed("")
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkTenantCollectorsPipelineProcessing()
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkTenantCollectorsPipelineProcessingFailed("")
			Expect(release.Status.CollectorsProcessing.TenantCollectorsProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkTenantCollectorsPipelineProcessing()
			release.MarkTenantCollectorsPipelineProcessingFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkTenantPipelineProcessingFailed method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release tenant pipeline processing has not started", func() {
			release.MarkTenantPipelineProcessingFailed("")
			Expect(release.Status.TenantProcessing.CompletionTime).To(BeNil())
		})

		It("should do nothing if the Release tenant pipeline processing finished", func() {
			release.MarkTenantPipelineProcessing()
			release.MarkTenantPipelineProcessed()
			Expect(release.Status.TenantProcessing.CompletionTime.IsZero()).To(BeFalse())
			release.Status.TenantProcessing.CompletionTime = &metav1.Time{}
			release.MarkTenantPipelineProcessingFailed("")
			Expect(release.Status.TenantProcessing.CompletionTime.IsZero()).To(BeTrue())
		})

		It("should register the completion time", func() {
			release.MarkTenantPipelineProcessing()
			Expect(release.Status.TenantProcessing.CompletionTime.IsZero()).To(BeTrue())
			release.MarkTenantPipelineProcessingFailed("")
			Expect(release.Status.TenantProcessing.CompletionTime.IsZero()).To(BeFalse())
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkTenantPipelineProcessing()
			release.MarkTenantPipelineProcessingFailed("foo")

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("foo"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})
	})

	When("MarkFinalPipelineProcessingSkipped method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release final pipeline processing finished already", func() {
			release.MarkFinalPipelineProcessing()
			release.MarkFinalPipelineProcessingFailed("error")
			release.MarkFinalPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, finalProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("error"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkFinalPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, finalProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SkippedReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkManagedCollectorsPipelineProcessingSkipped method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release managed collectors pipeline processing finished already", func() {
			release.MarkManagedCollectorsPipelineProcessing()
			release.MarkManagedCollectorsPipelineProcessingFailed("error")
			release.MarkManagedCollectorsPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, managedCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("error"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkManagedCollectorsPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, managedCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SkippedReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkManagedPipelineProcessingSkipped method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release managed pipeline processing finished already", func() {
			release.MarkManagedPipelineProcessing()
			release.MarkManagedPipelineProcessingFailed("error")
			release.MarkManagedPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, managedProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("error"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkManagedPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, managedProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SkippedReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkTenantCollectorsPipelineProcessingSkipped method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release tenant collectors pipeline processing finished already", func() {
			release.MarkTenantCollectorsPipelineProcessing()
			release.MarkTenantCollectorsPipelineProcessingFailed("error")
			release.MarkTenantCollectorsPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("error"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkTenantCollectorsPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantCollectorsProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SkippedReason.String()),
				"Status": Equal(metav1.ConditionTrue),
			}))
		})
	})

	When("MarkTenantPipelineProcessingSkipped method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should do nothing if the Release tenant pipeline processing finished already", func() {
			release.MarkTenantPipelineProcessing()
			release.MarkTenantPipelineProcessingFailed("error")
			release.MarkTenantPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Message": Equal("error"),
				"Reason":  Equal(FailedReason.String()),
				"Status":  Equal(metav1.ConditionFalse),
			}))
		})

		It("should register the condition", func() {
			Expect(release.Status.Conditions).To(HaveLen(0))
			release.MarkTenantPipelineProcessingSkipped()

			condition := meta.FindStatusCondition(release.Status.Conditions, tenantProcessedConditionType.String())
			Expect(condition).NotTo(BeNil())
			Expect(*condition).To(MatchFields(IgnoreExtras, Fields{
				"Reason": Equal(SkippedReason.String()),
				"Status": Equal(metav1.ConditionTrue),
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
			Expect(release.hasPhaseFinished(validatedConditionType)).To(BeFalse())
		})

		It("should return true when the condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.hasPhaseFinished(validatedConditionType)).To(BeTrue())
		})

		It("should return true when the condition status is True and the reason is Skipped", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionTrue, SkippedReason)
			Expect(release.hasPhaseFinished(validatedConditionType)).To(BeTrue())
		})

		It("should return false when the condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.hasPhaseFinished(validatedConditionType)).To(BeFalse())
		})

		It("should return true when the condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.hasPhaseFinished(validatedConditionType)).To(BeTrue())
		})

		It("should return false when the condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.hasPhaseFinished(validatedConditionType)).To(BeFalse())
		})
	})

	When("isPhaseProgressing method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should return false when the condition is missing", func() {
			Expect(release.isPhaseProgressing(validatedConditionType)).To(BeFalse())
		})

		It("should return false when the condition status is True", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionTrue, SucceededReason)
			Expect(release.isPhaseProgressing(validatedConditionType)).To(BeFalse())
		})

		It("should return false when the condition status is True and the reason is Skipped", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionTrue, SkippedReason)
			Expect(release.isPhaseProgressing(validatedConditionType)).To(BeFalse())
		})

		It("should return true when the condition status is False and the reason is Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionFalse, ProgressingReason)
			Expect(release.isPhaseProgressing(validatedConditionType)).To(BeTrue())
		})

		It("should return false when the condition status is False and the reason is not Progressing", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionFalse, FailedReason)
			Expect(release.isPhaseProgressing(validatedConditionType)).To(BeFalse())
		})

		It("should return false when the condition status is Unknown", func() {
			conditions.SetCondition(&release.Status.Conditions, validatedConditionType, metav1.ConditionUnknown, ProgressingReason)
			Expect(release.isPhaseProgressing(validatedConditionType)).To(BeFalse())
		})
	})

	When("SetExpirationTime method is called", func() {
		var release *Release

		BeforeEach(func() {
			release = &Release{}
		})

		It("should set the ExpirationTime", func() {
			expireDays := time.Duration(5)
			creationTime := release.CreationTimestamp
			expectedExpirationTime := &metav1.Time{Time: creationTime.Add(time.Hour * 24 * expireDays)}

			release.SetExpirationTime(expireDays)
			Expect(release.Status.ExpirationTime).To(Equal(expectedExpirationTime))
		})
	})
})
