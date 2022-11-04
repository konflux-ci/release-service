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
	"reflect"

	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kcp-dev/logicalcluster/v2"
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
	const (
		pipelineRunPrefixName = "test-pipeline"
		namespace             = "default"
		workspace             = "test-workspace"
		persistentVolumeClaim = "test-pvc"
		serviceAccountName    = "test-service-account"
		apiVersion            = "appstudio.redhat.com/v1alpha1"
		applicationName       = "test-application"
	)
	var (
		release                  *v1alpha1.Release
		extraParams              *ExtraParams
		releasePipelineRun       *ReleasePipelineRun
		snapshot                 *applicationapiv1alpha1.Snapshot
		strategy                 *v1alpha1.ReleaseStrategy
		enterpriseContractPolicy *ecapiv1alpha1.EnterpriseContractPolicy
		unmarshaledSnapshotSpec  *applicationapiv1alpha1.SnapshotSpec
	)
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
				APIVersion: apiVersion,
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "myrelease-",
				Namespace:    namespace,
				Annotations: map[string]string{
					logicalcluster.AnnotationKey: "kcp:test-cluster:dev",
				},
			},
			Spec: v1alpha1.ReleaseSpec{
				Snapshot:    "testsnapshot",
				ReleasePlan: "testreleaseplan",
			},
		}
		snapshot = &applicationapiv1alpha1.Snapshot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "testsnapshot-",
				Namespace:    "default",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiVersion,
				Kind:       "Snapshot",
			},
			Spec: applicationapiv1alpha1.SnapshotSpec{
				Application: applicationName,
				DisplayName: "Test application",
				Components:  []applicationapiv1alpha1.SnapshotComponent{},
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
				Sources: []ecapiv1alpha1.PolicySource{
					ecapiv1alpha1.PolicySource{
						GitRepository: &ecapiv1alpha1.GitPolicySource{
							Repository: "https://github.com/",
							Revision:   "main",
						},
					},
				},
			},
		}
		strategy = &v1alpha1.ReleaseStrategy{
			Spec: v1alpha1.ReleaseStrategySpec{
				Pipeline:              "release-pipeline",
				Bundle:                "testbundle",
				Policy:                "testpolicy",
				PersistentVolumeClaim: persistentVolumeClaim,
				ServiceAccount:        serviceAccountName,
				Params: []v1alpha1.Params{
					v1alpha1.Params{
						Name:   "testparam1",
						Values: []string{"val1", "val2"},
					},
				},
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

	Context("When managing a new ReleasePipelineRun", func() {

		It("can create a ReleasePipelineRun and the returned object name is prefixed with the provided GenerateName", func() {
			Expect(releasePipelineRun.ObjectMeta.Name).
				Should(HavePrefix(pipelineRunPrefixName))
			Expect(releasePipelineRun.ObjectMeta.Namespace).To(Equal(namespace))
		})

		It("can append extra params to ReleasePipelineRun and these parameters are present in the object Specs", func() {
			releasePipelineRun.WithExtraParam(extraParams.Name, extraParams.Value)
			Expect(releasePipelineRun.Spec.Params[0].Name).To(Equal(extraParams.Name))
			Expect(releasePipelineRun.Spec.Params[0].Value.StringVal).
				To(Equal(extraParams.Value.StringVal))
		})

		It("can append owner release information to the object as annotations", func() {
			releasePipelineRun.WithOwner(release)
			Expect(releasePipelineRun.Annotations).NotTo(BeNil())
		})

		It("can append the release Name, Namespace, and Workspace to a ReleasePipelineRun object and that these label key names match the correct label format", func() {
			releasePipelineRun.WithReleaseAndApplicationLabels(
				release.Name,
				release.Namespace,
				release.GetAnnotations()[logicalcluster.AnnotationKey],
				applicationName)
			Expect(releasePipelineRun.Labels["release.appstudio.openshift.io/name"]).
				To(Equal(release.Name))
			Expect(releasePipelineRun.Labels["release.appstudio.openshift.io/namespace"]).
				To(Equal(release.Namespace))
			Expect(releasePipelineRun.Labels["release.appstudio.openshift.io/workspace"]).
				To(Equal("kcp__test-cluster__dev"))
			Expect(releasePipelineRun.Labels["appstudio.openshift.io/application"]).
				To(Equal(applicationName))
		})

		It("can return a PipelineRun object from a ReleasePipelineRun object", func() {
			Expect(reflect.TypeOf(releasePipelineRun.AsPipelineRun())).
				To(Equal(reflect.TypeOf(&tektonv1beta1.PipelineRun{})))
		})

		It("can add an Snapshot object as a json string to the PipelineRun", func() {
			Expect(k8sClient.Create(ctx, snapshot)).Should(Succeed())

			snapshot.TypeMeta.APIVersion = apiVersion
			snapshot.TypeMeta.Kind = "Snapshot"

			releasePipelineRun.WithSnapshot(snapshot)

			Expect(releasePipelineRun.Spec.Params[0].Name).To(Equal("snapshot"))
			Expect(json.Unmarshal(
				[]byte(releasePipelineRun.Spec.Params[0].Value.StringVal),
				&unmarshaledSnapshotSpec)).Should(Succeed())

			// check if the unmarshaled data has what we expect
			Expect(unmarshaledSnapshotSpec.Application).To(Equal(applicationName))
		})

		It("can add the ReleaseStrategy information to a PipelineRun object and ", func() {
			releasePipelineRun.WithReleaseStrategy(strategy)
			Expect(releasePipelineRun.Spec.PipelineRef.Name).
				To(Equal("release-pipeline"))
		})

		It("can add the reference to the service account that should be used", func() {
			releasePipelineRun.WithServiceAccount(serviceAccountName)
			Expect(releasePipelineRun.Spec.ServiceAccountName).To(Equal(serviceAccountName))
		})

		It("can add a workspace to the PipelineRun using the given name and PVC", func() {
			releasePipelineRun.WithWorkspace(workspace, persistentVolumeClaim)
			Expect(releasePipelineRun.Spec.Workspaces).Should(ContainElement(HaveField("Name", Equal(workspace))))
			Expect(releasePipelineRun.Spec.Workspaces).Should(ContainElement(HaveField("PersistentVolumeClaim.ClaimName", Equal(persistentVolumeClaim))))
		})

		It("can add an EnterpriseContractPolicy to the PipelineRun", func() {
			releasePipelineRun.WithEnterpriseContractPolicy(enterpriseContractPolicy)
			jsonSpec, _ := json.Marshal(enterpriseContractPolicy.Spec)
			Expect(releasePipelineRun.Spec.Params).Should(ContainElement(HaveField("Value.StringVal", Equal(string(jsonSpec)))))
		})
	})
})
