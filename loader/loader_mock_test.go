package loader

import (
	"errors"
	v1alpha12 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Release Adapter", Ordered, func() {
	var (
		loader ObjectLoader
	)

	BeforeAll(func() {
		loader = NewMockLoader()
	})

	Context("When calling getMockedResourceAndErrorFromContext", func() {
		contextErr := errors.New("error")
		contextResource := &v1alpha1.Release{
			ObjectMeta: v12.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
			},
			Spec: v1alpha1.ReleaseSpec{
				ReleasePlan: "releasePlan",
				Snapshot:    "snapshot",
			},
		}

		It("returns the resource from the context", func() {
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleaseContextKey,
					Resource:   contextResource,
				},
			})
			resource, err := getMockedResourceAndErrorFromContext(mockContext, ReleaseContextKey, contextResource)
			Expect(err).To(BeNil())
			Expect(resource).To(Equal(contextResource))
		})

		It("returns the error from the context", func() {
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleaseContextKey,
					Err:        contextErr,
				},
			})
			resource, err := getMockedResourceAndErrorFromContext(mockContext, ReleaseContextKey, contextResource)
			Expect(err).To(Equal(contextErr))
			Expect(resource).To(BeNil())
		})

		It("returns the resource and the error from the context", func() {
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleaseContextKey,
					Resource:   contextResource,
					Err:        contextErr,
				},
			})
			resource, err := getMockedResourceAndErrorFromContext(mockContext, ReleaseContextKey, contextResource)
			Expect(err).To(Equal(contextErr))
			Expect(resource).To(Equal(contextResource))
		})

		It("should panic when the mocked data is not present", func() {
			Expect(func() {
				_, _ = getMockedResourceAndErrorFromContext(ctx, ReleaseContextKey, contextResource)
			}).To(Panic())
		})
	})

	Context("When calling GetActiveReleasePlanAdmission", func() {
		It("returns the resource and error from the context", func() {
			releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			resource, err := loader.GetActiveReleasePlanAdmission(mockContext, nil, nil)
			Expect(resource).To(Equal(releasePlanAdmission))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetActiveReleasePlanAdmissionFromRelease", func() {
		It("returns the resource and error from the context", func() {
			releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			resource, err := loader.GetActiveReleasePlanAdmissionFromRelease(mockContext, nil, nil)
			Expect(resource).To(Equal(releasePlanAdmission))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetApplication", func() {
		It("returns the resource and error from the context", func() {
			application := &applicationapiv1alpha1.Application{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ApplicationContextKey,
					Resource:   application,
				},
			})
			resource, err := loader.GetApplication(mockContext, nil, nil)
			Expect(resource).To(Equal(application))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetApplicationComponents", func() {
		It("returns the resource and error from the context", func() {
			var components []applicationapiv1alpha1.Component
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ApplicationComponentsContextKey,
					Resource:   components,
				},
			})
			resource, err := loader.GetApplicationComponents(mockContext, nil, &applicationapiv1alpha1.Application{})
			Expect(resource).To(Equal(components))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetEnterpriseContractPolicy", func() {
		It("returns the resource and error from the context", func() {
			enterpriseContractPolicy := &v1alpha12.EnterpriseContractPolicy{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: EnterpriseContractPolicyContextKey,
					Resource:   enterpriseContractPolicy,
				},
			})
			resource, err := loader.GetEnterpriseContractPolicy(mockContext, nil, nil)
			Expect(resource).To(Equal(enterpriseContractPolicy))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetEnvironment", func() {
		It("returns the resource and error from the context", func() {
			environment := &applicationapiv1alpha1.Environment{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: EnvironmentContextKey,
					Resource:   environment,
				},
			})
			resource, err := loader.GetEnvironment(mockContext, nil, nil)
			Expect(resource).To(Equal(environment))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetRelease", func() {
		It("returns the resource and error from the context", func() {
			release := &v1alpha1.Release{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleaseContextKey,
					Resource:   release,
				},
			})
			resource, err := loader.GetRelease(mockContext, nil, "", "")
			Expect(resource).To(Equal(release))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetReleasePipelineRun", func() {
		It("returns the resource and error from the context", func() {
			pipelineRun := &v1beta1.PipelineRun{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleasePipelineRunContextKey,
					Resource:   pipelineRun,
				},
			})
			resource, err := loader.GetReleasePipelineRun(mockContext, nil, nil)
			Expect(resource).To(Equal(pipelineRun))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetReleasePlan", func() {
		It("returns the resource and error from the context", func() {
			releasePlan := &v1alpha1.ReleasePlan{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleasePlanContextKey,
					Resource:   releasePlan,
				},
			})
			resource, err := loader.GetReleasePlan(mockContext, nil, nil)
			Expect(resource).To(Equal(releasePlan))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetReleaseStrategy", func() {
		It("returns the resource and error from the context", func() {
			releaseStrategy := &v1alpha1.ReleaseStrategy{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: ReleaseStrategyContextKey,
					Resource:   releaseStrategy,
				},
			})
			resource, err := loader.GetReleaseStrategy(mockContext, nil, nil)
			Expect(resource).To(Equal(releaseStrategy))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetSnapshot", func() {
		It("returns the resource and error from the context", func() {
			snapshot := &applicationapiv1alpha1.Snapshot{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: SnapshotContextKey,
					Resource:   snapshot,
				},
			})
			resource, err := loader.GetSnapshot(mockContext, nil, nil)
			Expect(resource).To(Equal(snapshot))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetSnapshotEnvironmentBinding", func() {
		It("returns the resource and error from the context", func() {
			snapshotEnvironmentBinding := &applicationapiv1alpha1.SnapshotEnvironmentBinding{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: SnapshotEnvironmentBindingContextKey,
					Resource:   snapshotEnvironmentBinding,
				},
			})
			resource, err := loader.GetSnapshotEnvironmentBinding(mockContext, nil, nil)
			Expect(resource).To(Equal(snapshotEnvironmentBinding))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetSnapshotEnvironmentBindingFromReleaseStatus", func() {
		It("returns the resource and error from the context", func() {
			snapshotEnvironmentBinding := &applicationapiv1alpha1.SnapshotEnvironmentBinding{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: SnapshotEnvironmentBindingContextKey,
					Resource:   snapshotEnvironmentBinding,
				},
			})
			resource, err := loader.GetSnapshotEnvironmentBindingFromReleaseStatus(mockContext, nil, nil)
			Expect(resource).To(Equal(snapshotEnvironmentBinding))
			Expect(err).To(BeNil())
		})
	})

	Context("When calling GetSnapshotEnvironmentBindingResources", func() {
		It("returns the resource and error from the context", func() {
			snapshotEnvironmentBindingResources := &SnapshotEnvironmentBindingResources{}
			mockContext := GetMockedContext(ctx, []MockData{
				{
					ContextKey: SnapshotEnvironmentBindingResourcesContextKey,
					Resource:   snapshotEnvironmentBindingResources,
				},
			})
			resource, err := loader.GetSnapshotEnvironmentBindingResources(mockContext, nil, nil, nil)
			Expect(resource).To(Equal(snapshotEnvironmentBindingResources))
			Expect(err).To(BeNil())
		})
	})

})
