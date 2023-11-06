package loader

import (
	toolkit "github.com/redhat-appstudio/operator-toolkit/loader"

	v1alpha12 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

var _ = Describe("Release Adapter", Ordered, func() {
	var (
		loader ObjectLoader
	)

	BeforeAll(func() {
		loader = NewMockLoader()
	})

	When("calling GetActiveReleasePlanAdmission", func() {
		It("returns the resource and error from the context", func() {
			releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetActiveReleasePlanAdmissionFromRelease", func() {
		It("returns the resource and error from the context", func() {
			releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetApplication", func() {
		It("returns the resource and error from the context", func() {
			application := &applicationapiv1alpha1.Application{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetEnterpriseContractPolicy", func() {
		It("returns the resource and error from the context", func() {
			enterpriseContractPolicy := &v1alpha12.EnterpriseContractPolicy{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetEnvironment", func() {
		It("returns the resource and error from the context", func() {
			environment := &applicationapiv1alpha1.Environment{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetManagedApplication", func() {
		It("returns the resource and error from the context", func() {
			application := &applicationapiv1alpha1.Application{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: ApplicationContextKey,
					Resource:   application,
				},
			})
			resource, err := loader.GetManagedApplication(mockContext, nil, nil)
			Expect(resource).To(Equal(application))
			Expect(err).To(BeNil())
		})
	})

	When("calling GetManagedApplicationComponents", func() {
		It("returns the resource and error from the context", func() {
			var components []applicationapiv1alpha1.Component
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: ApplicationComponentsContextKey,
					Resource:   components,
				},
			})
			resource, err := loader.GetManagedApplicationComponents(mockContext, nil, &applicationapiv1alpha1.Application{})
			Expect(resource).To(Equal(components))
			Expect(err).To(BeNil())
		})
	})

	When("calling GetMatchingReleasePlanAdmission", func() {
		It("returns the resource and error from the context", func() {
			releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: MatchedReleasePlanAdmissionContextKey,
					Resource:   releasePlanAdmission,
				},
			})
			resource, err := loader.GetMatchingReleasePlanAdmission(mockContext, nil, nil)
			Expect(resource).To(Equal(releasePlanAdmission))
			Expect(err).To(BeNil())
		})
	})

	When("calling GetMatchingReleasePlans", func() {
		It("returns the resource and error from the context", func() {
			releasePlans := &v1alpha1.ReleasePlanList{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: MatchedReleasePlansContextKey,
					Resource:   releasePlans,
				},
			})
			resource, err := loader.GetMatchingReleasePlans(mockContext, nil, nil)
			Expect(resource).To(Equal(releasePlans))
			Expect(err).To(BeNil())
		})
	})

	When("calling GetRelease", func() {
		It("returns the resource and error from the context", func() {
			release := &v1alpha1.Release{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetReleasePipelineRun", func() {
		It("returns the resource and error from the context", func() {
			pipelineRun := &tektonv1.PipelineRun{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetReleasePlan", func() {
		It("returns the resource and error from the context", func() {
			releasePlan := &v1alpha1.ReleasePlan{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetReleaseServiceConfig", func() {
		It("returns the resource and error from the context", func() {
			releaseServiceConfig := &v1alpha1.ReleaseServiceConfig{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: ReleaseServiceConfigContextKey,
					Resource:   releaseServiceConfig,
				},
			})
			resource, err := loader.GetReleaseServiceConfig(mockContext, nil, "", "")
			Expect(resource).To(Equal(releaseServiceConfig))
			Expect(err).To(BeNil())
		})
	})

	When("calling GetSnapshot", func() {
		It("returns the resource and error from the context", func() {
			snapshot := &applicationapiv1alpha1.Snapshot{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetSnapshotEnvironmentBinding", func() {
		It("returns the resource and error from the context", func() {
			snapshotEnvironmentBinding := &applicationapiv1alpha1.SnapshotEnvironmentBinding{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	When("calling GetSnapshotEnvironmentBindingFromReleaseStatus", func() {
		It("returns the resource and error from the context", func() {
			snapshotEnvironmentBinding := &applicationapiv1alpha1.SnapshotEnvironmentBinding{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
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

	// Composite functions

	When("calling GetDeploymentResources", func() {
		It("returns the resource and error from the context", func() {
			deploymentResources := &DeploymentResources{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: DeploymentResourcesContextKey,
					Resource:   deploymentResources,
				},
			})
			resource, err := loader.GetDeploymentResources(mockContext, nil, nil, nil)
			Expect(resource).To(Equal(deploymentResources))
			Expect(err).To(BeNil())
		})
	})

	When("calling GetProcessingResources", func() {
		It("returns the resource and error from the context", func() {
			processingResources := &ProcessingResources{}
			mockContext := toolkit.GetMockedContext(ctx, []toolkit.MockData{
				{
					ContextKey: ProcessingResourcesContextKey,
					Resource:   processingResources,
				},
			})
			resource, err := loader.GetProcessingResources(mockContext, nil, nil)
			Expect(resource).To(Equal(processingResources))
			Expect(err).To(BeNil())
		})
	})

})
