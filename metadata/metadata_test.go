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

package metadata

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMetadata(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metadata Test Suite")
}

var _ = Describe("Metadata", func() {

	Context("addEntries function", func() {
		When("called with an empty destination map", func() {
			src := map[string]string{
				"pet/dog":   "bark",
				"pet/cat":   "meow",
				"pond/frog": "ribit",
			}
			dst := map[string]string{}
			addEntries(src, dst)

			It("should copy all src key/value pairs unconditionally", func() {
				Expect(dst["pet/dog"]).To(Equal("bark"))
				Expect(dst["pet/cat"]).To(Equal("meow"))
				Expect(dst["pond/frog"]).To(Equal("ribit"))
			})
			It("source and destination maps should have the same length", func() {
				Expect(len(dst)).To(Equal(len(src)))
			})
		})
		When("called with a non-empty destination map containing duplicate keys", func() {
			src := map[string]string{
				"pet/dog":   "bark",
				"pet/cat":   "meow",
				"pond/frog": "ribit",
			}
			dst := map[string]string{
				"pet/dog": "howl",
			}
			addEntries(src, dst)
			It("should only copy source key/value pairs that do not already exist in the destination map", func() {
				Expect(dst["pet/dog"]).To(Equal("howl"))
				Expect(dst["pet/cat"]).To(Equal("meow"))
				Expect(dst["pond/frog"]).To(Equal("ribit"))
			})
			It("should have increased the length of the destination map from 1 to 3", func() {
				Expect(len(dst)).To(Equal(3))
			})
		})
	})

	Context("filterByPrefix function", func() {
		When("called with an empty prefix string", func() {
			src := map[string]string{
				"pet/dog":   "bark",
				"pet/cat":   "meow",
				"pond/frog": "ribit",
			}
			dst := filterByPrefix(src, "")
			It("should return a map identical to the map given", func() {
				Expect(dst["pet/dog"]).To(Equal("bark"))
				Expect(dst["pet/cat"]).To(Equal("meow"))
				Expect(dst["pond/frog"]).To(Equal("ribit"))
			})
			It("input and returned maps should have the same length", func() {
				Expect(len(dst)).To(Equal(len(src)))
			})
		})
		When("called with an non-empty prefix string", func() {
			src := map[string]string{
				"pet/dog":   "bark",
				"pet/cat":   "meow",
				"pond/frog": "ribit",
			}
			dst := filterByPrefix(src, "pond")
			It("should return a new map of key/value pairs where the key(s) matched the given prefix", func() {
				Expect(dst["pond/frog"]).To(Equal("ribit"))
			})
			It("returned map length should equal exactly 1.", func() {
				Expect(len(dst)).To(Equal(1))
			})
		})
	})

	Context("safeCopy function", func() {
		When("called with a key not already present in the map", func() {
			dst := map[string]string{"foo/dog": "bark"}
			safeCopy(dst, "foo/cat", "meow")
			It("should set the given key/value in the map", func() {
				Expect(dst["foo/dog"]).To(Equal("bark"))
				Expect(dst["foo/cat"]).To(Equal("meow"))
			})
			It("should have increased the map length from 1 to 2", func() {
				Expect(len(dst)).To(Equal(2))
			})
		})
		When("called with a key already present in dst map", func() {
			dst := map[string]string{"foo/dog": "bark"}
			safeCopy(dst, "foo/dog", "meow")
			It("should not set the given key/value pair into the map", func() {
				Expect(dst["foo/dog"]).To(Equal("bark"))
			})
			It("should not increase the map length", func() {
				Expect(len(dst)).To(Equal(1))
			})
		})
	})

	Context("AddAnnotations function", func() {
		When("called with a pipelineRun containing nil Annotations", func() {
			pipelineRun := &tektonv1.PipelineRun{
				ObjectMeta: v1.ObjectMeta{
					Annotations: nil,
					Labels:      nil,
				},
			}
			AddAnnotations(pipelineRun, map[string]string{})
			It("should create the Annotations map", func() {
				Expect(pipelineRun.GetAnnotations()).ToNot(BeNil())
			})
			It("should do nothing with the Labels map", func() {
				Expect(pipelineRun.GetLabels()).To(BeNil())
			})
		})
	})

	Context("AddLabels function", func() {
		When("called with an object containing nil Labels", func() {
			pod := &corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Annotations: nil,
					Labels:      nil,
				},
			}
			AddLabels(pod, map[string]string{})
			It("should create the Labels map", func() {
				Expect(pod.GetLabels()).ToNot(BeNil())
			})
			It("should do nothing with the Annotations map", func() {
				Expect(pod.GetAnnotations()).To(BeNil())
			})
		})
	})

	Context("GetAnnotationsWithPrefix function", func() {
		When("calling filterByPrefix with GetAnnotations()", func() {
			pod := &corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"pet/dog": "bark",
					},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			}
			dst := GetAnnotationsWithPrefix(pod, "pet/")
			It("should only fetch Annotations", func() {
				Expect(dst["pet/dog"]).To(Equal("bark"))
			})
			It("should not fetch Labels", func() {
				Expect(dst["foo"]).To(BeEmpty())
			})
		})
	})

	Context("GetLabelsWithPrefix function", func() {
		When("calling filterByPrefix with GetLabels()", func() {
			pod := &corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
					Labels: map[string]string{
						"pet/dog": "bark",
					},
				},
			}
			dst := GetLabelsWithPrefix(pod, "pet/")
			It("should only fetch Labels", func() {
				Expect(dst["pet/dog"]).To(Equal("bark"))
			})
			It("should not fetch Annotations", func() {
				Expect(dst["foo"]).To(BeEmpty())
			})
		})
	})
})
