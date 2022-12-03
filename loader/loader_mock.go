package loader

import (
	"context"
	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	contextKey int
	mockData   struct {
		Err      error
		Resource any
	}
	mockLoader struct{}
)

const contextDataKey contextKey = iota

func GetMockedContext(ctx context.Context, resource any, err error) context.Context {
	return context.WithValue(ctx, contextDataKey, mockData{
		Err:      err,
		Resource: resource,
	})
}

func NewMockLoader() ObjectLoader {
	return &mockLoader{}
}

// getMockedResourceAndErrorFromContext returns the mocked data found in the context passed as an argument. The data is
// to be found in the contextDataKey key. If not there, a panic will be raised.
func getMockedResourceAndErrorFromContext[T any](ctx context.Context, _ T) (T, error) {
	var resource T
	var err error

	value := ctx.Value(contextDataKey)
	if value == nil {
		panic("Mocked data not found in the context")
	}

	data, _ := value.(mockData)

	if data.Resource != nil {
		resource = data.Resource.(T)
	}

	if data.Err != nil {
		err = data.Err
	}

	return resource, err
}

// GetActiveReleasePlanAdmission returns the resource and error passed as values of the context.
func (l *mockLoader) GetActiveReleasePlanAdmission(ctx context.Context, _ client.Client, _ *v1alpha1.ReleasePlan) (*v1alpha1.ReleasePlanAdmission, error) {
	return getMockedResourceAndErrorFromContext(ctx, &v1alpha1.ReleasePlanAdmission{})
}

// GetActiveReleasePlanAdmissionFromRelease returns the resource and error passed as values of the context.
func (l *mockLoader) GetActiveReleasePlanAdmissionFromRelease(ctx context.Context, _ client.Client, _ *v1alpha1.Release) (*v1alpha1.ReleasePlanAdmission, error) {
	return getMockedResourceAndErrorFromContext(ctx, &v1alpha1.ReleasePlanAdmission{})
}

// GetApplication returns the resource and error passed as values of the context.
func (l *mockLoader) GetApplication(ctx context.Context, _ client.Client, _ *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.Application, error) {
	return getMockedResourceAndErrorFromContext(ctx, &applicationapiv1alpha1.Application{})
}

// GetApplicationComponents returns the resource and error passed as values of the context.
func (l *mockLoader) GetApplicationComponents(ctx context.Context, _ client.Client, _ *applicationapiv1alpha1.Application) ([]applicationapiv1alpha1.Component, error) {
	return getMockedResourceAndErrorFromContext(ctx, []applicationapiv1alpha1.Component{})
}

// GetEnterpriseContractPolicy returns the resource and error passed as values of the context.
func (l *mockLoader) GetEnterpriseContractPolicy(ctx context.Context, _ client.Client, _ *v1alpha1.ReleaseStrategy) (*ecapiv1alpha1.EnterpriseContractPolicy, error) {
	return getMockedResourceAndErrorFromContext(ctx, &ecapiv1alpha1.EnterpriseContractPolicy{})
}

// GetEnvironment returns the resource and error passed as values of the context.
func (l *mockLoader) GetEnvironment(ctx context.Context, _ client.Client, _ *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.Environment, error) {
	return getMockedResourceAndErrorFromContext(ctx, &applicationapiv1alpha1.Environment{})
}

// GetRelease returns the resource and error passed as values of the context.
func (l *mockLoader) GetRelease(ctx context.Context, _ client.Client, _, _ string) (*v1alpha1.Release, error) {
	return getMockedResourceAndErrorFromContext(ctx, &v1alpha1.Release{})
}

// GetReleasePipelineRun returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleasePipelineRun(ctx context.Context, _ client.Client, _ *v1alpha1.Release) (*v1beta1.PipelineRun, error) {
	return getMockedResourceAndErrorFromContext(ctx, &v1beta1.PipelineRun{})
}

// GetReleasePlan returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleasePlan(ctx context.Context, _ client.Client, _ *v1alpha1.Release) (*v1alpha1.ReleasePlan, error) {
	return getMockedResourceAndErrorFromContext(ctx, &v1alpha1.ReleasePlan{})
}

// GetReleaseStrategy returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleaseStrategy(ctx context.Context, _ client.Client, _ *v1alpha1.ReleasePlanAdmission) (*v1alpha1.ReleaseStrategy, error) {
	return getMockedResourceAndErrorFromContext(ctx, &v1alpha1.ReleaseStrategy{})
}

// GetSnapshot returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshot(ctx context.Context, _ client.Client, _ *v1alpha1.Release) (*applicationapiv1alpha1.Snapshot, error) {
	return getMockedResourceAndErrorFromContext(ctx, &applicationapiv1alpha1.Snapshot{})
}

// GetSnapshotEnvironmentBinding returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshotEnvironmentBinding(ctx context.Context, _ client.Client, _ *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	return getMockedResourceAndErrorFromContext(ctx, &applicationapiv1alpha1.SnapshotEnvironmentBinding{})
}

// GetSnapshotEnvironmentBindingFromReleaseStatus returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshotEnvironmentBindingFromReleaseStatus(ctx context.Context, _ client.Client, _ *v1alpha1.Release) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	return getMockedResourceAndErrorFromContext(ctx, &applicationapiv1alpha1.SnapshotEnvironmentBinding{})
}

// GetSnapshotEnvironmentBindingResources returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshotEnvironmentBindingResources(ctx context.Context, _ client.Client, _ *v1alpha1.Release, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*SnapshotEnvironmentBindingResources, error) {
	return getMockedResourceAndErrorFromContext(ctx, &SnapshotEnvironmentBindingResources{})
}
