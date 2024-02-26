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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/release-service/metadata"
	"github.com/redhat-appstudio/release-service/tekton/utils"
)

var _ = Describe("Utils", Ordered, func() {
	When("isReleasePipelineRun is called", func() {
		It("should return false when the PipelineRun is not of type 'managed'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(isReleasePipelineRun(pipelineRun)).To(BeFalse())
		})

		It("should return true when the PipelineRun is of type 'managed'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").
				WithLabels(map[string]string{metadata.PipelinesTypeLabel: metadata.ManagedPipelineType}).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(isReleasePipelineRun(pipelineRun)).To(BeTrue())
		})
	})

	When("hasPipelineSucceeded is called", func() {
		It("should return false when the PipelineRun has not succeeded", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(hasPipelineSucceeded(pipelineRun)).To(BeFalse())
		})

		It("should return true when the PipelineRun is of type 'managed'", func() {
			pipelineRun, err := utils.NewPipelineRunBuilder("pipeline-run", "default").Build()
			Expect(err).NotTo(HaveOccurred())
			pipelineRun.Status.MarkSucceeded("", "")
			Expect(hasPipelineSucceeded(pipelineRun)).To(BeTrue())
		})
	})
})
