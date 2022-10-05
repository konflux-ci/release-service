/*
Copyright 2022 Red Hat Inc.

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

package tekton

import (
	"context"
	"github.com/tektoncd/pipeline/pkg/clock"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kcp-dev/logicalcluster/v2"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Utils", func() {

	const (
		pipelineRunPrefixName = "test-pipeline"
		applicationName       = "test-application"
		apiVersion            = "appstudio.redhat.com/v1alpha1"
		namespace             = "default"
	)

	var release *v1alpha1.Release
	var releasePipelineRun *ReleasePipelineRun

	BeforeEach(func() {

		release = &v1alpha1.Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiVersion,
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "testrelease-",
				Namespace:    namespace,
				Annotations: map[string]string{
					logicalcluster.AnnotationKey: "test-cluster",
				},
			},
			Spec: v1alpha1.ReleaseSpec{
				ApplicationSnapshot: "testsnapshot",
				ReleasePlan:         "testreleaseplan",
			},
		}
		ctx := context.Background()

		// The code below sets the ownership for the Release Object
		kind := reflect.TypeOf(v1alpha1.Release{}).Name()
		gvk := v1alpha1.GroupVersion.WithKind(kind)
		controllerRef := metav1.NewControllerRef(release, gvk)

		// Creating a release
		Expect(k8sClient.Create(ctx, release)).Should(Succeed())
		release.SetOwnerReferences([]metav1.OwnerReference{*controllerRef})

		// Need to set the Kind and APIVersion as it loses it due to:
		// https://github.com/kubernetes-sigs/controller-runtime/issues/1870
		release.TypeMeta.APIVersion = apiVersion
		release.TypeMeta.Kind = "Release"

		// Creates the PipelineRun Object
		releasePipelineRun = NewReleasePipelineRun(pipelineRunPrefixName, namespace)
		Expect(k8sClient.Create(ctx, releasePipelineRun.AsPipelineRun())).Should(Succeed())
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, release)
		_ = k8sClient.Delete(ctx, releasePipelineRun.AsPipelineRun())
	})

	Context("when using utility functions on PipelineRun objects", func() {
		It("is a PipelineRun object and contains the required labels that identifies it as one", func() {
			Expect(isReleasePipelineRun(releasePipelineRun.
				WithReleaseAndApplicationLabels(release.Name, release.Namespace, release.GetAnnotations()[logicalcluster.AnnotationKey], applicationName).
				AsPipelineRun())).To(Equal(true))
		})

		It("returns true when ReleasePipelineRun.Status is `Succeeded` or false otherwise", func() {
			releasePipelineRun.AsPipelineRun().Status.InitializeConditions(clock.RealClock{})
			// MarkRunning sets Status to Unknown
			releasePipelineRun.Status.MarkRunning("PipelineRun Tests", "sets it to Unknown")
			Expect(hasPipelineSucceeded(releasePipelineRun.AsPipelineRun())).Should(BeFalse())
			releasePipelineRun.Status.MarkSucceeded("PipelineRun Tests", "sets it to Succeeded")
			Expect(hasPipelineSucceeded(releasePipelineRun.AsPipelineRun())).Should(BeTrue())
		})
	})
})
