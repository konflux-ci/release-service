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

package tekton

import (
	"context"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/release-service/api/v1alpha1"

	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ExtraParams struct {
	Name  string
	Value tektonv1beta1.ArrayOrString
}

type ReleasePipelineParams struct {
	Prefix    string
	Namespace string
	Extra     ExtraParams
}

const (
	PipelineRunPrefixName = "mypipeline"
	Namespace             = "default"
)

var _ = Describe("PipelineRun", func() {

	var release *v1alpha1.Release
	var extraParams *ExtraParams
	var releasePipelineRun *ReleasePipelineRun

	BeforeEach(func() {

		extraParams = &ExtraParams{
			Name: "extraConfigPath",
			Value: tektonv1beta1.ArrayOrString{
				Type:      tektonv1beta1.ParamTypeString,
				StringVal: "path/to/extra/config.yaml",
			},
		}
		release = &v1alpha1.Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "myrelease-",
				Namespace:    Namespace,
			},
			Spec: v1alpha1.ReleaseSpec{
				ApplicationSnapshot: "mysnapshot",
				ReleaseLink:         "myreleaselink",
			},
		}

		ctx := context.Background()

		// The code below sets the ownership for the Release Object
		kind := reflect.TypeOf(v1alpha1.Release{}).Name()
		gvk := v1alpha1.GroupVersion.WithKind(kind)
		controllerRef := metav1.NewControllerRef(release, gvk)

		// Creating a release
		Expect(k8sClient.Create(ctx, release)).Should(Succeed())
		release.SetOwnerReferences([]metav1.OwnerReference{*controllerRef})

		// Need to set the Kind and APIVersion as it loses it due to:
		// https://github.com/kubernetes-sigs/controller-runtime/issues/1870
		release.TypeMeta.APIVersion = "appstudio.redhat.com/v1alpha1"
		release.TypeMeta.Kind = "Release"

		// Creates the PipelineRun Object
		releasePipelineRun = NewReleasePipelineRun(PipelineRunPrefixName, Namespace)
		Expect(k8sClient.Create(ctx, releasePipelineRun.AsPipelineRun())).Should(Succeed())

	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, release)
		_ = k8sClient.Delete(ctx, releasePipelineRun.AsPipelineRun())
	})

	Context("Create PipelineRun and modify its attributes", func() {

		It("Can create a ReleasePipelineRun", func() {
			Expect(releasePipelineRun.ObjectMeta.Name).
				Should(MatchRegexp(PipelineRunPrefixName + `-[a-z0-9]{5}`))
			Expect(releasePipelineRun.ObjectMeta.Namespace).To(Equal(Namespace))
		})

		It("Can add extra params to ReleasePipelineRun", func() {
			releasePipelineRun.WithExtraParam(extraParams.Name, extraParams.Value)
			Expect(releasePipelineRun.Spec.Params[0].Name).To(Equal(extraParams.Name))
			Expect(releasePipelineRun.Spec.Params[0].Value.StringVal).To(Equal(extraParams.Value.StringVal))
		})

		It("Can add the release Owner annotations to ReleasePipelineRun", func() {
			releasePipelineRun.WithOwner(release)
			Expect(releasePipelineRun.Annotations).NotTo(BeNil())
		})

		It("Can add the release Labels to ReleasePipelineRun", func() {
			releasePipelineRun.WithReleaseLabels(release.Name, release.Namespace)
			Expect(releasePipelineRun.Labels["release.appstudio.openshift.io/name"]).
				To(Equal(release.Name))
		})

		It("Can cast ReleasePipelineRun type to PipelineRun", func() {
			Expect(reflect.TypeOf(releasePipelineRun.AsPipelineRun())).
				To(Equal(reflect.TypeOf(&tektonv1beta1.PipelineRun{})))
		})

	})
})
