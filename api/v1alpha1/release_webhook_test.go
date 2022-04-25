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
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Release validation webhook", func() {
	// Define utility constants for object names
	const (
		Name        = "test-release-"
		Namespace   = "default"
		Component   = "test-component"
		ReleaseLink = "test-releaselink"
	)

	Context("Update Release CR fields", func() {
		It("Should error out when updating the resource", func() {
			ctx := context.Background()

			release := &Release{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Release",
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: Name,
					Namespace:    Namespace,
				},
				Spec: ReleaseSpec{
					Component:   Component,
					ReleaseLink: ReleaseLink,
				},
			}
			Expect(k8sClient.Create(ctx, release)).Should(Succeed())

			// Look up the Release resource that was created
			releaseLookupKey := types.NamespacedName{Name: release.Name, Namespace: Namespace}
			createdRelease := &Release{}
			Eventually(func() bool {
				k8sClient.Get(ctx, releaseLookupKey, createdRelease)
				return !reflect.DeepEqual(createdRelease, &Release{})
			}, timeout, interval).Should(BeTrue())

			// Try to update the Release component
			createdRelease.Spec.Component = "another-component"
			err := k8sClient.Update(ctx, createdRelease)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("release resources cannot be updated"))

			// Delete the specified Release resource
			deleteReleaseCR(releaseLookupKey)
		})
	})

})

// deleteReleaseCR deletes the specified Release resource and verifies it was properly deleted.
func deleteReleaseCR(releaseLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &Release{}
		k8sClient.Get(context.Background(), releaseLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &Release{}
		return k8sClient.Get(context.Background(), releaseLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}
