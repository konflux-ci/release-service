package loader

import (
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/utils/strings/slices"

	toolkit "github.com/konflux-ci/operator-toolkit/loader"

	ecapiv1alpha1 "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/metadata"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectLoader interface {
	GetActiveReleasePlanAdmission(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*v1alpha1.ReleasePlanAdmission, error)
	GetActiveReleasePlanAdmissionFromRelease(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlanAdmission, error)
	GetApplication(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*applicationapiv1alpha1.Application, error)
	GetEnterpriseContractConfigMap(ctx context.Context, cli client.Client) (*corev1.ConfigMap, error)
	GetEnterpriseContractPolicy(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*ecapiv1alpha1.EnterpriseContractPolicy, error)
	GetMatchingReleasePlanAdmission(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*v1alpha1.ReleasePlanAdmission, error)
	GetMatchingReleasePlans(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*v1alpha1.ReleasePlanList, error)
	GetPreviousRelease(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.Release, error)
	GetRelease(ctx context.Context, cli client.Client, name, namespace string) (*v1alpha1.Release, error)
	GetRoleBindingFromReleaseStatus(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*rbac.RoleBinding, error)
	GetReleasePipelineRun(ctx context.Context, cli client.Client, release *v1alpha1.Release, pipelineType string) (*tektonv1.PipelineRun, error)
	GetReleasePlan(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlan, error)
	GetReleaseServiceConfig(ctx context.Context, cli client.Client, name, namespace string) (*v1alpha1.ReleaseServiceConfig, error)
	GetSnapshot(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*applicationapiv1alpha1.Snapshot, error)
	GetProcessingResources(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*ProcessingResources, error)
}

type loader struct{}

func NewLoader() ObjectLoader {
	return &loader{}
}

// GetActiveReleasePlanAdmission returns the ReleasePlanAdmission targeted by the given ReleasePlan.
// Only ReleasePlanAdmissions with the 'auto-release' label set to true (or missing the label, which is
// treated the same as having the label and it being set to true) will be searched for. If a matching
// ReleasePlanAdmission is not found or the List operation fails, an error will be returned.
func (l *loader) GetActiveReleasePlanAdmission(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*v1alpha1.ReleasePlanAdmission, error) {
	releasePlanAdmission, err := l.GetMatchingReleasePlanAdmission(ctx, cli, releasePlan)
	if err != nil {
		return nil, err
	}

	labelValue, found := releasePlanAdmission.GetLabels()[metadata.AutoReleaseLabel]
	if found && labelValue == "false" {
		return nil, fmt.Errorf("found ReleasePlanAdmission '%s' with auto-release label set to false",
			releasePlanAdmission.Name)
	}

	return releasePlanAdmission, nil
}

// GetActiveReleasePlanAdmissionFromRelease returns the ReleasePlanAdmission targeted by the ReleasePlan referenced by
// the given Release. Only ReleasePlanAdmissions with the 'auto-release' label set to true (or missing the label, which
// is treated the same as having the label and it being set to true) will be searched for. If a matching
// ReleasePlanAdmission is not found or the List operation fails, an error will be returned.
func (l *loader) GetActiveReleasePlanAdmissionFromRelease(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlanAdmission, error) {
	releasePlan, err := l.GetReleasePlan(ctx, cli, release)
	if err != nil {
		return nil, err
	}

	return l.GetActiveReleasePlanAdmission(ctx, cli, releasePlan)
}

// GetApplication returns the Application referenced by the ReleasePlan. If the Application is not found or
// the Get operation fails, an error will be returned.
func (l *loader) GetApplication(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*applicationapiv1alpha1.Application, error) {
	application := &applicationapiv1alpha1.Application{}
	return application, toolkit.GetObject(releasePlan.Spec.Application, releasePlan.Namespace, cli, ctx, application)
}

// GetEnterpriseContractPolicy returns the EnterpriseContractPolicy referenced by the given ReleasePlanAdmission. If the
// EnterpriseContractPolicy is not found or the Get operation fails, an error is returned.
func (l *loader) GetEnterpriseContractPolicy(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*ecapiv1alpha1.EnterpriseContractPolicy, error) {
	enterpriseContractPolicy := &ecapiv1alpha1.EnterpriseContractPolicy{}
	return enterpriseContractPolicy, toolkit.GetObject(releasePlanAdmission.Spec.Policy, releasePlanAdmission.Namespace, cli, ctx, enterpriseContractPolicy)
}

// GetEnterpriseContractConfigMap returns the defaults ConfigMap in the Enterprise Contract namespace . If the ENTERPRISE_CONTRACT_CONFIG_MAP
// value is invalid or not set, nil is returned. If the ConfigMap is not found or the Get operation fails, an error is returned.
func (l *loader) GetEnterpriseContractConfigMap(ctx context.Context, cli client.Client) (*corev1.ConfigMap, error) {
	enterpriseContractConfigMap := &corev1.ConfigMap{}
	namespacedName := os.Getenv("ENTERPRISE_CONTRACT_CONFIG_MAP")

	if index := strings.IndexByte(namespacedName, '/'); index >= 0 {
		return enterpriseContractConfigMap, toolkit.GetObject(namespacedName[index+1:], namespacedName[:index],
			cli, ctx, enterpriseContractConfigMap)
	}

	return nil, nil

}

// GetMatchingReleasePlanAdmission returns the ReleasePlanAdmission targeted by the given ReleasePlan.
// If a matching ReleasePlanAdmission is not found or the List operation fails, an error will be returned.
// If more than one matching ReleasePlanAdmission objects are found, an error will be returned.
func (l *loader) GetMatchingReleasePlanAdmission(ctx context.Context, cli client.Client, releasePlan *v1alpha1.ReleasePlan) (*v1alpha1.ReleasePlanAdmission, error) {
	designatedReleasePlanAdmissionName := releasePlan.GetLabels()[metadata.ReleasePlanAdmissionLabel]

	if designatedReleasePlanAdmissionName != "" {
		releasePlanAdmission := &v1alpha1.ReleasePlanAdmission{}
		return releasePlanAdmission, toolkit.GetObject(designatedReleasePlanAdmissionName, releasePlan.Spec.Target, cli, ctx, releasePlanAdmission)
	}

	if releasePlan.Spec.Target == "" {
		return nil, fmt.Errorf("releasePlan has no target, so no ReleasePlanAdmissions can be found")
	}

	releasePlanAdmissions := &v1alpha1.ReleasePlanAdmissionList{}
	err := cli.List(ctx, releasePlanAdmissions,
		client.InNamespace(releasePlan.Spec.Target),
		client.MatchingFields{"spec.origin": releasePlan.Namespace})
	if err != nil {
		return nil, err
	}

	var foundReleasePlanAdmission *v1alpha1.ReleasePlanAdmission

	for i, releasePlanAdmission := range releasePlanAdmissions.Items {
		if !slices.Contains(releasePlanAdmission.Spec.Applications, releasePlan.Spec.Application) {
			continue
		}

		if foundReleasePlanAdmission != nil {
			return nil, fmt.Errorf("multiple ReleasePlanAdmissions found in namespace (%+s) with the origin (%+s) for application '%s'",
				releasePlan.Spec.Target, releasePlan.Namespace, releasePlan.Spec.Application)
		}

		foundReleasePlanAdmission = &releasePlanAdmissions.Items[i]
	}

	if foundReleasePlanAdmission == nil {
		return nil, fmt.Errorf("no ReleasePlanAdmission found in namespace (%+s) with the origin (%+s) for application '%s'",
			releasePlan.Spec.Target, releasePlan.Namespace, releasePlan.Spec.Application)
	}

	return foundReleasePlanAdmission, nil
}

// GetMatchingReleasePlans returns a list of all ReleasePlans that target the given ReleasePlanAdmission's
// namespace, specify an application that is included in the ReleasePlanAdmission's application list, and
// are in the namespace specified by the ReleasePlanAdmission's origin. If the List operation fails, an
// error will be returned.
func (l *loader) GetMatchingReleasePlans(ctx context.Context, cli client.Client, releasePlanAdmission *v1alpha1.ReleasePlanAdmission) (*v1alpha1.ReleasePlanList, error) {
	releasePlans := &v1alpha1.ReleasePlanList{}
	err := cli.List(ctx, releasePlans,
		client.InNamespace(releasePlanAdmission.Spec.Origin),
		client.MatchingFields{"spec.target": releasePlanAdmission.Namespace})
	if err != nil {
		return nil, err
	}

	for i := len(releasePlans.Items) - 1; i >= 0; i-- {
		if !slices.Contains(releasePlanAdmission.Spec.Applications, releasePlans.Items[i].Spec.Application) {
			// Remove ReleasePlans that do not have matching applications from the list
			releasePlans.Items = append(releasePlans.Items[:i], releasePlans.Items[i+1:]...)
		}
	}

	return releasePlans, nil
}

// GetPreviousRelease returns the Release that was created just before the given Release.
// If no previous Release is found, a NotFound error is returned.
func (l *loader) GetPreviousRelease(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.Release, error) {
	releases := &v1alpha1.ReleaseList{}
	err := cli.List(ctx, releases,
		client.InNamespace(release.Namespace),
		client.MatchingFields{"spec.releasePlan": release.Spec.ReleasePlan})
	if err != nil {
		return nil, err
	}

	var previousRelease *v1alpha1.Release

	// Find the previous release
	for i, possiblePreviousRelease := range releases.Items {
		// Ignore the release passed as argument and any release created after that one
		if possiblePreviousRelease.Name == release.Name ||
			possiblePreviousRelease.CreationTimestamp.After(release.CreationTimestamp.Time) {
			continue
		}
		if previousRelease == nil || possiblePreviousRelease.CreationTimestamp.After(previousRelease.CreationTimestamp.Time) {
			previousRelease = &releases.Items[i]
		}
	}

	if previousRelease == nil {
		return nil, errors.NewNotFound(
			schema.GroupResource{
				Group:    v1alpha1.GroupVersion.Group,
				Resource: release.GetObjectKind().GroupVersionKind().Kind,
			}, release.Name)
	}

	return previousRelease, nil
}

// GetRelease returns the Release with the given name and namespace. If the Release is not found or the Get operation
// fails, an error will be returned.
func (l *loader) GetRelease(ctx context.Context, cli client.Client, name, namespace string) (*v1alpha1.Release, error) {
	release := &v1alpha1.Release{}
	return release, toolkit.GetObject(name, namespace, cli, ctx, release)
}

// GetRoleBindingFromReleaseStatus returns the RoleBinding associated with the given Release. That association is defined
// by the namespaced name stored in the Release's status.
func (l *loader) GetRoleBindingFromReleaseStatus(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*rbac.RoleBinding, error) {
	roleBinding := &rbac.RoleBinding{}
	roleBindingNamespacedName := strings.Split(release.Status.ManagedProcessing.RoleBinding, string(types.Separator))
	if len(roleBindingNamespacedName) != 2 {
		return nil, fmt.Errorf("release doesn't contain a valid reference to a RoleBinding ('%s')",
			release.Status.ManagedProcessing.RoleBinding)
	}

	err := cli.Get(ctx, types.NamespacedName{
		Namespace: roleBindingNamespacedName[0],
		Name:      roleBindingNamespacedName[1],
	}, roleBinding)
	if err != nil {
		return nil, err
	}

	return roleBinding, nil
}

// GetReleasePipelineRun returns the Release PipelineRun of the specified type referenced by the given Release
// or nil if it's not found. In the case the List operation fails, an error will be returned.
func (l *loader) GetReleasePipelineRun(ctx context.Context, cli client.Client, release *v1alpha1.Release, pipelineType string) (*tektonv1.PipelineRun, error) {
	if pipelineType != metadata.ManagedPipelineType && pipelineType != metadata.TenantPipelineType {
		return nil, fmt.Errorf("cannot fetch Release PipelineRun with invalid type %s", pipelineType)
	}

	pipelineRuns := &tektonv1.PipelineRunList{}
	err := cli.List(ctx, pipelineRuns,
		client.Limit(1),
		client.MatchingLabels{
			metadata.ReleaseNameLabel:      release.Name,
			metadata.ReleaseNamespaceLabel: release.Namespace,
			metadata.PipelinesTypeLabel:    pipelineType,
		})
	if err == nil && len(pipelineRuns.Items) > 0 {
		return &pipelineRuns.Items[0], nil
	}

	return nil, err
}

// GetReleasePlan returns the ReleasePlan referenced by the given Release. If the ReleasePlan is not found or
// the Get operation fails, an error will be returned.
func (l *loader) GetReleasePlan(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*v1alpha1.ReleasePlan, error) {
	releasePlan := &v1alpha1.ReleasePlan{}
	return releasePlan, toolkit.GetObject(release.Spec.ReleasePlan, release.Namespace, cli, ctx, releasePlan)
}

// GetReleaseServiceConfig returns the ReleaseServiceConfig with the given name and namespace. If the ReleaseServiceConfig is not
// found or the Get operation fails, an error will be returned.
func (l *loader) GetReleaseServiceConfig(ctx context.Context, cli client.Client, name, namespace string) (*v1alpha1.ReleaseServiceConfig, error) {
	releaseServiceConfig := &v1alpha1.ReleaseServiceConfig{}
	return releaseServiceConfig, toolkit.GetObject(name, namespace, cli, ctx, releaseServiceConfig)
}

// GetSnapshot returns the Snapshot referenced by the given Release. If the Snapshot is not found or the Get
// operation fails, an error is returned.
func (l *loader) GetSnapshot(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*applicationapiv1alpha1.Snapshot, error) {
	snapshot := &applicationapiv1alpha1.Snapshot{}
	return snapshot, toolkit.GetObject(release.Spec.Snapshot, release.Namespace, cli, ctx, snapshot)
}

// ProcessingResources contains the required resources to process the Release.
type ProcessingResources struct {
	EnterpriseContractConfigMap *corev1.ConfigMap
	EnterpriseContractPolicy    *ecapiv1alpha1.EnterpriseContractPolicy
	ReleasePlan                 *v1alpha1.ReleasePlan
	ReleasePlanAdmission        *v1alpha1.ReleasePlanAdmission
	Snapshot                    *applicationapiv1alpha1.Snapshot
}

// GetProcessingResources returns all the resources required to process the Release. If any of those resources cannot
// be retrieved from the cluster, an error will be returned.
func (l *loader) GetProcessingResources(ctx context.Context, cli client.Client, release *v1alpha1.Release) (*ProcessingResources, error) {
	var err error
	resources := &ProcessingResources{}

	resources.ReleasePlan, err = l.GetReleasePlan(ctx, cli, release)
	if err != nil {
		return resources, err
	}

	resources.ReleasePlanAdmission, err = l.GetActiveReleasePlanAdmissionFromRelease(ctx, cli, release)
	if err != nil {
		return resources, err
	}

	resources.EnterpriseContractConfigMap, err = l.GetEnterpriseContractConfigMap(ctx, cli)
	if err != nil {
		return resources, err
	}

	resources.EnterpriseContractPolicy, err = l.GetEnterpriseContractPolicy(ctx, cli, resources.ReleasePlanAdmission)
	if err != nil {
		return resources, err
	}

	resources.Snapshot, err = l.GetSnapshot(ctx, cli, release)
	if err != nil {
		return resources, err
	}

	return resources, nil
}
