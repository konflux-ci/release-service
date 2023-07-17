package loader

import (
	"context"
	toolkit "github.com/redhat-appstudio/operator-toolkit/loader"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ApplicationComponentsContextKey       toolkit.ContextKey = iota
	ApplicationContextKey                 toolkit.ContextKey = iota
	DeploymentResourcesContextKey         toolkit.ContextKey = iota
	EnterpriseContractConfigMapContextKey toolkit.ContextKey = iota
	EnterpriseContractPolicyContextKey    toolkit.ContextKey = iota
	EnvironmentContextKey                 toolkit.ContextKey = iota
	ProcessingResourcesContextKey         toolkit.ContextKey = iota
	ReleaseContextKey                     toolkit.ContextKey = iota
	ReleasePipelineRunContextKey          toolkit.ContextKey = iota
	ReleasePlanAdmissionContextKey        toolkit.ContextKey = iota
	ReleasePlanContextKey                 toolkit.ContextKey = iota
	ReleaseStrategyContextKey             toolkit.ContextKey = iota
	SnapshotContextKey                    toolkit.ContextKey = iota
	SnapshotEnvironmentBindingContextKey  toolkit.ContextKey = iota
)

type mockLoader struct {
	loader ObjectLoader
}

func NewMockLoader() ObjectLoader {
	return &mockLoader{
		loader: NewLoader(),
	}
}

// GetActiveReleasePlanAdmission returns the resource and error passed as values of the context.
func (l *mockLoader) GetActiveReleasePlanAdmission(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*v1alpha1.ReleasePlanAdmission, error) {
	if ctx.Value(ReleasePlanAdmissionContextKey) == nil {
		return l.loader.GetActiveReleasePlanAdmission(ctx, cli, releasePlan)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleasePlanAdmissionContextKey, &v1alpha1.ReleasePlanAdmission{})
}

// GetActiveReleasePlanAdmissionFromRelease returns the resource and error passed as values of the context.
func (l *mockLoader) GetActiveReleasePlanAdmissionFromRelease(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlanAdmission, error) {
	if ctx.Value(ReleasePlanAdmissionContextKey) == nil {
		return l.loader.GetActiveReleasePlanAdmissionFromRelease(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleasePlanAdmissionContextKey, &v1alpha1.ReleasePlanAdmission{})
}

// GetApplication returns the resource and error passed as values of the context.
func (l *mockLoader) GetApplication(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*applicationapiv1alpha1.Application, error) {
	if ctx.Value(ApplicationContextKey) == nil {
		return l.loader.GetApplication(ctx, cli, releasePlan)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ApplicationContextKey, &applicationapiv1alpha1.Application{})
}

// GetEnterpriseContractPolicy returns the resource and error passed as values of the context.
func (l *mockLoader) GetEnterpriseContractPolicy(ctx context.Context, cli client.Client, releaseStrategy *v1alpha1.ReleaseStrategy) (*ecapiv1alpha1.EnterpriseContractPolicy, error) {
	if ctx.Value(EnterpriseContractPolicyContextKey) == nil {
		return l.loader.GetEnterpriseContractPolicy(ctx, cli, releaseStrategy)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, EnterpriseContractPolicyContextKey, &ecapiv1alpha1.EnterpriseContractPolicy{})
}

// GetEnterpriseContractConfigMap returns the resource and error passed as values of the context.
func (l *mockLoader) GetEnterpriseContractConfigMap(ctx context.Context, cli client.Client) (*corev1.ConfigMap, error) {
	if ctx.Value(EnterpriseContractConfigMapContextKey) == nil {
		return l.loader.GetEnterpriseContractConfigMap(ctx, cli)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, EnterpriseContractConfigMapContextKey, &corev1.ConfigMap{})
}

// GetEnvironment returns the resource and error passed as values of the context.
func (l *mockLoader) GetEnvironment(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.Environment, error) {
	if ctx.Value(EnvironmentContextKey) == nil {
		return l.loader.GetEnvironment(ctx, cli, releasePlanAdmission)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, EnvironmentContextKey, &applicationapiv1alpha1.Environment{})
}

// GetManagedApplication returns the resource and error passed as values of the context.
func (l *mockLoader) GetManagedApplication(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.Application, error) {
	if ctx.Value(ApplicationContextKey) == nil {
		return l.loader.GetManagedApplication(ctx, cli, releasePlanAdmission)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ApplicationContextKey, &applicationapiv1alpha1.Application{})
}

// GetManagedApplicationComponents returns the resource and error passed as values of the context.
func (l *mockLoader) GetManagedApplicationComponents(ctx context.Context, cli client.Client, application *applicationapiv1alpha1.Application) ([]applicationapiv1alpha1.Component, error) {
	if ctx.Value(ApplicationComponentsContextKey) == nil {
		return l.loader.GetManagedApplicationComponents(ctx, cli, application)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ApplicationComponentsContextKey, []applicationapiv1alpha1.Component{})
}

// GetRelease returns the resource and error passed as values of the context.
func (l *mockLoader) GetRelease(ctx context.Context, cli client.Client, name, namespace string) (*v1alpha1.Release, error) {
	if ctx.Value(ReleaseContextKey) == nil {
		return l.loader.GetRelease(ctx, cli, name, namespace)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleaseContextKey, &v1alpha1.Release{})
}

// GetReleasePipelineRun returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleasePipelineRun(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1beta1.PipelineRun, error) {
	if ctx.Value(ReleasePipelineRunContextKey) == nil {
		return l.loader.GetReleasePipelineRun(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleasePipelineRunContextKey, &v1beta1.PipelineRun{})
}

// GetReleasePlan returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleasePlan(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlan, error) {
	if ctx.Value(ReleasePlanContextKey) == nil {
		return l.loader.GetReleasePlan(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleasePlanContextKey, &v1alpha1.ReleasePlan{})
}

// GetReleaseStrategy returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleaseStrategy(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*v1alpha1.ReleaseStrategy, error) {
	if ctx.Value(ReleaseStrategyContextKey) == nil {
		return l.loader.GetReleaseStrategy(ctx, cli, releasePlanAdmission)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleaseStrategyContextKey, &v1alpha1.ReleaseStrategy{})
}

