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
	"github.com/konflux-ci/release-service/metadata"
	"github.com/konflux-ci/release-service/tekton/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("Predicates", Ordered, func() {
	When("testing ReleasePipelineRunLifecyclePredicate predicate", func() {
		var err error
		var pipelineRun *v1.PipelineRun

		BeforeAll(func() {
			pipelineRun, err = utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should ignore creating events", func() {
			contextEvent := event.CreateEvent{
				Object: pipelineRun,
			}
			Expect(ReleasePipelineRunLifecyclePredicate().Create(contextEvent)).To(BeFalse())
		})

		It("should ignore deleting events for non-release PipelineRuns", func() {
			contextEvent := event.DeleteEvent{
				Object: pipelineRun,
			}
			Expect(ReleasePipelineRunLifecyclePredicate().Delete(contextEvent)).To(BeFalse())
		})

		It("should ignore deleting events for release PipelineRuns without our finalizer", func() {
			releasePipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				Build()
			Expect(err).NotTo(HaveOccurred())

			contextEvent := event.DeleteEvent{
				Object: releasePipelineRun,
			}
			Expect(ReleasePipelineRunLifecyclePredicate().Delete(contextEvent)).To(BeFalse())
		})

		It("should reconcile deleting events for release PipelineRuns with our finalizer", func() {
			releasePipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				WithFinalizer(metadata.ReleaseFinalizer).
				Build()
			Expect(err).NotTo(HaveOccurred())

			contextEvent := event.DeleteEvent{
				Object: releasePipelineRun,
			}
			Expect(ReleasePipelineRunLifecyclePredicate().Delete(contextEvent)).To(BeTrue())
		})

		It("should ignore generic events", func() {
			contextEvent := event.GenericEvent{
				Object: pipelineRun,
			}
			Expect(ReleasePipelineRunLifecyclePredicate().Generic(contextEvent)).To(BeFalse())
		})

		It("should return true when an updated event is received for a succeeded managed PipelineRun", func() {
			var releasePipelineRun *v1.PipelineRun
			releasePipelineRun, err = utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				Build()
			Expect(err).NotTo(HaveOccurred())
			contextEvent := event.UpdateEvent{
				ObjectOld: pipelineRun,
				ObjectNew: releasePipelineRun,
			}
			releasePipelineRun.Status.MarkRunning("Predicate function tests", "Set it to Unknown")
			Expect(ReleasePipelineRunLifecyclePredicate().Update(contextEvent)).To(BeFalse())

			releasePipelineRun.Status.MarkSucceeded("Predicate function tests", "Set it to Succeeded")
			Expect(ReleasePipelineRunLifecyclePredicate().Update(contextEvent)).To(BeTrue())
		})

		It("should trigger when finalizers change on a release PipelineRun", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				WithFinalizer(metadata.ReleaseFinalizer).
				Build()
			Expect(err).NotTo(HaveOccurred())

			contextEvent := event.UpdateEvent{
				ObjectOld: oldPipelineRun,
				ObjectNew: newPipelineRun,
			}

			Expect(ReleasePipelineRunLifecyclePredicate().Update(contextEvent)).To(BeTrue())
		})

		It("should trigger when finalizers are removed from a release PipelineRun", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				WithFinalizer(metadata.ReleaseFinalizer).
				WithFinalizer("tekton.dev/finalizer").
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				WithFinalizer("tekton.dev/finalizer").
				Build()
			Expect(err).NotTo(HaveOccurred())

			contextEvent := event.UpdateEvent{
				ObjectOld: oldPipelineRun,
				ObjectNew: newPipelineRun,
			}

			Expect(ReleasePipelineRunLifecyclePredicate().Update(contextEvent)).To(BeTrue())
		})

		It("should not trigger when finalizers are unchanged", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				WithFinalizer(metadata.ReleaseFinalizer).
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType.String()}).
				WithFinalizer(metadata.ReleaseFinalizer).
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun.Status.MarkRunning("Test", "Running")

			contextEvent := event.UpdateEvent{
				ObjectOld: oldPipelineRun,
				ObjectNew: newPipelineRun,
			}

			Expect(ReleasePipelineRunLifecyclePredicate().Update(contextEvent)).To(BeFalse())
		})

		It("should not trigger when finalizers change on non-release PipelineRuns", func() {
			oldPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				Build()
			Expect(err).NotTo(HaveOccurred())

			newPipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithFinalizer("some.other/finalizer").
				Build()
			Expect(err).NotTo(HaveOccurred())

			contextEvent := event.UpdateEvent{
				ObjectOld: oldPipelineRun,
				ObjectNew: newPipelineRun,
			}

			Expect(ReleasePipelineRunLifecyclePredicate().Update(contextEvent)).To(BeFalse())
		})
	})
})
