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

package release

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Release Controller", Ordered, func() {

	When("NewReleaseReconciler is called", func() {
		It("creates and return a new Reconciler", func() {
			Expect(reflect.TypeOf(NewReleaseReconciler(k8sClient, &ctrl.Log, scheme.Scheme))).To(Equal(reflect.TypeOf(&Reconciler{})))
		})
	})

	// For the Reconcile function test we don't want to make a successful call as it will call every single operation
	// defined there. We don't have any control over the operations being executed, and we want to keep a clean env for
	// the adapter tests.
	When("Reconcile is called", func() {
		It("should succeed even if the release is not found", func() {
			reconciler := NewReleaseReconciler(k8sClient, &ctrl.Log, scheme.Scheme)
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

	When("Register is called", func() {
		It("should setup the controller successfully", func() {
			reconciler := NewReleaseReconciler(k8sClient, &ctrl.Log, scheme.Scheme)
			manager, _ := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:             scheme.Scheme,
				MetricsBindAddress: "0", // disable metrics
				LeaderElection:     false,
			})
			Expect(reconciler.Register(manager, &ctrl.Log, nil)).To(Succeed())
		})
	})

	When("SetupCache is called", func() {
		It("should setup the cache successfully", func() {
			reconciler := NewReleaseReconciler(k8sClient, &ctrl.Log, scheme.Scheme)
			manager, _ := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:             scheme.Scheme,
				MetricsBindAddress: "0", // disable metrics
				LeaderElection:     false,
			})
			Expect(reconciler.SetupCache(manager)).To(Succeed())
		})
	})

})
