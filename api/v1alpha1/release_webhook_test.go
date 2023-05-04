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
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/release-service/metadata"
	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("Release webhook", Ordered, func() {
	var release *Release
	var rHandler *releaseHandler
	var admissionRequest admission.Request
	var err error

	BeforeAll(func() {
		rHandler = &releaseHandler{
			Client: mgr.GetClient(),
		}

		admissionRequest.UserInfo.Username = "admin"
		admissionRequest.UID = types.UID("123")

		release = &Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-release",
				Namespace: "default",
			},
			Spec: ReleaseSpec{
				Snapshot:    "test-snapshot",
				ReleasePlan: "test-releaseplan",
			},
		}
	})

	Context("When a Release is created", func() {
		BeforeEach(func() {
			admissionRequest.AdmissionRequest.Operation = admissionv1.Create
		})
		It("should add admin as the value for the author label", func() {
			admissionRequest.Object.Raw, err = json.Marshal(release)
			Expect(err).NotTo(HaveOccurred())

			rsp := rHandler.Handle(ctx, admissionRequest)
			Expect(rsp.AdmissionResponse.UID).To(Equal(types.UID("123")))
			Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())

			var jsonPatch []jsonpatch.JsonPatchOperation
			err = json.Unmarshal(rsp.AdmissionResponse.Patch, &jsonPatch)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(jsonPatch)).To(Equal(1))
			patch := jsonPatch[0]
			Expect(patch.Operation).To(Equal("replace"))
			Expect(patch.Path).To(Equal("/metadata/labels"))
			Expect(patch.Value).To(Equal(map[string]interface{}{
				metadata.AuthorLabel: "admin",
			}))
		})
		It("should overwrite the author label value when one is provided by user", func() {
			releaseDifferentAuthor := &Release{
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
				Spec: ReleaseSpec{
					Snapshot:    "test-snapshot",
					ReleasePlan: "test-releaseplan",
				},
			}

			admissionRequest.Object.Raw, err = json.Marshal(releaseDifferentAuthor)
			Expect(err).NotTo(HaveOccurred())

			rsp := rHandler.Handle(ctx, admissionRequest)
			Expect(rsp.AdmissionResponse.UID).To(Equal(types.UID("123")))
			Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())

			var jsonPatch []jsonpatch.JsonPatchOperation
			err = json.Unmarshal(rsp.AdmissionResponse.Patch, &jsonPatch)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(jsonPatch)).To(Equal(1))
			patch := jsonPatch[0]
			Expect(patch.Operation).To(Equal("replace"))
			Expect(patch.Path).To(Equal("/metadata/labels"))
			Expect(patch.Value).To(Equal(map[string]interface{}{
				metadata.AuthorLabel: "admin",
			}))
		})
	})

	Context("When a Release is updated", func() {
		BeforeEach(func() {
			admissionRequest.AdmissionRequest.Operation = admissionv1.Update
		})
		It("should not allow spec updates", func() {
			releaseSpecChange := &Release{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appstudio.redhat.com/v1alpha1",
					Kind:       "Release",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-release",
					Namespace: "default",
				},
				Spec: ReleaseSpec{
					Snapshot:    "new-snapshot",
					ReleasePlan: "test-releaseplan",
				},
			}

			admissionRequest.Object.Raw, err = json.Marshal(release)
			Expect(err).NotTo(HaveOccurred())
			admissionRequest.OldObject.Raw, err = json.Marshal(releaseSpecChange)
			Expect(err).NotTo(HaveOccurred())

			rsp := rHandler.Handle(ctx, admissionRequest)
			Expect(rsp.AdmissionResponse.UID).To(Equal(types.UID("123")))
			Expect(rsp.AdmissionResponse.Allowed).To(BeFalse())
			Expect(rsp.AdmissionResponse.Result).To(Equal(&metav1.Status{
				Status:  "Failure",
				Message: "release resources spec cannot be updated",
			}))
		})
		It("should allow changes to metadata besides the author label", func() {
			release.ObjectMeta.Labels = map[string]string{
				metadata.AuthorLabel: "admin",
			}
			releaseMetadataChange := &Release{
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
				Spec: ReleaseSpec{
					Snapshot:    "test-snapshot",
					ReleasePlan: "test-releaseplan",
				},
			}

			admissionRequest.Object.Raw, err = json.Marshal(release)
			Expect(err).NotTo(HaveOccurred())
			admissionRequest.OldObject.Raw, err = json.Marshal(releaseMetadataChange)
			Expect(err).NotTo(HaveOccurred())

			rsp := rHandler.Handle(ctx, admissionRequest)
			Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
			Expect(rsp.AdmissionResponse.Result).To(Equal(&metav1.Status{
				Status: "Success",
			}))
		})
		It("should not allow the author label to be set to a different value", func() {
			releaseMetadataChange := &Release{
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
				Spec: ReleaseSpec{
					Snapshot:    "test-snapshot",
					ReleasePlan: "test-releaseplan",
				},
			}

			admissionRequest.Object.Raw, err = json.Marshal(release)
			Expect(err).NotTo(HaveOccurred())
			admissionRequest.OldObject.Raw, err = json.Marshal(releaseMetadataChange)
			Expect(err).NotTo(HaveOccurred())

			rsp := rHandler.Handle(ctx, admissionRequest)
			Expect(rsp.AdmissionResponse.UID).To(Equal(types.UID("123")))
			Expect(rsp.AdmissionResponse.Allowed).To(BeFalse())
			Expect(rsp.AdmissionResponse.Result).To(Equal(&metav1.Status{
				Status:  "Failure",
				Message: "release author label cannnot be updated",
			}))
		})
	})

	Context("When a Release is deleted", func() {
		BeforeEach(func() {
			admissionRequest.AdmissionRequest.Operation = admissionv1.Delete
		})
		It("should allow the operation not include a patch in the response", func() {
			rsp := rHandler.Handle(ctx, admissionRequest)
			Expect(rsp.AdmissionResponse.UID).To(Equal(types.UID("123")))
			Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
			Expect(rsp.AdmissionResponse.Result).To(Equal(&metav1.Status{
				Status: "Success",
			}))
		})
	})

	Context("When failedResponse is called", func() {
		It("should return a failed admission response", func() {
			rsp := failedResponse(types.UID("111"), "message")
			Expect(rsp.AdmissionResponse.UID).To(Equal(types.UID("111")))
			Expect(rsp.AdmissionResponse.Allowed).To(BeFalse())
			Expect(rsp.AdmissionResponse.Result).To(Equal(&metav1.Status{
				Status:  "Failure",
				Message: "message",
			}))
		})
	})

	Context("When patchAuthorResponse is called", func() {
		It("should return an admission response containing a patch", func() {
			rsp := patchAuthorResponse(admissionRequest)
			Expect(rsp.AdmissionResponse.UID).To(Equal(types.UID("123")))
			Expect(rsp.AdmissionResponse.Allowed).To(BeTrue())
			Expect(rsp.AdmissionResponse.Result).To(Equal(&metav1.Status{
				Status: "Success",
			}))
			Expect(rsp.AdmissionResponse.Patch).NotTo(BeNil())
			Expect(rsp.AdmissionResponse.PatchType).NotTo(BeNil())
		})
		It("should convert : to _ in the author", func() {
			admissionRequest.UserInfo.Username = "a:b"
			rsp := patchAuthorResponse(admissionRequest)

			var jsonPatch []jsonpatch.JsonPatchOperation
			err = json.Unmarshal(rsp.AdmissionResponse.Patch, &jsonPatch)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(jsonPatch)).To(Equal(1))
			patch := jsonPatch[0]
			Expect(patch.Value).To(Equal(map[string]interface{}{
				metadata.AuthorLabel: "a_b",
			}))
		})
		It("should convert 'system:serviceaccount' to 'sa' in the author", func() {
			admissionRequest.UserInfo.Username = "system:serviceaccount_foo"
			rsp := patchAuthorResponse(admissionRequest)

			var jsonPatch []jsonpatch.JsonPatchOperation
			err = json.Unmarshal(rsp.AdmissionResponse.Patch, &jsonPatch)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(jsonPatch)).To(Equal(1))
			patch := jsonPatch[0]
			Expect(patch.Value).To(Equal(map[string]interface{}{
				metadata.AuthorLabel: "sa_foo",
			}))
		})
		It("should convert only use the first 63 characters of the author", func() {
			admissionRequest.UserInfo.Username = "abcdefghijklmnopqrstuvwxyz_abcdefghijklmnopqrstuvwxyz_1234567890"
			rsp := patchAuthorResponse(admissionRequest)

			var jsonPatch []jsonpatch.JsonPatchOperation
			err = json.Unmarshal(rsp.AdmissionResponse.Patch, &jsonPatch)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(jsonPatch)).To(Equal(1))
			patch := jsonPatch[0]
			Expect(patch.Value).To(Equal(map[string]interface{}{
				metadata.AuthorLabel: "abcdefghijklmnopqrstuvwxyz_abcdefghijklmnopqrstuvwxyz_123456789",
			}))
		})
	})
})
