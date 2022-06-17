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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("ReleaseLink validation webhook", func() {
	// Define utility constants for object names
	const (
		Name            = "test-releaselink-"
		Namespace       = "default"
		DisplayName     = "example-release-link"
		Application     = "test-application"
		Target          = "test-target"
		ReleaseStrategy = "test-releasestrategy"
	)

	Context("Create ReleaseLink CR without auto-release label", func() {
		It("Should add the label with value true", func() {
			ctx := context.Background()

			// ReleaseLink without autoReleaseLabel set
			releaseLink := &ReleaseLink{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ReleaseLink",
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: Name,
					Namespace:    Namespace,
				},
				Spec: ReleaseLinkSpec{
					DisplayName:     DisplayName,
					Application:     Application,
					Target:          Target,
					ReleaseStrategy: ReleaseStrategy,
				},
			}

			err := k8sClient.Create(ctx, releaseLink)
			Expect(err).Should(BeNil())

			// Look up the ReleaseLink resource that was created
			releaseLinkLookupKey := types.NamespacedName{Name: releaseLink.Name, Namespace: Namespace}
			createdReleaseLink := &ReleaseLink{}
			Eventually(func() bool {
				k8sClient.Get(ctx, releaseLinkLookupKey, createdReleaseLink)
				return !reflect.DeepEqual(createdReleaseLink, &ReleaseLink{})
			}, timeout, interval).Should(BeTrue())

			// Ensure that the label was created and set to true
			Expect(createdReleaseLink.Labels[autoReleaseLabel]).Should(Equal("true"))

			// Delete the specified ReleaseLink resource
			deleteReleaseLinkCR(releaseLinkLookupKey)
		})
	})

	Context("Create ReleaseLink CR with bad fields", func() {
		It("Should reject until all the fields are valid", func() {
			ctx := context.Background()

			// Bad ReleaseLink Target
			releaseLink := &ReleaseLink{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ReleaseLink",
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: Name,
					Namespace:    Namespace,
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "test",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName:     DisplayName,
					Application:     Application,
					Target:          "",
					ReleaseStrategy: ReleaseStrategy,
				},
			}

			err := k8sClient.Create(ctx, releaseLink)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("spec.target in body should match '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'"))

			releaseLink.Spec.Target = Namespace

			err = k8sClient.Create(ctx, releaseLink)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("field spec.target and namespace cannot have the same value"))

			// Good Target
			releaseLink.Spec.Target = Target
			err = k8sClient.Create(ctx, releaseLink)
			Expect(err.Error()).Should(ContainSubstring("%s label can only be set to true or false", autoReleaseLabel))

			// Good auto-release label
			releaseLink.Labels[autoReleaseLabel] = "true"
			err = k8sClient.Create(ctx, releaseLink)
			Expect(err).Should(BeNil())

			// Look up the ReleaseLink resource that was created
			releaseLinkLookupKey := types.NamespacedName{Name: releaseLink.Name, Namespace: Namespace}
			createdReleaseLink := &ReleaseLink{}
			Eventually(func() bool {
				k8sClient.Get(ctx, releaseLinkLookupKey, createdReleaseLink)
				return !reflect.DeepEqual(createdReleaseLink, &ReleaseLink{})
			}, timeout, interval).Should(BeTrue())

			// Delete the specified ReleaseLink resource
			deleteReleaseLinkCR(releaseLinkLookupKey)
		})
	})

	Context("Update ReleaseLink CR fields", func() {
		It("Should update target successfully when different from workspace and err out when the same", func() {
			ctx := context.Background()

			releaseLink := &ReleaseLink{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "ReleaseLink",
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: Name,
					Namespace:    Namespace,
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "false",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName:     DisplayName,
					Application:     Application,
					Target:          Target,
					ReleaseStrategy: ReleaseStrategy,
				},
			}
			Expect(k8sClient.Create(ctx, releaseLink)).Should(Succeed())

			// Look up the ReleaseLink resource that was created
			releaseLinkLookupKey := types.NamespacedName{Name: releaseLink.Name, Namespace: Namespace}
			createdReleaseLink := &ReleaseLink{}
			Eventually(func() bool {
				k8sClient.Get(ctx, releaseLinkLookupKey, createdReleaseLink)
				return !reflect.DeepEqual(createdReleaseLink, &ReleaseLink{})
			}, timeout, interval).Should(BeTrue())

			// Update the ReleaseLink target
			createdReleaseLink.Spec.Target = "another-target"
			err := k8sClient.Update(ctx, createdReleaseLink)
			Expect(err).Should(BeNil())

			// Update the ReleaseLink target to be invalid
			createdReleaseLink.Spec.Target = Namespace
			err = k8sClient.Update(ctx, createdReleaseLink)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("field spec.target and namespace cannot have the same value"))

			// Update the auto-release label to be invalid
			createdReleaseLink.Spec.Target = "another-target"
			createdReleaseLink.Labels[autoReleaseLabel] = "test"
			err = k8sClient.Update(ctx, createdReleaseLink)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("%s label can only be set to true or false", autoReleaseLabel))

			// Delete the specified ReleaseLink resource
			deleteReleaseLinkCR(releaseLinkLookupKey)
		})
	})

})

// deleteReleaseLinkCR deletes the specified ReleaseLink resource and verifies it was properly deleted.
func deleteReleaseLinkCR(releaseLinkLookupKey types.NamespacedName) {
	// Delete
	Eventually(func() error {
		f := &ReleaseLink{}
		k8sClient.Get(context.Background(), releaseLinkLookupKey, f)
		return k8sClient.Delete(context.Background(), f)
	}, timeout, interval).Should(Succeed())

	// Wait for delete to finish
	Eventually(func() error {
		f := &ReleaseLink{}
		return k8sClient.Get(context.Background(), releaseLinkLookupKey, f)
	}, timeout, interval).ShouldNot(Succeed())
}
