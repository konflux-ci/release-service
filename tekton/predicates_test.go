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
	When("testing ReleasePipelineRunSucceededPredicate predicate", func() {
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
			Expect(ReleasePipelineRunSucceededPredicate().Create(contextEvent)).To(BeFalse())
		})

		It("should ignore deleting events", func() {
			contextEvent := event.DeleteEvent{
				Object: pipelineRun,
			}
			Expect(ReleasePipelineRunSucceededPredicate().Delete(contextEvent)).To(BeFalse())
		})

		It("should ignore generic events", func() {
			contextEvent := event.GenericEvent{
				Object: pipelineRun,
			}
			Expect(ReleasePipelineRunSucceededPredicate().Generic(contextEvent)).To(BeFalse())
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
			Expect(ReleasePipelineRunSucceededPredicate().Update(contextEvent)).To(BeFalse())

			releasePipelineRun.Status.MarkSucceeded("Predicate function tests", "Set it to Succeeded")
			Expect(ReleasePipelineRunSucceededPredicate().Update(contextEvent)).To(BeTrue())
		})
	})
})
