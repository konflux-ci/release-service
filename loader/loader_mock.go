package loader

import (
	"context"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	contextKey int
	MockData   struct {
		ContextKey contextKey
		Err        error
		Resource   any
	}
	mockLoader struct {
		loader ObjectLoader
	}
)

const (
	ApplicationComponentsContextKey      contextKey = iota
	ApplicationContextKey                contextKey = iota
	DeploymentResourcesContextKey        contextKey = iota
	EnterpriseContractPolicyContextKey   contextKey = iota
	EnvironmentContextKey                contextKey = iota
	ProcessingResourcesContextKey        contextKey = iota
	ReleaseContextKey                    contextKey = iota
	ReleasePipelineRunContextKey         contextKey = iota
	ReleasePlanAdmissionContextKey       contextKey = iota
	ReleasePlanContextKey                contextKey = iota
	ReleaseStrategyContextKey            contextKey = iota
	SnapshotContextKey                   contextKey = iota
	SnapshotEnvironmentBindingContextKey contextKey = iota
)

func GetMockedContext(ctx context.Context, data []MockData) context.Context {
	for _, mockData := range data {
		ctx = context.WithValue(ctx, mockData.ContextKey, mockData)
	}

	return ctx
}

func NewMockLoader() ObjectLoader {
	return &mockLoader{
		loader: NewLoader(),
	}
}

// getMockedResourceAndErrorFromContext returns the mocked data found in the context passed as an argument. The data is
// to be found in the contextDataKey key. If not there, a panic will be raised.
func getMockedResourceAndErrorFromContext[T any](ctx context.Context, contextKey contextKey, _ T) (T, error) {
	var resource T
	var err error

	value := ctx.Value(contextKey)
	if value == nil {
		panic("Mocked data not found in the context")
	}

	data, _ := value.(MockData)

	if data.Resource != nil {
		resource = data.Resource.(T)
	}

	if data.Err != nil {
		err = data.Err
	}

	return resource, err
}

// GetActiveReleasePlanAdmission returns the resource and error passed as values of the context.
func (l *mockLoader) GetActiveReleasePlanAdmission(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*v1alpha1.ReleasePlanAdmission, error) {
	if ctx.Value(ReleasePlanAdmissionContextKey) == nil {
		return l.loader.GetActiveReleasePlanAdmission(ctx, cli, releasePlan)
	}
	return getMockedResourceAndErrorFromContext(ctx, ReleasePlanAdmissionContextKey, &v1alpha1.ReleasePlanAdmission{})
}

// GetActiveReleasePlanAdmissionFromRelease returns the resource and error passed as values of the context.
func (l *mockLoader) GetActiveReleasePlanAdmissionFromRelease(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlanAdmission, error) {
	if ctx.Value(ReleasePlanAdmissionContextKey) == nil {
		return l.loader.GetActiveReleasePlanAdmissionFromRelease(ctx, cli, release)
	}
	return getMockedResourceAndErrorFromContext(ctx, ReleasePlanAdmissionContextKey, &v1alpha1.ReleasePlanAdmission{})
}

// GetApplication returns the resource and error passed as values of the context.
func (l *mockLoader) GetApplication(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.Application, error) {
	if ctx.Value(ApplicationContextKey) == nil {
		return l.loader.GetApplication(ctx, cli, releasePlanAdmission)
	}
	return getMockedResourceAndErrorFromContext(ctx, ApplicationContextKey, &applicationapiv1alpha1.Application{})
}

// GetApplicationComponents returns the resource and error passed as values of the context.
func (l *mockLoader) GetApplicationComponents(ctx context.Context, cli client.Client, application *applicationapiv1alpha1.Application) ([]applicationapiv1alpha1.Component, error) {
	if ctx.Value(ApplicationComponentsContextKey) == nil {
		return l.loader.GetApplicationComponents(ctx, cli, application)
	}
	return getMockedResourceAndErrorFromContext(ctx, ApplicationComponentsContextKey, []applicationapiv1alpha1.Component{})
}

// GetEnterpriseContractPolicy returns the resource and error passed as values of the context.
func (l *mockLoader) GetEnterpriseContractPolicy(ctx context.Context, cli client.Client, releaseStrategy *v1alpha1.ReleaseStrategy) (*ecapiv1alpha1.EnterpriseContractPolicy, error) {
	if ctx.Value(EnterpriseContractPolicyContextKey) == nil {
		return l.loader.GetEnterpriseContractPolicy(ctx, cli, releaseStrategy)
	}
	return getMockedResourceAndErrorFromContext(ctx, EnterpriseContractPolicyContextKey, &ecapiv1alpha1.EnterpriseContractPolicy{})
}

