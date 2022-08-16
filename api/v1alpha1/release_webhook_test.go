//
// Copyright 2022 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Release validation webhook", func() {
	var release *Release

	BeforeEach(func() {
		release = &Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-release-",
				Namespace:    "default",
			},
			Spec: ReleaseSpec{
				ApplicationSnapshot: "test-snapshot",
				ReleasePlan:         "test-releaseplan",
			},
		}
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, release)
	})

	Context("Update Release CR fields", func() {
		It("Should error out when updating the resource", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, release)).Should(Succeed())

			// Try to update the Release application snapshot
			release.Spec.ApplicationSnapshot = "another-snapshot"

			err := k8sClient.Update(ctx, release)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("release resources spec cannot be updated"))
		})

		It("Should not error out when updating the resource metadata", func() {
			ctx := context.Background()

			Expect(k8sClient.Create(ctx, release)).Should(Succeed())

			// Try to update the Release annotations
			release.ObjectMeta.Annotations = map[string]string{
				"foo": "bar",
			}

			Expect(k8sClient.Update(ctx, release)).ShouldNot(HaveOccurred())
		})
	})
})
