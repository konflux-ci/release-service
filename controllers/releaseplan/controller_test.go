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

package releaseplan

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ReleasePlan Controller", Ordered, func() {

	Context("When NewReleasePlanReconciler is called", func() {
		It("creates and return a new Reconciler", func() {
			Expect(reflect.TypeOf(NewReleasePlanReconciler(k8sClient, &ctrl.Log, scheme.Scheme))).To(Equal(reflect.TypeOf(&Reconciler{})))
		})
	})

	// For the Reconcile function test we don't want to make a successful call as it will call every single operation
	// defined there. We don't have any control over the operations being executed, and we want to keep a clean env for
	// the adapter tests.
	Context("When Reconcile is called", func() {
		It("should succeed even if the releasePlan is not found", func() {
			reconciler := NewReleasePlanReconciler(k8sClient, &ctrl.Log, scheme.Scheme)
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: "default",
				},
			}
			result, err := reconciler.Reconcile(ctx, req)
			Expect(reflect.TypeOf(result)).To(Equal(reflect.TypeOf(reconcile.Result{})))
			Expect(err).To(BeNil())
		})
	})

	Context("When SetupController is called", func() {
		It("should setup the controller successfully", func() {
			manager, _ := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:             scheme.Scheme,
				MetricsBindAddress: "0", // disable metrics
				LeaderElection:     false,
			})
			Expect(SetupController(manager, &ctrl.Log)).To(Succeed())
		})
	})

	Context("When setupControllerWithManager is called", func() {
		It("should setup the controller successfully", func() {
			reconciler := NewReleasePlanReconciler(k8sClient, &ctrl.Log, scheme.Scheme)
			manager, _ := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:             scheme.Scheme,
				MetricsBindAddress: "0", // disable metrics
				LeaderElection:     false,
			})
			Expect(setupControllerWithManager(manager, reconciler)).To(Succeed())
		})
	})

})
