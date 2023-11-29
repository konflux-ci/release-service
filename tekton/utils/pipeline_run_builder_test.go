/*
Copyright 2023.

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

package utils

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

var _ = Describe("PipelineRun builder", func() {

	When("NewPipelineRunBuilder method is called", func() {
		var (
			namePrefix = "testPrefix"
			namespace  = "testNamespace"
			builder    *PipelineRunBuilder
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder(namePrefix, namespace)
		})

		It("should return a new PipelineRunBuilder instance", func() {
			Expect(builder).To(Not(BeNil()))
		})

		It("should set the correct GenerateName in the returned PipelineRunBuilder instance", func() {
			Expect(builder.pipelineRun.ObjectMeta.GenerateName).To(Equal(namePrefix + "-"))
		})

		It("should set the correct Namespace in the returned PipelineRunBuilder instance", func() {
			Expect(builder.pipelineRun.ObjectMeta.Namespace).To(Equal(namespace))
		})

		It("should initialize an empty PipelineRunSpec", func() {
			Expect(builder.pipelineRun.Spec).To(Equal(tektonv1.PipelineRunSpec{}))
		})
	})

	When("Build method is called", func() {
		It("should return the constructed PipelineRun if there are no errors", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			pr, err := builder.Build()
			Expect(pr).To(Not(BeNil()))
			Expect(err).To(BeNil())
		})

		It("should return the accumulated errors", func() {
			builder := &PipelineRunBuilder{
				err: multierror.Append(nil, fmt.Errorf("dummy error 1"), fmt.Errorf("dummy error 2")),
			}
			_, err := builder.Build()
			Expect(err).To(Not(BeNil()))
			Expect(err.Error()).To(ContainSubstring("dummy error 1"))
			Expect(err.Error()).To(ContainSubstring("dummy error 2"))
		})
	})

	When("WithAnnotations method is called", func() {
		var (
			builder *PipelineRunBuilder
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder("testPrefix", "testNamespace")
		})

		It("should add annotations when none previously existed", func() {
			builder.WithAnnotations(map[string]string{
				"annotation1": "value1",
				"annotation2": "value2",
			})
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation1", "value1"))
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation2", "value2"))
		})

		It("should update existing annotations and add new ones", func() {
			builder.pipelineRun.ObjectMeta.Annotations = map[string]string{
				"annotation1": "oldValue1",
				"annotation3": "value3",
			}
			builder.WithAnnotations(map[string]string{
				"annotation1": "newValue1",
				"annotation2": "value2",
			})
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation1", "newValue1"))
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation2", "value2"))
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation3", "value3"))
		})
	})

	When("WithFinalizer method is called", func() {
		var (
			builder *PipelineRunBuilder
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder("testPrefix", "testNamespace")
		})

		It("should add a finalizer when none previously existed", func() {
			builder.WithFinalizer("finalizer1")
			Expect(builder.pipelineRun.ObjectMeta.Finalizers).To(ContainElement("finalizer1"))
		})

		It("should append a new finalizer to the existing finalizers", func() {
			builder.pipelineRun.ObjectMeta.Finalizers = []string{"existingFinalizer"}
			builder.WithFinalizer("finalizer2")
			Expect(builder.pipelineRun.ObjectMeta.Finalizers).To(ContainElements("existingFinalizer", "finalizer2"))
		})
	})

	When("WithLabels method is called", func() {
		var (
			builder *PipelineRunBuilder
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder("testPrefix", "testNamespace")
		})

		It("should add labels when none previously existed", func() {
			builder.WithLabels(map[string]string{
				"label1": "value1",
				"label2": "value2",
			})
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label1", "value1"))
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label2", "value2"))
		})

		It("should update existing labels and add new ones", func() {
			builder.pipelineRun.ObjectMeta.Labels = map[string]string{
				"label1": "oldValue1",
				"label3": "value3",
			}
			builder.WithLabels(map[string]string{
				"label1": "newValue1",
				"label2": "value2",
			})
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label1", "newValue1"))
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label2", "value2"))
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label3", "value3"))
		})
	})

	When("WithObjectReferences method is called", func() {
		It("should add parameters based on the provided client.Objects", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			configMap1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configName1",
					Namespace: "configNamespace1",
				},
			}
			configMap1.Kind = "ConfigMap"
			configMap2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configName2",
					Namespace: "configNamespace2",
				},
			}
			configMap2.Kind = "ConfigMap"

			builder.WithObjectReferences(configMap1, configMap2)

			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(tektonv1.Param{
				Name:  "configMap",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "configNamespace1/configName1"},
			}))
			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(tektonv1.Param{
				Name:  "configMap",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "configNamespace2/configName2"},
			}))
		})
	})

	When("WithObjectSpecsAsJson method is called", func() {
		It("should add parameters with JSON representation of the object's Spec", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			pod1 := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container1",
							Image: "image1",
						},
					},
				},
			}
			pod1.Kind = "Pod"
			pod2 := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container2",
							Image: "image2",
						},
					},
				},
			}
			pod2.Kind = "Pod"

			builder.WithObjectSpecsAsJson(pod1, pod2)

			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(tektonv1.Param{
				Name: "pod",
				Value: tektonv1.ParamValue{
					Type:      tektonv1.ParamTypeString,
					StringVal: `{"containers":[{"name":"container1","image":"image1","resources":{}}]}`,
				},
			}))
			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(tektonv1.Param{
				Name: "pod",
				Value: tektonv1.ParamValue{
					Type:      tektonv1.ParamTypeString,
					StringVal: `{"containers":[{"name":"container2","image":"image2","resources":{}}]}`,
				},
			}))
		})
	})

	When("WithParams method is called", func() {
		It("should append the provided parameters to the PipelineRun's spec", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")

			param1 := tektonv1.Param{
				Name:  "param1",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "value1"},
			}
			param2 := tektonv1.Param{
				Name:  "param2",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "value2"},
			}

			builder.WithParams(param1, param2)

			Expect(builder.pipelineRun.Spec.Params).To(ContainElements(param1, param2))
		})
	})

	When("WithOwner method is called", func() {
		var (
			builder   *PipelineRunBuilder
			configMap *corev1.ConfigMap
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder("testPrefix", "testNamespace")
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configName",
					Namespace: "configNamespace",
				},
			}
			configMap.Kind = "Config"
		})

		It("should handle owner without errors", func() {
			builder.WithOwner(configMap)
			_, err := builder.Build()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have added owner annotations to the PipelineRun", func() {
			builder.WithOwner(configMap)
			Expect(builder.pipelineRun.Annotations).ToNot(BeEmpty())
		})
	})

	When("WithParamsFromConfigMap method is called", func() {
		It("should add parameters corresponding to the provided keys", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			configMap := &corev1.ConfigMap{
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			}

			builder.WithParamsFromConfigMap(configMap, []string{"key1", "key2", "key3"}) // "key3" doesn't exist in the ConfigMap.

			paramKey1 := tektonv1.Param{
				Name:  "key1",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "value1"},
			}
			paramKey2 := tektonv1.Param{
				Name:  "key2",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "value2"},
			}

			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(paramKey1))
			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(paramKey2))

			// Check that "key3" is not added as a Param since it doesn't exist in the ConfigMap.
			for _, param := range builder.pipelineRun.Spec.Params {
				Expect(param.Name).ToNot(Equal("key3"))
			}
		})
	})

	When("WithPipelineRef method is called", func() {
		It("should set the PipelineRef for the PipelineRun's spec", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			pipelineRef := &tektonv1.PipelineRef{
				Name:       "samplePipeline",
				APIVersion: "tekton.dev/v1",
			}

			builder.WithPipelineRef(pipelineRef)
			Expect(builder.pipelineRun.Spec.PipelineRef).To(Equal(pipelineRef))
		})
	})

	When("WithServiceAccount method is called", func() {
		It("should set the ServiceAccountName for the PipelineRun's TaskRunTemplate", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			serviceAccount := "sampleServiceAccount"
			builder.WithServiceAccount(serviceAccount)
			Expect(builder.pipelineRun.Spec.TaskRunTemplate.ServiceAccountName).To(Equal(serviceAccount))
		})
	})

	When("WithTimeouts method is called", func() {
		It("should set the timeouts for the PipelineRun", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			timeouts := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
				Tasks:    &metav1.Duration{Duration: 1 * time.Hour},
				Finally:  &metav1.Duration{Duration: 1 * time.Hour},
			}
			builder.WithTimeouts(timeouts)
			Expect(builder.pipelineRun.Spec.Timeouts).To(Equal(timeouts))
		})
	})

	When("WithWorkspaceFromVolumeTemplate method is called", func() {
		var (
			builder *PipelineRunBuilder
			name    string
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder("testPrefix", "testNamespace")
			name = "sampleWorkspace"
		})

		It("should add a new workspace binding to the PipelineRun's spec with the correct name and size", func() {
			size := "5Gi"
			builder.WithWorkspaceFromVolumeTemplate(name, size)
			Expect(len(builder.pipelineRun.Spec.Workspaces)).To(Equal(1))

			workspaceBinding := builder.pipelineRun.Spec.Workspaces[0]
			Expect(workspaceBinding.Name).To(Equal(name))
			workspaceQuantity := workspaceBinding.VolumeClaimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(workspaceQuantity.String()).To(Equal(size))
		})

		It("should fail if the size is not in the right format", func() {
			builder.WithWorkspaceFromVolumeTemplate(name, "invalid")
			_, err := builder.Build()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid size format"))
		})
	})
})
