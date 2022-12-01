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

package gitops

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Utils", Ordered, func() {
	const (
		namespace       = "default"
		applicationName = "test-application"
		environmentName = "test-environment"
		snapshotName    = "test-snapshot"
	)

	var pod *corev1.Pod
	var binding, bindingUnknownStatus *applicationapiv1alpha1.SnapshotEnvironmentBinding

	BeforeAll(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    namespace,
				GenerateName: "testpod-",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test",
						Image: "test",
					},
				},
			},
		}
		binding = &applicationapiv1alpha1.SnapshotEnvironmentBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "binding",
				Namespace: namespace,
			},
			Spec: applicationapiv1alpha1.SnapshotEnvironmentBindingSpec{
				Application: applicationName,
				Environment: environmentName,
				Snapshot:    snapshotName,
				Components:  []applicationapiv1alpha1.BindingComponent{},
			},
		}
		bindingUnknownStatus = &applicationapiv1alpha1.SnapshotEnvironmentBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bindingunknownstatus",
				Namespace: namespace,
			},
			Spec: applicationapiv1alpha1.SnapshotEnvironmentBindingSpec{
				Application: applicationName,
				Environment: environmentName,
				Snapshot:    snapshotName,
				Components:  []applicationapiv1alpha1.BindingComponent{},
			},
		}
		ctx := context.Background()

		Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
		Expect(k8sClient.Create(ctx, binding)).Should(Succeed())
		Expect(k8sClient.Create(ctx, bindingUnknownStatus)).Should(Succeed())

		// Set the status of the unknown status binding after it is created
		bindingUnknownStatus.Status.ComponentDeploymentConditions = []metav1.Condition{
			{
				Type:   v1alpha1.BindingDeploymentStatusConditionType,
				Status: metav1.ConditionUnknown,
			},
		}
	})

	AfterAll(func() {
		err := k8sClient.Delete(ctx, pod)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, binding)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
		err = k8sClient.Delete(ctx, bindingUnknownStatus)
		Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())
	})

	Context("when using utility functions on SnapshotEnvironmentBinding objects", func() {
		It("returns false when called with an object that isn't a SnapshotEnvironmentBinding", func() {
			Expect(hasDeploymentFinished(pod, binding)).To(Equal(false))
		})

		It("returns false when a SnapshotEnvironmentBinding has no status field", func() {
			Expect(hasDeploymentFinished(binding, binding)).To(Equal(false))
		})

		It("returns false when the new SnapshotEnvironmentBinding does not have status set to true or false", func() {
			Expect(hasDeploymentFinished(bindingUnknownStatus, bindingUnknownStatus)).To(Equal(false))
		})

		It("returns true when the old SnapshotEnvironmentBinding has unknown status and the new one has false status", func() {
			binding.Status.ComponentDeploymentConditions = []metav1.Condition{
				{
					Type:   v1alpha1.BindingDeploymentStatusConditionType,
					Status: metav1.ConditionFalse,
				},
			}
			Expect(hasDeploymentFinished(bindingUnknownStatus, binding)).To(Equal(true))
		})

		It("returns true when the old SnapshotEnvironmentBinding has unknown status and the new one has true status", func() {
			binding.Status.ComponentDeploymentConditions = []metav1.Condition{
				{
					Type:   v1alpha1.BindingDeploymentStatusConditionType,
					Status: metav1.ConditionTrue,
				},
			}
			Expect(hasDeploymentFinished(bindingUnknownStatus, binding)).To(Equal(true))
		})
	})
})
