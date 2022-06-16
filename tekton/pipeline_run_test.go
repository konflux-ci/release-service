/**
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

var _ = Describe("PipelineRun", func() {

	var params *ReleasePipelineParams
	var rplr *ReleasePipelineRun

	var release *v1alpha1.Release
	var releaseParams *v1alpha1.ReleaseSpec

	BeforeEach(func() {

		params = &ReleasePipelineParams{
			Prefix:    "mypipeline",
			Namespace: "default",
			// @todo: use a valid Pipeline params
			Extra: ExtraParams{
				Name:  "ExtraParam",
				Value: tektonv1beta1.ArrayOrString{Type: tektonv1beta1.ParamTypeString, StringVal: "ExtraValue"},
			},
		}
		releaseParams = &v1alpha1.ReleaseSpec{
			ApplicationSnapshot: "mysnapshot",
			ReleaseLink:         "myreleaselink",
		}

		release = &v1alpha1.Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1",
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "myrelease-",
				Namespace:    params.Namespace,
				UID:          "8ba3294a-2e1b-4a13-9b3a-faf3cfe3d31b", // This is an arbitrary random UUID
			},
			Spec: *releaseParams,
		}

		ctx := context.Background()

		// The code below sets the ownership for the Release Object
		kind := reflect.TypeOf(v1alpha1.Release{}).Name()
		gvk := v1alpha1.GroupVersion.WithKind(kind)
		controllerRef := metav1.NewControllerRef(release, gvk)

		// creating a release
		Expect(k8sClient.Create(ctx, release)).Should(Succeed())
		release.SetOwnerReferences([]metav1.OwnerReference{*controllerRef})

		// need to set the Kind as it loses it for reasons...
		release.TypeMeta.APIVersion = "appstudio.redhat.com/v1alpha1"
		release.TypeMeta.Kind = "Release"

		// Creates the PipelineRun Object
		rplr = NewReleasePipelineRun(params.Prefix, params.Namespace)
		Expect(k8sClient.Create(ctx, rplr.AsPipelineRun())).Should(Succeed())

	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, release)
		_ = k8sClient.Delete(ctx, rplr.AsPipelineRun())
	})

	Context("Pipeline Run", func() {

		It("Can create a ReleasePipelineRun", func() {
			Expect(rplr.ObjectMeta.Name).
				Should(MatchRegexp("mypipeline-" + `[a-z0-9]{5}`))
			Expect(rplr.ObjectMeta.Namespace).To(Equal("default"))
		})

		It("Can add extra params to ReleasePipelineRun", func() {
			rplr.WithExtraParam(params.Extra.Name, params.Extra.Value)

			Expect(rplr.Spec.Params[0].Name).To(Equal("ExtraParam"))
			Expect(rplr.Spec.Params[0].Value.StringVal).To(Equal("ExtraValue"))
		})

		It("Can add the release Owner annotations to ReleasePipelineRun", func() {
			rplr.WithOwner(release)
			Expect(rplr.Annotations).NotTo(BeNil())
		})

		It("Can add the release Labels to ReleasePipelineRun", func() {
			rplr.WithRelease(release)
			Expect(rplr.Labels["release.appstudio.openshift.io/name"]).
				Should(MatchRegexp(release.GenerateName + `[a-z1-9]{5}`))
		})

		It("Can cast ReleasePipelineRun type to PipelineRun", func() {
			Expect(reflect.TypeOf(rplr.AsPipelineRun())).
				To(Equal(reflect.TypeOf(&tektonv1beta1.PipelineRun{})))
		})

	})
})
