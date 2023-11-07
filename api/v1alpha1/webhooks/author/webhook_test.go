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

package author

import (
	"encoding/json"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/release-service/metadata"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Author webhook", Ordered, func() {
	var admissionRequest admission.Request
	var err error

	BeforeAll(func() {
		admissionRequest.UserInfo.Username = "admin"
	})

	Describe("A Release request is made", func() {
		var release *v1alpha1.Release

		BeforeEach(func() {
			admissionRequest.Kind.Kind = "Release"

			release = &v1alpha1.Release{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Release",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-release",
					Namespace: "default",
				},
				Spec: v1alpha1.ReleaseSpec{
					Snapshot:    "test-snapshot",
					ReleasePlan: "test-releaseplan",
				},
			}
		})

		When("a Release is created", func() {
			BeforeAll(func() {
				admissionRequest.AdmissionRequest.Operation = admissionv1.Create
			})

			It("should add admin as the value for the author label", func() {
				admissionRequest.Object.Raw, err = json.Marshal(release)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(len(rsp.Patches)).To(Equal(1))
				patch := rsp.Patches[0]
				Expect(patch.Operation).To(Equal("add"))
				Expect(patch.Path).To(Equal("/metadata/labels"))
				Expect(patch.Value).To(Equal(map[string]interface{}{
					metadata.AuthorLabel: "admin",
				}))
			})

			It("should overwrite the author label value when one is provided by user", func() {
				releaseDifferentAuthor := &v1alpha1.Release{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "Release",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-release",
						Namespace: "default",
						Labels: map[string]string{
							metadata.AuthorLabel: "user",
						},
					},
					Spec: v1alpha1.ReleaseSpec{
						Snapshot:    "test-snapshot",
						ReleasePlan: "test-releaseplan",
					},
				}

				admissionRequest.Object.Raw, err = json.Marshal(releaseDifferentAuthor)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(len(rsp.Patches)).To(Equal(1))
				patch := rsp.Patches[0]
				Expect(patch.Operation).To(Equal("replace"))
				// The json functions replace `/` so checking the entire value does not work
				Expect(patch.Path).To(ContainSubstring("author"))
				Expect(patch.Value).To(Equal("admin"))
			})

			It("should not add the author label if the automated label is present and true", func() {
				release.Labels = map[string]string{
					metadata.AutomatedLabel: "true",
				}
				admissionRequest.Object.Raw, err = json.Marshal(release)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(rsp.AdmissionResponse.Patch).To(BeNil())
				Expect(len(rsp.Patches)).To(Equal(0))
			})

			It("should add the author label if the automated label is false", func() {
				release.Labels = map[string]string{
					metadata.AutomatedLabel: "false",
				}
				admissionRequest.Object.Raw, err = json.Marshal(release)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(len(rsp.Patches)).To(Equal(1))
				patch := rsp.Patches[0]
				Expect(patch.Operation).To(Equal("add"))
				// The json functions replace `/` so checking the entire value does not work
				Expect(patch.Path).To(ContainSubstring("author"))
				Expect(patch.Value).To(Equal("admin"))
			})
		})

		When("a Release is updated", func() {
			BeforeAll(func() {
				admissionRequest.AdmissionRequest.Operation = admissionv1.Update
			})

			It("should allow changes to metadata besides the author label", func() {
				release.ObjectMeta.Labels = map[string]string{
					metadata.AuthorLabel: "admin",
				}
				releaseMetadataChange := &v1alpha1.Release{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "Release",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-release",
						Namespace: "default",
						Labels: map[string]string{
							metadata.AuthorLabel: "admin",
						},
						Annotations: map[string]string{
							"foo": "bar",
						},
					},
					Spec: v1alpha1.ReleaseSpec{
						Snapshot:    "test-snapshot",
						ReleasePlan: "test-releaseplan",
					},
				}

				admissionRequest.Object.Raw, err = json.Marshal(release)
				Expect(err).NotTo(HaveOccurred())
				admissionRequest.OldObject.Raw, err = json.Marshal(releaseMetadataChange)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(rsp.AdmissionResponse.Result.Code).To(Equal(int32(http.StatusOK)))
				Expect(rsp.AdmissionResponse.Result.Message).To(Equal(metav1.StatusSuccess))
			})

			It("should not allow the author label to be set to a different value", func() {
				releaseMetadataChange := &v1alpha1.Release{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "Release",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-release",
						Namespace: "default",
						Labels: map[string]string{
							metadata.AuthorLabel: "user",
						},
					},
					Spec: v1alpha1.ReleaseSpec{
						Snapshot:    "test-snapshot",
						ReleasePlan: "test-releaseplan",
					},
				}

				admissionRequest.Object.Raw, err = json.Marshal(release)
				Expect(err).NotTo(HaveOccurred())
				admissionRequest.OldObject.Raw, err = json.Marshal(releaseMetadataChange)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeFalse())
				Expect(rsp.AdmissionResponse.Result).To(Equal(&metav1.Status{
					Code:    http.StatusBadRequest,
					Message: "release author label cannnot be updated",
				}))
			})
		})
	})

	Describe("A ReleasePlan request is made", func() {
		var releasePlan *v1alpha1.ReleasePlan

		BeforeEach(func() {
			admissionRequest.Kind.Kind = "ReleasePlan"

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
					Application: "test-application",
					Target:      "test-target",
				},
			}
		})

		When("a ReleasePlan is created", func() {
			BeforeAll(func() {
				admissionRequest.AdmissionRequest.Operation = admissionv1.Create
			})

			It("should add admin as the value for the author label if attribution is set to true", func() {
				releasePlan.Labels = map[string]string{
					metadata.AttributionLabel: "true",
				}
				admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(len(rsp.Patches)).To(Equal(1))
				patch := rsp.Patches[0]
				Expect(patch.Operation).To(Equal("add"))
				// The json functions replace `/` so checking the entire value does not work
				Expect(patch.Path).To(ContainSubstring("author"))
				Expect(patch.Value).To(Equal("admin"))
			})

			It("should allow the operation with no patch if the Attribution label is missing", func() {
				admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(rsp.AdmissionResponse.Patch).To(BeNil())
				Expect(len(rsp.Patches)).To(Equal(0))
			})

			It("should allow the operation with no patch if the Attribution label is false", func() {
				releasePlan.Labels = map[string]string{
					metadata.AttributionLabel: "false",
				}
				admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(rsp.AdmissionResponse.Patch).To(BeNil())
				Expect(len(rsp.Patches)).To(Equal(0))
			})
		})

		When("a ReleasePlan is updated", func() {
			var previousReleasePlan *v1alpha1.ReleasePlan

			BeforeEach(func() {
				admissionRequest.AdmissionRequest.Operation = admissionv1.Update
				previousReleasePlan = &v1alpha1.ReleasePlan{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "appstudio.redhat.com/v1alpha1",
						Kind:       "ReleasePlan",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "previous-releaseplan",
						Namespace: "default",
					},
					Spec: v1alpha1.ReleasePlanSpec{
						Application: "test-application",
						Target:      "test-target",
					},
				}
			})

			When("the Attribution label goes from true to true", func() {
				BeforeEach(func() {
					previousReleasePlan.Labels = map[string]string{
						metadata.AttributionLabel: "true",
					}
					releasePlan.Labels = map[string]string{
						metadata.AttributionLabel: "true",
					}
				})

				It("should maintain author value if trying to set it to a different user", func() {
					previousReleasePlan.Labels[metadata.AuthorLabel] = "admin"
					releasePlan.Labels[metadata.AuthorLabel] = "user"
					admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
					Expect(err).NotTo(HaveOccurred())
					admissionRequest.OldObject.Raw, err = json.Marshal(previousReleasePlan)
					Expect(err).NotTo(HaveOccurred())

					rsp := webhook.Handle(ctx, admissionRequest)
					Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
					Expect(rsp.AdmissionResponse.Patch).To(BeNil())
					Expect(len(rsp.Patches)).To(Equal(1))
					patch := rsp.Patches[0]
					Expect(patch.Operation).To(Equal("replace"))
					// The json functions replace `/` so checking the entire value does not work
					Expect(patch.Path).To(ContainSubstring("author"))
					Expect(patch.Value).To(Equal("admin"))
				})

				It("should allow the change if author value is not modified", func() {
					previousReleasePlan.Labels[metadata.AuthorLabel] = "user"
					releasePlan.Labels[metadata.AuthorLabel] = "user"
					admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
					Expect(err).NotTo(HaveOccurred())
					admissionRequest.OldObject.Raw, err = json.Marshal(previousReleasePlan)
					Expect(err).NotTo(HaveOccurred())

					rsp := webhook.Handle(ctx, admissionRequest)
					Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
					Expect(rsp.AdmissionResponse.Patch).To(BeNil())
					Expect(len(rsp.Patches)).To(Equal(0))
				})

				It("should allow changing the author to the current user", func() {
					previousReleasePlan.Labels[metadata.AuthorLabel] = "user"
					releasePlan.Labels[metadata.AuthorLabel] = "admin"
					admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
					Expect(err).NotTo(HaveOccurred())
					admissionRequest.OldObject.Raw, err = json.Marshal(previousReleasePlan)
					Expect(err).NotTo(HaveOccurred())

					rsp := webhook.Handle(ctx, admissionRequest)
					Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
					Expect(rsp.AdmissionResponse.Patch).To(BeNil())
					Expect(len(rsp.Patches)).To(Equal(0))
				})
			})

			When("the Attribution label goes to true to false", func() {
				BeforeAll(func() {
					previousReleasePlan.Labels = map[string]string{
						metadata.AttributionLabel: "true",
						metadata.AuthorLabel:      "admin",
					}
					releasePlan.Labels = map[string]string{
						metadata.AttributionLabel: "false",
						metadata.AuthorLabel:      "admin",
					}
				})

				It("should allow the change and remove the author label", func() {
					admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
					Expect(err).NotTo(HaveOccurred())
					admissionRequest.OldObject.Raw, err = json.Marshal(previousReleasePlan)
					Expect(err).NotTo(HaveOccurred())

					rsp := webhook.Handle(ctx, admissionRequest)
					Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())

					Expect(len(rsp.Patches)).To(Equal(1))
					patch := rsp.Patches[0]
					Expect(patch.Operation).To(Equal("remove"))
					// The json functions replace `/` so checking the entire value does not work
					Expect(patch.Path).To(ContainSubstring("author"))
				})
			})

			When("the Attribution label goes to false to true", func() {
				BeforeEach(func() {
					previousReleasePlan.Labels = map[string]string{
						metadata.AttributionLabel: "false",
					}
					releasePlan.Labels = map[string]string{
						metadata.AttributionLabel: "true",
					}
				})

				It("should set the author to be current user if provided different user", func() {
					releasePlan.Labels[metadata.AuthorLabel] = "user"
					admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
					Expect(err).NotTo(HaveOccurred())
					admissionRequest.OldObject.Raw, err = json.Marshal(previousReleasePlan)
					Expect(err).NotTo(HaveOccurred())

					rsp := webhook.Handle(ctx, admissionRequest)
					Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
					Expect(len(rsp.Patches)).To(Equal(1))
					patch := rsp.Patches[0]
					Expect(patch.Operation).To(Equal("replace"))
					// The json functions replace `/` so checking the entire value does not work
					Expect(patch.Path).To(ContainSubstring("author"))
					Expect(patch.Value).To(Equal("admin"))
				})
			})

			When("the Attribution label goes to false to false", func() {
				BeforeAll(func() {
					previousReleasePlan.Labels = map[string]string{
						metadata.AttributionLabel: "false",
					}
					releasePlan.Labels = map[string]string{
						metadata.AttributionLabel: "false",
					}
				})

				It("should allow the change", func() {
					admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
					Expect(err).NotTo(HaveOccurred())
					admissionRequest.OldObject.Raw, err = json.Marshal(previousReleasePlan)
					Expect(err).NotTo(HaveOccurred())

					rsp := webhook.Handle(ctx, admissionRequest)
					Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
					Expect(rsp.AdmissionResponse.Patch).To(BeNil())
					Expect(len(rsp.Patches)).To(Equal(0))
				})
			})

			It("should allow changes when releaseplans have no labels", func() {
				releasePlan.Labels = nil
				previousReleasePlan.Labels = nil
				admissionRequest.Object.Raw, err = json.Marshal(releasePlan)
				Expect(err).NotTo(HaveOccurred())
				admissionRequest.OldObject.Raw, err = json.Marshal(previousReleasePlan)
				Expect(err).NotTo(HaveOccurred())

				rsp := webhook.Handle(ctx, admissionRequest)
				Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
				Expect(rsp.AdmissionResponse.Patch).To(BeNil())
				Expect(len(rsp.Patches)).To(Equal(0))
			})
		})
	})

	When("patchResponse is called", func() {

		It("should return an admission response with a patch", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			}
			marshalledPod, err := json.Marshal(pod)
			Expect(err).NotTo(HaveOccurred())
			pod.Labels = map[string]string{
				"foo": "bar",
			}

			rsp := webhook.patchResponse(marshalledPod, pod)
			Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
			Expect(len(rsp.Patches)).To(Equal(1))
			patch := rsp.Patches[0]
			Expect(patch.Operation).To(Equal("add"))
			Expect(patch.Path).To(Equal("/metadata/labels"))
			Expect(patch.Value).To(Equal(map[string]interface{}{
				"foo": "bar",
			}))
		})
	})

	When("setAuthorLabel is called", func() {

		It("should add the author label", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			}
			webhook.setAuthorLabel("admin", pod)
			Expect(pod.GetLabels()).To(Equal(map[string]string{
				metadata.AuthorLabel: "admin",
				"foo":                "bar",
			}))
		})

		It("should add the author label if object has no existing labels", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			}
			webhook.setAuthorLabel("admin", pod)
			Expect(pod.GetLabels()).To(Equal(map[string]string{
				metadata.AuthorLabel: "admin",
			}))
		})
	})

	When("sanitizeLabelValue is called", func() {

		It("should convert : to _", func() {
			str := webhook.sanitizeLabelValue("a:b")
			Expect(str).To(Equal("a_b"))
		})

		It("should trim long author values", func() {
			str := webhook.sanitizeLabelValue("abcdefghijklmnopqrstuvwxyz_abcdefghijklmnopqrstuvwxyz_1234567890")
			Expect(str).To(Equal("abcdefghijklmnopqrstuvwxyz_abcdefghijklmnopqrstuvwxyz_123456789"))
		})
	})
})
