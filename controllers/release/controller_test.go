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
	"os"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/konflux-ci/release-service/api/v1alpha1"
)

var _ = Describe("Release Controller", Ordered, func() {
	// For the Reconcile function test we don't want to make a successful call as it will call every single operation
	// defined there. We don't have any control over the operations being executed, and we want to keep a clean env for
	// the adapter tests.
	When("Reconcile is called", func() {
		It("should succeed even if the release is not found", func() {
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

		// This Release is deliberately invalid (it references a non-existent ReleasePlan) and
		// SERVICE_NAMESPACE is deliberately left unset, so EnsureConfigIsLoaded halts the
		// operation chain on the very first real operation, after a single status patch and
		// before any finalizer is added or pipeline processing is attempted. This lets the test
		// reach the adapter construction in Reconcile (and therefore exercise the apiReader
		// wiring) without running the full operation pipeline against the shared test env.
		It("constructs the adapter using the configured apiReader when the release exists", func() {
			previousServiceNamespace, hadServiceNamespace := os.LookupEnv("SERVICE_NAMESPACE")
			Expect(os.Unsetenv("SERVICE_NAMESPACE")).To(Succeed())
			defer func() {
				if hadServiceNamespace {
					os.Setenv("SERVICE_NAMESPACE", previousServiceNamespace)
				}
			}()

			release := &v1alpha1.Release{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "coverage-",
					Namespace:    "default",
				},
				Spec: v1alpha1.ReleaseSpec{
					Snapshot:    "non-existent-snapshot",
					ReleasePlan: "non-existent-releaseplan",
				},
			}
			Expect(k8sClient.Create(ctx, release)).To(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, release)).To(Succeed())
			}()

			controller := &Controller{
				client:    k8sClient,
				apiReader: k8sClient,
				log:       ctrl.Log,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      release.Name,
					Namespace: release.Namespace,
				},
			}
			_, err := controller.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("SetupCache is called", func() {
		It("should setup the cache successfully", func() {
			controller := &Controller{
				client: k8sClient,
				log:    ctrl.Log,
			}

			manager, _ := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: scheme.Scheme,
				Metrics: server.Options{
					BindAddress: "0", // disables metrics
				},
				LeaderElection: false,
			})
			Expect(controller.SetupCache(manager)).To(Succeed())
		})
	})

	When("Register is called", func() {
		It("should setup the controller successfully", func() {
			controller := &Controller{
				client:                  k8sClient,
				log:                     ctrl.Log,
				MaxConcurrentReconciles: 10,
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
