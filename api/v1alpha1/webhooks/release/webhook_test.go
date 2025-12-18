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

package release

import (
	"context"

	toolkit "github.com/konflux-ci/operator-toolkit/loader"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/loader"
	"github.com/konflux-ci/release-service/metadata"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Release validation webhook", func() {
	var (
		createResources func()

		release     *v1alpha1.Release
		releasePlan *v1alpha1.ReleasePlan
	)

	When("Default method is called", func() {
		var mockedWebhook *Webhook

		BeforeEach(func() {
			createResources()

			mockedWebhook = &Webhook{
				client: k8sClient,
				loader: loader.NewMockLoader(),
			}
		})

		It("should set GracePeriodDays to ReleasePlan's value and return nil", func() {
			mockedCtx := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
			})

			Expect(mockedWebhook.Default(mockedCtx, release)).To(BeNil())
			Expect(release.Spec.GracePeriodDays).To(Equal(releasePlan.Spec.ReleaseGracePeriodDays))
		})

		It("should return nil and keep the default value of a go `int` for GracePeriodDays when the specified ReleasePlan does not exist", func() {
			mockedCtx := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Err:        errors.NewNotFound(schema.GroupResource{}, ""),
				},
			})

			Expect(mockedWebhook.Default(mockedCtx, release)).To(BeNil())
			Expect(release.Spec.GracePeriodDays).To(Equal(0))
		})

		It("should set snapshot and releasePlan labels from spec fields", func() {
			mockedCtx := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
			})

			Expect(mockedWebhook.Default(mockedCtx, release)).To(BeNil())
			Expect(release.Labels).NotTo(BeNil())
			Expect(release.Labels[metadata.SnapshotLabel]).To(Equal(release.Spec.Snapshot))
			Expect(release.Labels[metadata.ReleasePlanLabel]).To(Equal(release.Spec.ReleasePlan))
		})

		It("should initialize labels map if nil and set labels", func() {
			release.Labels = nil
			mockedCtx := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: loader.ReleasePlanContextKey,
					Resource:   releasePlan,
				},
			})

			Expect(mockedWebhook.Default(mockedCtx, release)).To(BeNil())
			Expect(release.Labels).NotTo(BeNil())
			Expect(release.Labels[metadata.SnapshotLabel]).To(Equal(release.Spec.Snapshot))
			Expect(release.Labels[metadata.ReleasePlanLabel]).To(Equal(release.Spec.ReleasePlan))
		})
	})

	When("When ValidateUpdate is called", func() {
		It("should error out when updating the resource", func() {
			updatedRelease := release.DeepCopy()
			updatedRelease.Spec.Snapshot = "another-snapshot"

			_, err := webhook.ValidateUpdate(ctx, release, updatedRelease)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("release resources spec cannot be updated"))
		})

		It("should not error out when updating the resource metadata", func() {
			ctx := context.Background()

			updatedRelease := release.DeepCopy()
			updatedRelease.ObjectMeta.Annotations = map[string]string{
				"foo": "bar",
			}

			_, err := webhook.ValidateUpdate(ctx, release, updatedRelease)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should error out when updating the snapshot label", func() {
			updatedRelease := release.DeepCopy()
			updatedRelease.Labels = map[string]string{
				metadata.SnapshotLabel:    "different-snapshot",
				metadata.ReleasePlanLabel: release.Spec.ReleasePlan,
			}

			_, err := webhook.ValidateUpdate(ctx, release, updatedRelease)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("release snapshot label cannot be updated"))
		})

		It("should error out when updating the releasePlan label", func() {
			updatedRelease := release.DeepCopy()
			updatedRelease.Labels = map[string]string{
				metadata.SnapshotLabel:    release.Spec.Snapshot,
				metadata.ReleasePlanLabel: "different-releaseplan",
			}

			_, err := webhook.ValidateUpdate(ctx, release, updatedRelease)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("release releasePlan label cannot be updated"))
		})

		It("should not error out when updating other labels", func() {
			updatedRelease := release.DeepCopy()
			updatedRelease.Labels = map[string]string{
				metadata.SnapshotLabel:    release.Spec.Snapshot,
				metadata.ReleasePlanLabel: release.Spec.ReleasePlan,
				"custom-label":            "custom-value",
			}

			_, err := webhook.ValidateUpdate(ctx, release, updatedRelease)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("ValidateDelete method is called", func() {
		It("should return nil", func() {
			_, err := webhook.ValidateDelete(ctx, &v1alpha1.Release{})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("ValidateCreate method is called", func() {
		It("should return an error when release name is longer than 63 characters", func() {
			release := &v1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "this-is-a-very-long-release-name-that-exceeds-the-maximum-allowed-length-of-sixty-three-characters",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleaseSpec{
					Snapshot:    "test-snapshot",
					ReleasePlan: "test-releaseplan",
				},
			}
			warnings, err := webhook.ValidateCreate(context.TODO(), release)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("release name must be no more than 63 characters"))
			Expect(warnings).To(BeEmpty())
		})

		It("should return an error when snapshot name is longer than 63 characters", func() {
			release := &v1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-release",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleaseSpec{
					Snapshot:    "this-is-a-very-long-snapshot-name-that-exceeds-sixty-three-chars",
					ReleasePlan: "test-releaseplan",
				},
			}
			warnings, err := webhook.ValidateCreate(context.TODO(), release)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("snapshot name must be no more than 63 characters"))
			Expect(warnings).To(BeEmpty())
		})

		It("should return an error when releasePlan name is longer than 63 characters", func() {
			release := &v1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-release",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleaseSpec{
					Snapshot:    "test-snapshot",
					ReleasePlan: "this-is-a-very-long-releaseplan-name-that-exceeds-sixty-three-characters",
				},
			}
			warnings, err := webhook.ValidateCreate(context.TODO(), release)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("releasePlan name must be no more than 63 characters"))
			Expect(warnings).To(BeEmpty())
		})

		It("should not return an error when both release name and snapshot name are within 63 characters", func() {
			release := &v1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-release",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleaseSpec{
					Snapshot:    "test-snapshot",
					ReleasePlan: "test-releaseplan",
				},
			}
			warnings, err := webhook.ValidateCreate(context.TODO(), release)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})

	createResources = func() {
		release = &v1alpha1.Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-release-",
				Namespace:    "default",
				Labels: map[string]string{
					metadata.SnapshotLabel:    "test-snapshot",
					metadata.ReleasePlanLabel: "test-releaseplan",
				},
			},
			Spec: v1alpha1.ReleaseSpec{
				Snapshot:    "test-snapshot",
				ReleasePlan: "test-releaseplan",
			},
		}

		releasePlan = &v1alpha1.ReleasePlan{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "ReleasePlan",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-releaseplan",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleasePlanSpec{
				Application:            "test-application",
				Target:                 "default",
				ReleaseGracePeriodDays: 7,
			},
		}
	}
})