// GetEnvironment returns the resource and error passed as values of the context.
func (l *mockLoader) GetEnvironment(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.Environment, error) {
	if ctx.Value(EnvironmentContextKey) == nil {
		return l.loader.GetEnvironment(ctx, cli, releasePlanAdmission)
	}
	return getMockedResourceAndErrorFromContext(ctx, EnvironmentContextKey, &applicationapiv1alpha1.Environment{})
}

// GetRelease returns the resource and error passed as values of the context.
func (l *mockLoader) GetRelease(ctx context.Context, cli client.Client, name, namespace string) (*v1alpha1.Release, error) {
	if ctx.Value(ReleaseContextKey) == nil {
		return l.loader.GetRelease(ctx, cli, name, namespace)
	}
	return getMockedResourceAndErrorFromContext(ctx, ReleaseContextKey, &v1alpha1.Release{})
}

// GetReleasePipelineRun returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleasePipelineRun(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1beta1.PipelineRun, error) {
	if ctx.Value(ReleasePipelineRunContextKey) == nil {
		return l.loader.GetReleasePipelineRun(ctx, cli, release)
	}
	return getMockedResourceAndErrorFromContext(ctx, ReleasePipelineRunContextKey, &v1beta1.PipelineRun{})
}

// GetReleasePlan returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleasePlan(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlan, error) {
	if ctx.Value(ReleasePlanContextKey) == nil {
		return l.loader.GetReleasePlan(ctx, cli, release)
	}
	return getMockedResourceAndErrorFromContext(ctx, ReleasePlanContextKey, &v1alpha1.ReleasePlan{})
}

// GetReleaseStrategy returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleaseStrategy(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*v1alpha1.ReleaseStrategy, error) {
	if ctx.Value(ReleaseStrategyContextKey) == nil {
		return l.loader.GetReleaseStrategy(ctx, cli, releasePlanAdmission)
	}
	return getMockedResourceAndErrorFromContext(ctx, ReleaseStrategyContextKey, &v1alpha1.ReleaseStrategy{})
}

// GetSnapshot returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshot(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*applicationapiv1alpha1.Snapshot, error) {
	if ctx.Value(SnapshotContextKey) == nil {
		return l.loader.GetSnapshot(ctx, cli, release)
	}
	return getMockedResourceAndErrorFromContext(ctx, SnapshotContextKey, &applicationapiv1alpha1.Snapshot{})
}

// GetSnapshotEnvironmentBinding returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshotEnvironmentBinding(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	if ctx.Value(SnapshotEnvironmentBindingContextKey) == nil {
		return l.loader.GetSnapshotEnvironmentBinding(ctx, cli, releasePlanAdmission)
	}
	return getMockedResourceAndErrorFromContext(ctx, SnapshotEnvironmentBindingContextKey, &applicationapiv1alpha1.SnapshotEnvironmentBinding{})
}

// GetSnapshotEnvironmentBindingFromReleaseStatus returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshotEnvironmentBindingFromReleaseStatus(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	if ctx.Value(SnapshotEnvironmentBindingContextKey) == nil {
		return l.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(ctx, cli, release)
	}
	return getMockedResourceAndErrorFromContext(ctx, SnapshotEnvironmentBindingContextKey, &applicationapiv1alpha1.SnapshotEnvironmentBinding{})
}

// Composite functions

// GetDeploymentResources returns the resource and error passed as values of the context.
func (l *mockLoader) GetDeploymentResources(ctx context.Context, cli client.Client, release *v1alpha1.Release, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*DeploymentResources, error) {
	if ctx.Value(DeploymentResourcesContextKey) == nil {
		return l.loader.GetDeploymentResources(ctx, cli, release, releasePlanAdmission)
	}
	return getMockedResourceAndErrorFromContext(ctx, DeploymentResourcesContextKey, &DeploymentResources{})
}

// GetProcessingResources returns the resource and error passed as values of the context.
func (l *mockLoader) GetProcessingResources(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*ProcessingResources, error) {
	if ctx.Value(ProcessingResourcesContextKey) == nil {
		return l.loader.GetProcessingResources(ctx, cli, release)
	}
	return getMockedResourceAndErrorFromContext(ctx, ProcessingResourcesContextKey, &ProcessingResources{})
}
