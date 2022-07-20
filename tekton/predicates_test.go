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
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Predicates", func() {

	const (
		pipelineRunPrefixName = "test-pipeline"
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
				ClusterName:  "test-cluster",
			},
			Spec: v1alpha1.ReleaseSpec{
				ApplicationSnapshot: "testsnapshot",
				ReleaseLink:         "testreleaselink",
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

	Context("when testing ReleasePipelineRunSucceededPredicate predicate", func() {
		instance := ReleasePipelineRunSucceededPredicate()

		It("should ignore creating events", func() {
			contextEvent := event.CreateEvent{
				Object: releasePipelineRun.AsPipelineRun(),
			}
			Expect(instance.Create(contextEvent)).To(BeFalse())
		})

		It("should ignore deleting events", func() {
			contextEvent := event.DeleteEvent{
				Object: releasePipelineRun.AsPipelineRun(),
			}
			Expect(instance.Delete(contextEvent)).To(BeFalse())
		})

		It("should ignore generic events", func() {
			contextEvent := event.GenericEvent{
				Object: releasePipelineRun.AsPipelineRun(),
			}
			Expect(instance.Generic(contextEvent)).To(BeFalse())
		})

		It("should return true when an updated event is received for a succeeded release PipelineRun", func() {
			releasePipelineRun.AsPipelineRun().Status.InitializeConditions()
			contextEvent := event.UpdateEvent{
				ObjectOld: releasePipelineRun.AsPipelineRun(),
				ObjectNew: releasePipelineRun.WithServiceAccount("test-service-account").
					WithReleaseLabels(release.Name, release.Namespace, release.ClusterName).AsPipelineRun(),
			}
			releasePipelineRun.Status.MarkRunning("Predicate function tests", "Set it to Unknown")
			Expect(instance.Update(contextEvent)).To(BeFalse())
			releasePipelineRun.Status.MarkSucceeded("Predicate function tests", "Set it to Succeeded")
			Expect(instance.Update(contextEvent)).To(BeTrue())
		})
	})
})
