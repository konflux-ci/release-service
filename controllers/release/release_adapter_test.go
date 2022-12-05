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

package release

import (
	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/loader"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var (
	application                *applicationapiv1alpha1.Application
	component                  *applicationapiv1alpha1.Component
	enterpriseContractPolicy   *ecapiv1alpha1.EnterpriseContractPolicy
	environment                *applicationapiv1alpha1.Environment
	releasePlan                *v1alpha1.ReleasePlan
	releasePlanAdmission       *v1alpha1.ReleasePlanAdmission
	releaseStrategy            *v1alpha1.ReleaseStrategy
	snapshot                   *applicationapiv1alpha1.Snapshot
	snapshotEnvironmentBinding *applicationapiv1alpha1.SnapshotEnvironmentBinding
)

var _ = Describe("Release Adapter", Ordered, func() {
	BeforeAll(func() {
		createResources()
	})

	Context("When EnsureFinalizersAreCalled is called", func() {
		var adapter *Adapter

		BeforeEach(func() {
			adapter = createAdapter()
			Expect(adapter.client.Create(adapter.ctx, adapter.release)).To(Succeed())
			Eventually(func() bool {
				release, err := loader.GetRelease(adapter.release.Name, adapter.release.Namespace, adapter.client, adapter.ctx)
				return release != nil && err == nil
			}).Should(BeTrue())
		})

		It("should do nothing if the release is not set to be deleted", func() {
			result, err := adapter.EnsureFinalizersAreCalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(!result.CancelRequest && !result.RequeueRequest).To(BeTrue())
		})

		It("should finalize the release if it's set to be deleted and it has a finalizer", func() {
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				adapter.release, err = loader.GetRelease(adapter.release.Name, adapter.release.Namespace, adapter.client, adapter.ctx)
				return adapter.release != nil && err == nil && len(adapter.release.Finalizers) > 0
			}).Should(BeTrue())

			result, err = adapter.EnsureReleasePipelineRunExists()
			Expect(result.CancelRequest).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				pipelineRun, err := loader.GetReleasePipelineRun(adapter.release, adapter.client, adapter.ctx)
				return pipelineRun != nil && err == nil
			}).Should(BeTrue())

			Expect(adapter.client.Delete(adapter.ctx, adapter.release)).To(Succeed())
			Eventually(func() bool {
				adapter.release, err = loader.GetRelease(adapter.release.Name, adapter.release.Namespace, adapter.client, adapter.ctx)
				return adapter.release != nil && err == nil && adapter.release.DeletionTimestamp != nil
			}, time.Second*10).Should(BeTrue())

			result, err = adapter.EnsureFinalizersAreCalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(!result.CancelRequest && result.RequeueRequest).To(BeTrue())

			Eventually(func() bool {
				pipelineRun, err := loader.GetReleasePipelineRun(adapter.release, adapter.client, adapter.ctx)
				return pipelineRun == nil && err == nil
			}, time.Second*10).Should(BeTrue())

			Eventually(func() bool {
				_, err = loader.GetRelease(adapter.release.Name, adapter.release.Namespace, adapter.client, adapter.ctx)
				return err != nil && errors.IsNotFound(err)
			}, time.Second*10).Should(BeTrue())
		})

		AfterEach(func() {
			_ = adapter.client.Delete(adapter.ctx, adapter.release)
		})
	})

	Context("When EnsureFinalizerIsAdded is called", func() {
		var adapter *Adapter

		BeforeEach(func() {
			adapter = createAdapter()
			Expect(adapter.client.Create(adapter.ctx, adapter.release)).To(Succeed())
			Eventually(func() bool {
				release, err := loader.GetRelease(adapter.release.Name, adapter.release.Namespace, adapter.client, adapter.ctx)
				return release != nil && err == nil
			}).Should(BeTrue())
		})

		It("should a finalizer is added to the release", func() {
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.CancelRequest && err == nil).To(BeTrue())
			Expect(adapter.release.Finalizers).To(ContainElement(finalizerName))
		})

		It("shouldn't fail if the release already has the finalizer added", func() {
			result, err := adapter.EnsureFinalizerIsAdded()
			Expect(!result.CancelRequest && err == nil).To(BeTrue())
			Expect(adapter.release.Finalizers).To(ContainElement(finalizerName))

			result, err = adapter.EnsureFinalizerIsAdded()
			Expect(!result.CancelRequest && err == nil).To(BeTrue())
		})

		AfterEach(func() {
			_ = adapter.client.Delete(adapter.ctx, adapter.release)
		})
	})

	Context("When EnsureFinalizerIsAdded is called", func() {
		var adapter *Adapter

		BeforeEach(func() {
			adapter = createAdapter()
			Expect(adapter.client.Create(adapter.ctx, adapter.release)).To(Succeed())
			Eventually(func() bool {
				release, err := loader.GetRelease(adapter.release.Name, adapter.release.Namespace, adapter.client, adapter.ctx)
				return release != nil && err == nil
			}).Should(BeTrue())
		})

		It("ensure there is an enabled release plan admission", func() {
			result, err := adapter.EnsureReleasePlanAdmissionEnabled()
			Expect(err).NotTo(HaveOccurred())
			Expect(!result.RequeueRequest && !result.CancelRequest).To(BeTrue())
		})

		It("ensures processing stops if the release plan admission is found to be disabled", func() {
			originalLabels := releasePlanAdmission.Labels

			// Disable the release plan admission
			patch := client.MergeFrom(releasePlanAdmission.DeepCopy())
			releasePlanAdmission.Labels[v1alpha1.AutoReleaseLabel] = "false"
			Expect(k8sClient.Patch(ctx, releasePlanAdmission, patch)).Should(Succeed())

			Eventually(func() bool {
				result, err := adapter.EnsureReleasePlanAdmissionEnabled()
				Expect(err).NotTo(HaveOccurred())
				return !result.RequeueRequest && result.CancelRequest
			}).Should(BeTrue())
			Expect(adapter.release.Status.Conditions).To(HaveLen(1))
			Expect(adapter.release.Status.Conditions[0].Reason).To(Equal(string(v1alpha1.ReleaseReasonTargetDisabledError)))

			// Restore original labels
			patch = client.MergeFrom(releasePlanAdmission.DeepCopy())
			releasePlanAdmission.Labels = originalLabels
			Expect(k8sClient.Patch(ctx, releasePlanAdmission, patch)).Should(Succeed())
		})

		AfterEach(func() {
			_ = adapter.client.Delete(adapter.ctx, adapter.release)
		})
	})
})

