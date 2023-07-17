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

package syncer

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
)

type key int

const (
	syncerContextKey key = iota
)

var _ = Describe("Syncer", Ordered, func() {
	const targetNamespace string = "syncer"

	snapshot := &applicationapiv1alpha1.Snapshot{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				"foo": "bar",
			},
			GenerateName: "snapshot-",
			Labels: map[string]string{
				"foo": "bar",
			},
			Namespace: "default",
		},
		Spec: applicationapiv1alpha1.SnapshotSpec{
			Application: "app",
			Components: []applicationapiv1alpha1.SnapshotComponent{
				{
					Name:           "foo",
					ContainerImage: "quay.io/foo",
				},
			},
		},
	}

	BeforeAll(func() {
		namespace := &v12.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: targetNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		Expect(k8sClient.Create(ctx, snapshot)).To(Succeed())
	})

	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, snapshot)).To(Succeed())
	})

	It("can create a new Syncer without specifying a context", func() {
		syncer := NewSyncer(k8sClient, &ctrl.Log)
		Expect(reflect.TypeOf(syncer)).To(Equal(reflect.TypeOf(&Syncer{})))
	})

	It("can create a new Syncer specifying a context", func() {
		syncer := NewSyncerWithContext(k8sClient, &ctrl.Log, context.TODO())
		Expect(reflect.TypeOf(syncer)).To(Equal(reflect.TypeOf(&Syncer{})))
	})

	It("can modify the context being used", func() {
		syncerContext := context.WithValue(context.TODO(), syncerContextKey, "bar")
		syncer := NewSyncerWithContext(k8sClient, &ctrl.Log, syncerContext)
		Expect(syncer.ctx.Value(syncerContextKey)).To(Equal("bar"))
		syncer.SetContext(context.TODO())
		Expect(syncer.ctx.Value(syncerContextKey)).To(BeNil())
	})

	It("can sync an snapshot into a given namespace", func() {
		syncer := NewSyncer(k8sClient, &ctrl.Log)

		Expect(syncer.SyncSnapshot(snapshot, targetNamespace)).To(Succeed())

		syncedSnapshot := &applicationapiv1alpha1.Snapshot{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      snapshot.Name,
			Namespace: targetNamespace,
		}, syncedSnapshot)).To(Succeed())
		Expect(*syncedSnapshot).To(MatchFields(IgnoreExtras, Fields{
			"ObjectMeta": MatchFields(IgnoreExtras, Fields{
				"Annotations": HaveLen(len(snapshot.Annotations)),
				"Labels":      HaveLen(len(snapshot.Labels)),
			}),
			"Spec": MatchFields(IgnoreExtras, Fields{
				"Application": Equal(snapshot.Spec.Application),
				"Components":  HaveLen(len(snapshot.Spec.Components)),
			}),
		}))

		Expect(k8sClient.Delete(ctx, syncedSnapshot)).To(Succeed())
	})
})
