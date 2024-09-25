package loader

import (
	"context"

	toolkit "github.com/konflux-ci/operator-toolkit/loader"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ApplicationComponentsContextKey toolkit.ContextKey = iota
	ApplicationContextKey
	EnterpriseContractConfigMapContextKey
	EnterpriseContractPolicyContextKey
	MatchedReleasePlansContextKey
	MatchedReleasePlanAdmissionContextKey
	PreviousReleaseContextKey
	ProcessingResourcesContextKey
	ReleaseContextKey
	ReleasePipelineRunContextKey
	ReleasePlanAdmissionContextKey
	ReleasePlanContextKey
	ReleaseServiceConfigContextKey
	RoleBindingContextKey
	SnapshotContextKey
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
func (l *mockLoader) GetEnterpriseContractPolicy(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*ecapiv1alpha1.EnterpriseContractPolicy, error) {
	if ctx.Value(EnterpriseContractPolicyContextKey) == nil {
		return l.loader.GetEnterpriseContractPolicy(ctx, cli, releasePlanAdmission)
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

// GetMatchingReleasePlanAdmission returns the resource and error passed as values of the context.
func (l *mockLoader) GetMatchingReleasePlanAdmission(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*v1alpha1.ReleasePlanAdmission, error) {
	if ctx.Value(MatchedReleasePlanAdmissionContextKey) == nil {
		return l.loader.GetMatchingReleasePlanAdmission(ctx, cli, releasePlan)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, MatchedReleasePlanAdmissionContextKey, &v1alpha1.ReleasePlanAdmission{})
}

// GetMatchingReleasePlans returns the resource and error passed as values of the context.
func (l *mockLoader) GetMatchingReleasePlans(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*v1alpha1.ReleasePlanList, error) {
	if ctx.Value(MatchedReleasePlansContextKey) == nil {
		return l.loader.GetMatchingReleasePlans(ctx, cli, releasePlanAdmission)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, MatchedReleasePlansContextKey, &v1alpha1.ReleasePlanList{})
}

// GetPreviousRelease returns the resource and error passed as values of the context.
func (l *mockLoader) GetPreviousRelease(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.Release, error) {
	if ctx.Value(PreviousReleaseContextKey) == nil {
		return l.loader.GetPreviousRelease(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, PreviousReleaseContextKey, &v1alpha1.Release{})
}

// GetRelease returns the resource and error passed as values of the context.
func (l *mockLoader) GetRelease(ctx context.Context, cli client.Client, name, namespace string) (*v1alpha1.Release, error) {
	if ctx.Value(ReleaseContextKey) == nil {
		return l.loader.GetRelease(ctx, cli, name, namespace)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleaseContextKey, &v1alpha1.Release{})
}

// GetRoleBindingFromReleaseStatus returns the resource and error passed as values of the context.
func (l *mockLoader) GetRoleBindingFromReleaseStatus(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*rbac.RoleBinding, error) {
	if ctx.Value(RoleBindingContextKey) == nil {
		return l.loader.GetRoleBindingFromReleaseStatus(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, RoleBindingContextKey, &rbac.RoleBinding{})
}

// GetReleasePipelineRun returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleasePipelineRun(ctx context.Context, cli client.Client, release *v1alpha1.Release, pipelineType string) (*tektonv1.PipelineRun, error) {
	if ctx.Value(ReleasePipelineRunContextKey) == nil {
		return l.loader.GetReleasePipelineRun(ctx, cli, release, pipelineType)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleasePipelineRunContextKey, &tektonv1.PipelineRun{})
}

// GetReleasePlan returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleasePlan(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlan, error) {
	if ctx.Value(ReleasePlanContextKey) == nil {
		return l.loader.GetReleasePlan(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleasePlanContextKey, &v1alpha1.ReleasePlan{})
}

// GetReleaseServiceConfig returns the resource and error passed as values of the context.
func (l *mockLoader) GetReleaseServiceConfig(ctx context.Context, cli client.Client, name, namespace string) (*v1alpha1.ReleaseServiceConfig, error) {
	if ctx.Value(ReleaseServiceConfigContextKey) == nil {
		return l.loader.GetReleaseServiceConfig(ctx, cli, name, namespace)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ReleaseServiceConfigContextKey, &v1alpha1.ReleaseServiceConfig{})
}

// GetSnapshot returns the resource and error passed as values of the context.
func (l *mockLoader) GetSnapshot(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*applicationapiv1alpha1.Snapshot, error) {
	if ctx.Value(SnapshotContextKey) == nil {
		return l.loader.GetSnapshot(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, SnapshotContextKey, &applicationapiv1alpha1.Snapshot{})
}

// Composite functions

// GetProcessingResources returns the resource and error passed as values of the context.
func (l *mockLoader) GetProcessingResources(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*ProcessingResources, error) {
	if ctx.Value(ProcessingResourcesContextKey) == nil {
		return l.loader.GetProcessingResources(ctx, cli, release)
	}
	return toolkit.GetMockedResourceAndErrorFromContext(ctx, ProcessingResourcesContextKey, &ProcessingResources{})
}
