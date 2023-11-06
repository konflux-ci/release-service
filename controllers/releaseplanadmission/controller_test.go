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

package releaseplanadmission

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ReleasePlanAdmission Controller", Ordered, func() {
	// For the Reconcile function test we don't want to make a successful call as it will call every single operation
	// defined there. We don't have any control over the operations being executed, and we want to keep a clean env for
	// the adapter tests.
	When("Reconcile is called", func() {
		It("should succeed even if the releasePlanAdmission is not found", func() {
			controller := &Controller{
				client: k8sClient,
				log:    ctrl.Log,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: "default",
				},
			}
			result, err := controller.Reconcile(ctx, req)
			Expect(reflect.TypeOf(result)).To(Equal(reflect.TypeOf(reconcile.Result{})))
			Expect(err).To(BeNil())
		})
	})

	When("Register is called", func() {
		It("should setup the controller successfully", func() {
			controller := &Controller{
				client: k8sClient,
				log:    ctrl.Log,
			}

			mgr, _ := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: scheme.Scheme,
				Metrics: server.Options{
					BindAddress: "0", // disables metrics
				},
				LeaderElection: false,
			})
			Expect(controller.Register(mgr, &ctrl.Log, nil)).To(Succeed())
		})
	})

})
