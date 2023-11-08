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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/release-service/api/v1alpha1"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ExtraParams struct {
	Name  string
	Value tektonv1.ParamValue
}

var _ = Describe("PipelineRun", func() {
	const (
		pipelineRunPrefixName = "test-pipeline"
		namespace             = "default"
		workspace             = "test-workspace"
		persistentVolumeClaim = "test-pvc"
		serviceAccountName    = "test-service-account"
		timeout               = "1h0m0s"
		apiVersion            = "appstudio.redhat.com/v1alpha1"
		applicationName       = "test-application"
	)
	var (
		release                     *v1alpha1.Release
		extraParams                 *ExtraParams
		releasePipelineRun          *ReleasePipelineRun
		releasePlanAdmission        *v1alpha1.ReleasePlanAdmission
		enterpriseContractConfigMap *corev1.ConfigMap
		enterpriseContractPolicy    *ecapiv1alpha1.EnterpriseContractPolicy
	)
	BeforeEach(func() {
		extraParams = &ExtraParams{
			Name: "extraConfigPath",
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: "path/to/extra/config.yaml",
			},
		}
		release = &v1alpha1.Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiVersion,
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "myrelease-",
				Namespace:    namespace,
			},
			Spec: v1alpha1.ReleaseSpec{
				Snapshot:    "testsnapshot",
				ReleasePlan: "testreleaseplan",
			},
		}
		enterpriseContractConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cm",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "ConfigMap",
			},
			Data: map[string]string{
				"verify_ec_task_bundle": "test-bundle",
			},
		}
		enterpriseContractPolicy = &ecapiv1alpha1.EnterpriseContractPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testpolicy",
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "EnterpriseContractPolicy",
			},
			Spec: ecapiv1alpha1.EnterpriseContractPolicySpec{
				Description: "test-policy-description",
				Sources: []ecapiv1alpha1.Source{
					{
						Name:   "foo",
						Policy: []string{"https://github.com/company/policy"},
						Data:   []string{"https://github.com/company/data"},
					},
				},
			},
		}
		releasePlanAdmission = &v1alpha1.ReleasePlanAdmission{
			Spec: v1alpha1.ReleasePlanAdmissionSpec{
				Applications: []string{"application"},
				PipelineRef: &tektonutils.PipelineRef{
					Resolver: "bundles",
					Params: []tektonutils.Param{
						{Name: "bundle", Value: "testbundle"},
						{Name: "name", Value: "release-pipeline"},
						{Name: "kind", Value: "pipeline"},
					},
				},
				Policy:         "testpolicy",
				ServiceAccount: serviceAccountName,
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
		release.TypeMeta.APIVersion = apiVersion
		release.TypeMeta.Kind = "Release"

		// Creates the PipelineRun Object
		releasePipelineRun = NewReleasePipelineRun(pipelineRunPrefixName, namespace)
		Expect(k8sClient.Create(ctx, releasePipelineRun.AsPipelineRun())).Should(Succeed())
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, release)
		_ = k8sClient.Delete(ctx, releasePipelineRun.AsPipelineRun())
	})

	When("managing a new PipelineRun", func() {

		It("can create a PipelineRun and the returned object name is prefixed with the provided GenerateName", func() {
			Expect(releasePipelineRun.ObjectMeta.Name).
				Should(HavePrefix(pipelineRunPrefixName))
			Expect(releasePipelineRun.ObjectMeta.Namespace).To(Equal(namespace))
		})

		It("can append extra params to PipelineRun and these parameters are present in the object Specs", func() {
			releasePipelineRun.WithExtraParam(extraParams.Name, extraParams.Value)
			Expect(releasePipelineRun.Spec.Params[0].Name).To(Equal(extraParams.Name))
			Expect(releasePipelineRun.Spec.Params[0].Value.StringVal).
				To(Equal(extraParams.Value.StringVal))
		})

		It("can append owner release information to the object as annotations", func() {
			releasePipelineRun.WithOwner(release)
			Expect(releasePipelineRun.Annotations).NotTo(BeNil())
			Expect(releasePipelineRun.Finalizers).NotTo(BeEmpty())
		})

		It("can append the release Name, Namespace, and Application to a PipelineRun object and that these label key names match the correct label format", func() {
			releasePipelineRun.WithReleaseAndApplicationMetadata(release, applicationName)
			Expect(releasePipelineRun.Labels["release.appstudio.openshift.io/name"]).
				To(Equal(release.Name))
			Expect(releasePipelineRun.Labels["release.appstudio.openshift.io/namespace"]).
				To(Equal(release.Namespace))
			Expect(releasePipelineRun.Labels["appstudio.openshift.io/application"]).
				To(Equal(applicationName))
		})

		It("can return a PipelineRun object from a PipelineRun object", func() {
			Expect(reflect.TypeOf(releasePipelineRun.AsPipelineRun())).
				To(Equal(reflect.TypeOf(&tektonv1.PipelineRun{})))
		})

		It("can add the PipelineRef to a PipelineRun object ", func() {
			releasePipelineRun.WithPipelineRef(releasePlanAdmission.Spec.PipelineRef.ToTektonPipelineRef())
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef).NotTo(Equal(tektonv1.ResolverRef{}))
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef.Resolver).To(Equal(tektonv1.ResolverName("bundles")))
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef.Params).To(HaveLen(3))
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef.Params[0].Name).To(Equal("bundle"))
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef.Params[0].Value.StringVal).To(Equal(releasePlanAdmission.Spec.PipelineRef.Params[0].Value))
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef.Params[1].Name).To(Equal("name"))
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef.Params[1].Value.StringVal).To(Equal(releasePlanAdmission.Spec.PipelineRef.Params[1].Value))
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef.Params[2].Name).To(Equal("kind"))
			Expect(releasePipelineRun.Spec.PipelineRef.ResolverRef.Params[2].Value.StringVal).To(Equal("pipeline"))
		})

		It("can add the reference to the service account that should be used", func() {
			releasePipelineRun.WithServiceAccount(serviceAccountName)
			Expect(releasePipelineRun.Spec.TaskRunTemplate.ServiceAccountName).To(Equal(serviceAccountName))
		})

		It("can add the timeout that should be used", func() {
			releasePipelineRun.WithTimeout(timeout)
			Expect(releasePipelineRun.Spec.Timeouts.Pipeline.Duration.String()).To(Equal(timeout))
		})

		It("can add a workspace to the PipelineRun using the given name and PVC", func() {
			releasePipelineRun.WithWorkspace(workspace, persistentVolumeClaim)
			Expect(releasePipelineRun.Spec.Workspaces).Should(ContainElement(HaveField("Name", Equal(workspace))))
			Expect(releasePipelineRun.Spec.Workspaces).Should(ContainElement(HaveField("PersistentVolumeClaim.ClaimName", Equal(persistentVolumeClaim))))
		})

		It("can add the EC task bundle parameter to the PipelineRun", func() {
			releasePipelineRun.WithEnterpriseContractConfigMap(enterpriseContractConfigMap)
			Expect(releasePipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string("test-bundle")))))
		})

		It("can add an EnterpriseContractPolicy to the PipelineRun", func() {
			releasePipelineRun.WithEnterpriseContractPolicy(enterpriseContractPolicy)
			jsonSpec, _ := json.Marshal(enterpriseContractPolicy.Spec)
			Expect(releasePipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(jsonSpec)))))
		})
	})

	When("calling WithObjectReferences is called", func() {
		It("does nothing if no objects are passed", func() {
			releasePipelineRun.WithObjectReferences()
			Expect(releasePipelineRun.Spec.Params).To(HaveLen(0))
		})

		It("adds a new parameter to the Pipeline with a reference to the object", func() {
			releasePipelineRun.WithObjectReferences(release)

			Expect(releasePipelineRun.Spec.Params).To(HaveLen(1))
			Expect(releasePipelineRun.Spec.Params[0].Name).To(Equal(strings.ToLower(release.Kind)))
			Expect(releasePipelineRun.Spec.Params[0].Value.Type).To(Equal(tektonv1.ParamTypeString))
			Expect(releasePipelineRun.Spec.Params[0].Value.StringVal).To(Equal(
				fmt.Sprintf("%s%c%s", release.Namespace, types.Separator, release.Name)))
		})
	})
})