// GetSnapshot returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshot(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*applicationapiv1alpha1.Snapshot, error) {
	if ctx.Value(SnapshotContextKey) == nil {
		return l.loader.GetSnapshot(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, SnapshotContextKey, &applicationapiv1alpha1.Snapshot{})
}

// GetSnapshotEnvironmentBinding returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshotEnvironmentBinding(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	if ctx.Value(SnapshotEnvironmentBindingContextKey) == nil {
		return l.loader.GetSnapshotEnvironmentBinding(ctx, cli, releasePlanAdmission)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, SnapshotEnvironmentBindingContextKey, &applicationapiv1alpha1.SnapshotEnvironmentBinding{})
}

// GetSnapshotEnvironmentBindingFromReleaseStatus returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshotEnvironmentBindingFromReleaseStatus(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	if ctx.Value(SnapshotEnvironmentBindingContextKey) == nil {
		return l.loader.GetSnapshotEnvironmentBindingFromReleaseStatus(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, SnapshotEnvironmentBindingContextKey, &applicationapiv1alpha1.SnapshotEnvironmentBinding{})
}

// Composite functions

// GetDeploymentResources returns the resource and error passed as values of the context.
func (l *mockLoader) GetDeploymentResources(ctx context.Context, cli client.Client, release *v1alpha1.Release, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*DeploymentResources, error) {
	if ctx.Value(DeploymentResourcesContextKey) == nil {
		return l.loader.GetDeploymentResources(ctx, cli, release, releasePlanAdmission)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, DeploymentResourcesContextKey, &DeploymentResources{})
}

// GetProcessingResources returns the resource and error passed as values of the context.
func (l *mockLoader) GetProcessingResources(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*ProcessingResources, error) {
	if ctx.Value(ProcessingResourcesContextKey) == nil {
		return l.loader.GetProcessingResources(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ProcessingResourcesContextKey, &ProcessingResources{})
}