func createAdapter() *Adapter {
	release := &v1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "release-",
			Namespace:    "default",
		},
		Spec: v1alpha1.ReleaseSpec{
			Snapshot:    snapshot.Name,
			ReleasePlan: releasePlan.Name,
		},
	}

	return NewAdapter(release, ctrl.Log, k8sClient, ctx)
}

// createResources creates all the required resources for the tests in this file.
func createResources() {
	application = &applicationapiv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "application",
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.ApplicationSpec{
			DisplayName: "application",
		},
	}
	Expect(k8sClient.Create(ctx, application)).To(Succeed())

	component = &applicationapiv1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "component",
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.ComponentSpec{
			Application:   application.Name,
			ComponentName: "component",
		},
	}
	Expect(k8sClient.Create(ctx, component)).Should(Succeed())

	enterpriseContractPolicy = &ecapiv1alpha1.EnterpriseContractPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "enterprise-contract-policy",
			Namespace: "default",
		},
		Spec: ecapiv1alpha1.EnterpriseContractPolicySpec{
			Sources: []string{"foo"},
		},
	}
	Expect(k8sClient.Create(ctx, enterpriseContractPolicy)).Should(Succeed())

	environment = &applicationapiv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "environment",
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.EnvironmentSpec{
			DeploymentStrategy: applicationapiv1alpha1.DeploymentStrategy_Manual,
			DisplayName:        "production",
			Type:               applicationapiv1alpha1.EnvironmentType_POC,
			Configuration: applicationapiv1alpha1.EnvironmentConfiguration{
				Env: []applicationapiv1alpha1.EnvVarPair{},
			},
		},
	}
	Expect(k8sClient.Create(ctx, environment)).Should(Succeed())

	releasePlan = &v1alpha1.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release-plan",
			Namespace: "default",
		},
		Spec: v1alpha1.ReleasePlanSpec{
			Application: application.Name,
			Target:      "default",
		},
	}
	Expect(k8sClient.Create(ctx, releasePlan)).To(Succeed())

	releaseStrategy = &v1alpha1.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release-strategy",
			Namespace: "default",
		},
		Spec: v1alpha1.ReleaseStrategySpec{
			Pipeline: "release-pipeline",
			Policy:   enterpriseContractPolicy.Name,
		},
	}
	Expect(k8sClient.Create(ctx, releaseStrategy)).Should(Succeed())

	releasePlanAdmission = &v1alpha1.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release-plan-admission",
			Namespace: "default",
			Labels: map[string]string{
				v1alpha1.AutoReleaseLabel: "true",
			},
		},
		Spec: v1alpha1.ReleasePlanAdmissionSpec{
			Application:     application.Name,
			Origin:          "default",
			Environment:     environment.Name,
			ReleaseStrategy: releaseStrategy.Name,
		},
	}
	Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())

	snapshot = &applicationapiv1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snapshot",
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.SnapshotSpec{
			Application: application.Name,
		},
	}
	Expect(k8sClient.Create(ctx, snapshot)).To(Succeed())

	snapshotEnvironmentBinding = &applicationapiv1alpha1.SnapshotEnvironmentBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "snapshot-environment-binding",
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.SnapshotEnvironmentBindingSpec{
			Application: application.Name,
			Environment: environment.Name,
			Snapshot:    snapshot.Name,
			Components:  []applicationapiv1alpha1.BindingComponent{},
		},
	}
	Expect(k8sClient.Create(ctx, snapshotEnvironmentBinding)).To(Succeed())
}
