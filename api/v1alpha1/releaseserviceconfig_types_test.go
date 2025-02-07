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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ReleaseServiceConfig type", func() {
	When("IsPipelineOverridden method is called", func() {
		var releaseServiceConfig *ReleaseServiceConfig

		BeforeEach(func() {
			releaseServiceConfig = &ReleaseServiceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: "default",
				},
			}
		})

		It("should return false if the resource is not overridden", func() {
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeFalse())
		})

		It("should return true if the resource is overridden", func() {
			releaseServiceConfig.Spec.EmptyDirOverrides = []EmptyDirOverrides{
				{"foo", "bar", "baz"},
			}
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeTrue())
		})

		It("should return true if the resource is overridden using a regex expression in the url field", func() {
			releaseServiceConfig.Spec.EmptyDirOverrides = []EmptyDirOverrides{
				{".*", "bar", "baz"},
			}
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeTrue())
		})

		It("should return true if the resource is overridden using a regex expression in the revision field", func() {
			releaseServiceConfig.Spec.EmptyDirOverrides = []EmptyDirOverrides{
				{"foo", ".*", "baz"},
			}
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeTrue())
		})

		It("should return false if the resource is overridden using a regex expression in the pathInRepo field", func() {
			releaseServiceConfig.Spec.EmptyDirOverrides = []EmptyDirOverrides{
				{"foo", "bar", ".*"},
			}
			Expect(releaseServiceConfig.IsPipelineOverridden("foo", "bar", "baz")).To(BeFalse())
		})
	})
})
