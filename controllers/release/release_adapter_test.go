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
	"reflect"
	"strings"
	"time"

	"github.com/kcp-dev/logicalcluster/v2"
	"github.com/redhat-appstudio/release-service/kcp"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/operator-framework/operator-lib/handler"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appstudioshared "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/tekton"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Release Adapter", Ordered, func() {
	var (
		adapter *Adapter

		applicationSnapshot  *appstudioshared.ApplicationSnapshot
		release              *appstudiov1alpha1.Release
		releaseStrategy      *appstudiov1alpha1.ReleaseStrategy
		releasePlan          *appstudiov1alpha1.ReleasePlan
		releasePlanAdmission *appstudiov1alpha1.ReleasePlanAdmission
	)

	BeforeAll(func() {
		releaseStrategy = &appstudiov1alpha1.ReleaseStrategy{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-releasestrategy-",
				Namespace:    testNamespace,
			},
			Spec: appstudiov1alpha1.ReleaseStrategySpec{
				Pipeline:              "release-pipeline",
				Bundle:                "test-bundle",
				Policy:                "test-policy",
				PersistentVolumeClaim: "test-pvc",
				ServiceAccount:        "test-account",
			},
		}
		Expect(k8sClient.Create(ctx, releaseStrategy)).Should(Succeed())

		applicationSnapshot = &appstudioshared.ApplicationSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-snapshot-",
				Namespace:    "default",
			},
			Spec: appstudioshared.ApplicationSnapshotSpec{
				Application: "testapplication",
				Components:  []appstudioshared.ApplicationSnapshotComponent{},
			},
		}
		Expect(k8sClient.Create(ctx, applicationSnapshot)).Should(Succeed())

		releasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-releaseplanadmission-",
				Namespace:    testNamespace,
				Labels: map[string]string{
					appstudiov1alpha1.AutoReleaseLabel: "true",
				},
			},
			Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
				Application: "test-app",
				Origin: kcp.NamespaceReference{
					Namespace: testNamespace,
				},
				Environment:     "test-environment",
				ReleaseStrategy: releaseStrategy.GetName(),
			},
		}
		Expect(k8sClient.Create(ctx, releasePlanAdmission)).Should(Succeed())

		releasePlan = &appstudiov1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-releaseplan-",
				Namespace:    testNamespace,
				Labels: map[string]string{
					appstudiov1alpha1.AutoReleaseLabel: "true",
				},
			},
			Spec: appstudiov1alpha1.ReleasePlanSpec{
				Application: "test-app",
				Target: kcp.NamespaceReference{
					Namespace: testNamespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, releasePlan)).Should(Succeed())

	})

	BeforeEach(func() {

		release = &appstudiov1alpha1.Release{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-release-",
				Namespace:    testNamespace,
			},
			Spec: appstudiov1alpha1.ReleaseSpec{
				ApplicationSnapshot: applicationSnapshot.GetName(),
				ReleasePlan:         releasePlan.GetName(),
			},
		}
		Expect(k8sClient.Create(ctx, release)).Should(Succeed())

		Eventually(func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      release.Name,
				Namespace: testNamespace,
			}, release)
			return err
		}, time.Second*10).ShouldNot(HaveOccurred())

		adapter = NewAdapter(release, ctrl.Log, k8sClient, ctx)
		adapter.targetContext = adapter.context
		Expect(reflect.TypeOf(adapter)).To(Equal(reflect.TypeOf(&Adapter{})))
	})

	AfterEach(func() {
		err := k8sClient.Delete(ctx, release)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())

	})

	AfterAll(func() {
		err := k8sClient.Delete(ctx, applicationSnapshot)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())

		Expect(k8sClient.Delete(ctx, releaseStrategy)).Should(Succeed())

		err = k8sClient.Delete(ctx, releasePlan)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())

		err = k8sClient.Delete(ctx, releasePlanAdmission)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	It("can create a new Adapter instance", func() {
		Expect(reflect.TypeOf(NewAdapter(release, ctrl.Log, k8sClient, ctx))).To(Equal(reflect.TypeOf(&Adapter{})))
	})

	It("ensures the finalizer is added to the Release object", func() {
		result, err := adapter.EnsureFinalizerIsAdded()
		Expect(!result.CancelRequest && err == nil).To(BeTrue())
		Expect(release.ObjectMeta.GetFinalizers()).To(ContainElement(finalizerName))

		// Call should not fail if the finalizer already exists
		result, err = adapter.EnsureFinalizerIsAdded()
		Expect(!result.CancelRequest && err == nil).To(BeTrue())
	})

	It("ensures the ReleasePlanAdmission is checked to be enabled", func() {
		result, err := adapter.EnsureReleasePlanAdmissionEnabled()
		Expect(!result.RequeueRequest && !result.CancelRequest && err == nil).To(BeTrue())
	})

	It("ensures processing stops if the ReleasePlanAdmission is found to be disabled", func() {
		// Disable the ReleasePlanAdmission
		patch := client.MergeFrom(releasePlanAdmission.DeepCopy())
		releasePlanAdmission.SetLabels(map[string]string{
			appstudiov1alpha1.AutoReleaseLabel: "false",
		})
		Expect(k8sClient.Patch(ctx, releasePlanAdmission, patch)).Should(Succeed())

		Eventually(func() bool {
			result, _ := adapter.EnsureReleasePlanAdmissionEnabled()
			return !result.RequeueRequest && result.CancelRequest &&
				release.Status.Conditions[0].Reason == string(appstudiov1alpha1.ReleaseReasonTargetDisabledError)
		})

		// Reenable the ReleasePlanAdmission
		patch = client.MergeFrom(releasePlanAdmission.DeepCopy())
		releasePlanAdmission.SetLabels(map[string]string{
			appstudiov1alpha1.AutoReleaseLabel: "true",
		})
		Expect(k8sClient.Patch(ctx, releasePlanAdmission, patch)).Should(Succeed())
	})

	It("ensures a PipelineRun object exists", func() {
		Eventually(func() bool {
			result, err := adapter.EnsureReleasePipelineRunExists()
			return !result.CancelRequest && err == nil
		}, time.Second*10).Should(BeTrue())

		pipelineRun, err := adapter.getReleasePipelineRun()
		Expect(pipelineRun != nil && err == nil).To(BeTrue())
		Expect(k8sClient.Delete(ctx, pipelineRun)).Should(Succeed())
	})

	It("ensures the Release Pipeline status is tracked", func() {
		result, err := adapter.EnsureReleasePipelineRunExists()
		Expect(!result.CancelRequest && err == nil).Should(BeTrue())

		pipelineRun, err := adapter.getReleasePipelineRun()
		Expect(pipelineRun != nil && err == nil).Should(BeTrue())

		result, err = adapter.EnsureReleasePipelineStatusIsTracked()
		Expect(!result.CancelRequest && err == nil).To(BeTrue())

		// It should return results.ContinueProcessing() in case the release is done
		release.MarkSucceeded()
		result, err = adapter.EnsureReleasePipelineStatusIsTracked()
		Expect(!result.CancelRequest && err == nil).To(BeTrue())

		// The Release is running but has no pipelineRun matching the release
		release.MarkRunning()

		// avoiding "(SA5011)"
		if pipelineRun != nil {
			pipelineRun.ObjectMeta.Labels = make(map[string]string)
		}
		Expect(k8sClient.Update(ctx, pipelineRun)).Should(Succeed())

		release.Status.Conditions = []metav1.Condition{}
		result, err = adapter.EnsureReleasePipelineStatusIsTracked()
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool {
			pipelineRun, err := adapter.getReleasePipelineRun()
			return pipelineRun == nil && err == nil

		}, time.Second*10).Should(BeTrue())
		Expect(k8sClient.Delete(ctx, pipelineRun)).Should(Succeed())
	})

	It("ensures the target context is set when a workspace is targeted by releaseplan", func() {
		// Set the targetContext to nil to check if the EnsureTargetContext sets its value
		adapter.targetContext = nil

		// Add a target workspace to the ReleasePlan
		patch := client.MergeFrom(releasePlan.DeepCopy())
		releasePlan.Spec.Target.Workspace = "test-workspace"
		Expect(k8sClient.Patch(ctx, releasePlan, patch)).Should(Succeed())

		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      releasePlan.Name,
				Namespace: releasePlan.Namespace,
			}, releasePlan)
			return err == nil && releasePlan.Spec.Target.Workspace == "test-workspace"
		}, time.Second*10).Should(BeTrue())

		result, err := adapter.EnsureTargetContextIsSet()
		clusterName, _ := logicalcluster.ClusterFromContext(adapter.targetContext)
		Expect(!result.RequeueRequest).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())
		Expect(clusterName.String()).To(Equal("test-workspace"))

		// Restore original targetContext value
		adapter.targetContext = adapter.context
	})

	It("ensures the target context is set to default context when releaseplan does not contain a target workspace", func() {
		result, err := adapter.EnsureTargetContextIsSet()
		Expect(!result.RequeueRequest && err == nil).To(BeTrue())
		Expect(adapter.targetContext).To(Equal(adapter.context))
	})

	It("ensures finalizers are called when a release is deleted", func() {
		// Call to EnsureFinalizersAreCalled should succeed if the Release was not set to be deleted
		result, err := adapter.EnsureFinalizersAreCalled()
		Expect(!result.CancelRequest && !result.RequeueRequest).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())

		// Add finalizer
		result, err = adapter.EnsureFinalizerIsAdded()
		Expect(result.CancelRequest).To(BeFalse())
		Expect(err).NotTo(HaveOccurred())

		// Create a ReleasePipelineRun to ensure it gets finalized
		result, err = adapter.EnsureReleasePipelineRunExists()
		Expect(result.CancelRequest).To(BeFalse())
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Delete(ctx, release)).NotTo(HaveOccurred())
		Eventually(func() bool {
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      release.Name,
				Namespace: release.Namespace,
			}, release)
			return err == nil && release.GetDeletionTimestamp() != nil
		}, time.Second*10).Should(BeTrue())

		result, err = adapter.EnsureFinalizersAreCalled()
		Expect(result.RequeueRequest).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{
				Name:      release.Name,
				Namespace: release.Namespace,
			}, release)
		}, time.Second*10).Should(HaveOccurred())

		Eventually(func() bool {
			pipelineRun, _ := adapter.getReleasePipelineRun()
			return pipelineRun == nil
		}, time.Second*10).Should(BeTrue())
	})

	It("can finalize a Release", func() {
		// Release should be properly finalized if there is no ReleasePipelineRun
		Expect(adapter.finalizeRelease()).To(Succeed())

		// It should finalize properly as well when there's a ReleasePipelineRun
		result, err := adapter.EnsureReleasePipelineRunExists()
		Expect(result.CancelRequest).To(BeFalse())
		Expect(err).NotTo(HaveOccurred())
		Expect(adapter.finalizeRelease()).To(Succeed())
		Eventually(func() bool {
			pipelineRun, _ := adapter.getReleasePipelineRun()
			return pipelineRun == nil
		}, time.Second*10).Should(BeTrue())
	})

	It("can create a ReleasePipelineRun", func() {
		applicationSnapshot.TypeMeta.Kind = "applicationSnapshot"
		Expect(k8sClient.Update(ctx, applicationSnapshot)).Should(Succeed())

		pipelineRun, err := adapter.createReleasePipelineRun(
			releaseStrategy,
			applicationSnapshot)
		Expect(err).ShouldNot(HaveOccurred())

		// AsPipelineRun()
		Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1beta1.PipelineRun{})))

		// WithOwner()
		Expect(pipelineRun.GetAnnotations()[handler.NamespacedNameAnnotation]).To(ContainSubstring(release.Name))
		Expect(pipelineRun.GetAnnotations()[handler.TypeAnnotation]).To(ContainSubstring("Release"))

		// WithReleaseLabels()
		Expect(pipelineRun.GetLabels()[tekton.PipelinesTypeLabel]).To(Equal("release"))
		Expect(pipelineRun.GetLabels()[tekton.ReleaseNameLabel]).To(Equal(release.Name))
		Expect(pipelineRun.GetLabels()[tekton.ReleaseNamespaceLabel]).To(Equal(testNamespace))
		Expect(pipelineRun.GetLabels()[tekton.ReleaseWorkspaceLabel]).To(Equal(""))

		// WithReleaseStrategy()
		Expect(pipelineRun.Spec.PipelineRef.Name).To(Equal(releaseStrategy.Spec.Pipeline))
		Expect(pipelineRun.Spec.PipelineRef.Bundle).To(Equal(releaseStrategy.Spec.Bundle))
		Expect(pipelineRun.Spec.Params[0].Name).To(Equal("policy"))
		Expect(pipelineRun.Spec.Params[0].Value.StringVal).Should(ContainSubstring(releaseStrategy.Spec.Policy))
		Expect(pipelineRun.Spec.ServiceAccountName).Should(ContainSubstring("test-account"))

		// WithApplicationSnapshot()
		Expect(pipelineRun.Spec.Params[1].Name).To(Equal("applicationSnapshot"))
		Expect(pipelineRun.Spec.Params[1].Value.StringVal).To(ContainSubstring(applicationSnapshot.Spec.Application))

		Expect(k8sClient.Delete(ctx, pipelineRun)).Should(Succeed())
	})

	It("can return the pipelineRun from the release", func() {
		result, err := adapter.EnsureReleasePipelineRunExists()
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result.CancelRequest).To(BeFalse())

		pipelineRun, err := adapter.getReleasePipelineRun()
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(pipelineRun)).To(Equal(reflect.TypeOf(&tektonv1beta1.PipelineRun{})))
	})

	It("can return the releaseStrategy referenced in a given ReleasePlanAdmission", func() {
		strategy, err := adapter.getReleaseStrategy(releasePlanAdmission)
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(strategy)).To(Equal(reflect.TypeOf(&appstudiov1alpha1.ReleaseStrategy{})))
	})

	It("can return the ReleasePlanAdmission targeted from a given ReleasePlan", func() {
		link, _ := adapter.getActiveReleasePlanAdmission()
		Expect(reflect.TypeOf(link)).To(Equal(reflect.TypeOf(&appstudiov1alpha1.ReleasePlanAdmission{})))
		Expect(link.Spec.Application).To(Equal("test-app"))
		Expect(link.Spec.ReleaseStrategy).To(Equal(releaseStrategy.Name))
		Expect(link.Namespace).Should(Equal(releasePlan.Spec.Target.Namespace))

		// It should return nil if the ReleasePlanAdmission has auto-release set to false
		patch := client.MergeFrom(link.DeepCopy())
		link.SetLabels(map[string]string{
			appstudiov1alpha1.AutoReleaseLabel: "false",
		})
		Expect(k8sClient.Patch(ctx, link, patch)).Should(Succeed())
		Eventually(func() bool {
			activeReleasePlanAdmission, err := adapter.getActiveReleasePlanAdmission()
			return activeReleasePlanAdmission == nil && err != nil &&
				strings.Contains(err.Error(), "with auto-release label set to false")
		}, time.Second*10).Should(BeTrue())
	})

	It("fails to find the target ReleasePlanAdmission when target namespace does not match", func() {
		patch := client.MergeFrom(releasePlan.DeepCopy())
		releasePlan.Spec.Target.Namespace = "foo"
		Expect(k8sClient.Patch(ctx, releasePlan, patch)).Should(Succeed())
		Eventually(func() bool {
			activeReleasePlanAdmission, err := adapter.getActiveReleasePlanAdmission()
			return activeReleasePlanAdmission == nil && err != nil &&
				strings.Contains(err.Error(), "no ReleasePlanAdmission found in the target")
		}, time.Second*10).Should(BeTrue())
	})

	It("can mark the status for a given Release according to the pipelineRun status", func() {
		pipelineRun := tekton.NewReleasePipelineRun("test-pipeline", testNamespace).AsPipelineRun()
		pipelineRun.Name = "foo" // PipelineRun needs a name before registering the Release status
		err := adapter.registerReleaseStatusData(pipelineRun, releaseStrategy)
		Expect(err).ToNot(HaveOccurred())
		Expect(adapter.release.Status.StartTime).ToNot(BeNil())
		pipelineRun.Status.MarkSucceeded("succeeded", "set it to succeeded")
		Expect(pipelineRun.IsDone()).To(BeTrue())

		err = adapter.registerReleasePipelineRunStatus(pipelineRun)
		Expect(err).ToNot(HaveOccurred())
		Expect(release.HasSucceeded()).To(BeTrue())

		// Double check that we still have a valid StartTime
		Expect(adapter.release.Status.StartTime).ToNot(BeNil())

		// Clear up previous condition so isDone returns false
		release.Status.Conditions = []metav1.Condition{}
		pipelineRun.Status.MarkFailed("failed", "set it to failed")
		err = adapter.registerReleasePipelineRunStatus(pipelineRun)
		Expect(err).NotTo(HaveOccurred())
		Expect(release.HasSucceeded()).To(BeFalse())
	})

	It("can add the release information to its status and mark it as Running", func() {
		// The call should succeed if releaseStrategy or pipelineRun are nil
		Expect(adapter.registerReleaseStatusData(nil, nil)).To(Succeed())

		pipelineRun := tekton.NewReleasePipelineRun("test-pipeline", testNamespace).AsPipelineRun()
		pipelineRun.Name = "new-name"
		err := adapter.registerReleaseStatusData(pipelineRun, releaseStrategy)
		Expect(err).NotTo(HaveOccurred())
		Expect(release.Status.ReleasePipelineRun).To(Equal(testNamespace + "/" + pipelineRun.Name))
		Expect(release.Status.Conditions[0].Status == metav1.ConditionUnknown)
		Expect(release.Status.Conditions[0].Reason == string(appstudiov1alpha1.ReleaseReasonRunning))
	})

	It("can update the Status of a release", func() {
		patch := client.MergeFrom(adapter.release.DeepCopy())
		adapter.release.Status.ReleasePipelineRun = "foo/test"
		Expect(k8sClient.Status().Patch(ctx, release, patch)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      release.Name,
				Namespace: testNamespace,
			}, release)
			return err == nil && release.Status.ReleasePipelineRun == "foo/test"
		}, time.Second*10).Should(BeTrue())
	})

	It("can get an existing ApplicationSnapshot", func() {
		snapshot, err := adapter.getApplicationSnapshot()
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(snapshot)).To(Equal(reflect.TypeOf(&appstudioshared.ApplicationSnapshot{})))

		Expect(k8sClient.Delete(ctx, snapshot)).Should(Succeed())
		Eventually(func() bool {
			_, err = adapter.getApplicationSnapshot()
			return errors.IsNotFound(err)
		}, time.Second*10).Should(BeTrue())
	})

	It("can return the ReleasePlan from the release", func() {
		link, err := adapter.getReleasePlan()
		Expect(err).Should(Succeed())
		Expect(reflect.TypeOf(link)).To(Equal(reflect.TypeOf(&appstudiov1alpha1.ReleasePlan{})))
	})

	It("ensure an error is returned when setting the target context with a missing releaseplan", func() {
		// Delete the ReleasePlan
		Expect(k8sClient.Delete(ctx, releasePlan)).Should(Succeed())

		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      releasePlan.Name,
				Namespace: releasePlan.Namespace,
			}, releasePlan)
			return err != nil && errors.IsNotFound(err)
		}, time.Second*10).Should(BeTrue())

		// Set the targetContext to nil to check if the EnsureTargetContext sets its value
		adapter.targetContext = nil

		result, err := adapter.EnsureTargetContextIsSet()
		Expect(!result.RequeueRequest && result.CancelRequest && err == nil).To(BeTrue())

		// Restore original targetContext value
		adapter.targetContext = adapter.context
	})
})
