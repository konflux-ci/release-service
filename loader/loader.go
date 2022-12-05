package loader

import (
	"context"
	"fmt"
	ecapiv1alpha1 "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// convertUnstructuredObject converts an unstructured object into an instance of the crd passed as an argument.
func convertUnstructuredObject[T any](unstructuredObject *unstructured.Unstructured, crd *T) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObject.UnstructuredContent(), crd)
}

// getObject loads an object from the cluster. This is a generic function that requires the object to be passed as an
// argument. The object is modified during the invocation, but it's returned as well to simplify the implementation of
// functions using this function.
func getObject[T any](name, namespace string, cli client.Client, ctx context.Context, object *T) (*T, error) {
	unstructuredObject, err := getUnstructuredObject(object)
	if err != nil {
		return nil, err
	}

	err = cli.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, unstructuredObject)

	if err != nil {
		return nil, err
	}

	return object, convertUnstructuredObject(unstructuredObject, object)
}

// getUnstructuredObject returns an unstructured object containing the same data as the object passed as an argument.
func getUnstructuredObject[T any](object T) (*unstructured.Unstructured, error) {
	unstructuredObject := &unstructured.Unstructured{}
	return unstructuredObject, scheme.Scheme.Convert(object, unstructuredObject, nil)
}

// GetActiveReleasePlanAdmission returns the ReleasePlanAdmission targeted by the given ReleasePlan.
// Only ReleasePlanAdmissions with the 'auto-release' label set to true (or missing the label, which is
// treated the same as having the label and it being set to true) will be searched for. If a matching
// ReleasePlanAdmission is not found or the List operation fails, an error will be returned.
func GetActiveReleasePlanAdmission(releasePlan *v1alpha1.ReleasePlan, cli client.Client, ctx context.Context) (*v1alpha1.ReleasePlanAdmission, error) {
	releasePlanAdmissions := &v1alpha1.ReleasePlanAdmissionList{}
	opts := []client.ListOption{
		client.InNamespace(releasePlan.Spec.Target),
		client.MatchingFields{"spec.origin": releasePlan.Namespace},
	}

	err := cli.List(ctx, releasePlanAdmissions, opts...)
	if err != nil {
		return nil, err
	}

	activeReleasePlanAdmissionFound := false

	for _, releasePlanAdmission := range releasePlanAdmissions.Items {
		if releasePlanAdmission.Spec.Application == releasePlan.Spec.Application {
			labelValue, found := releasePlanAdmission.GetLabels()[v1alpha1.AutoReleaseLabel]
			if found && labelValue == "false" {
				return nil, fmt.Errorf("found ReleasePlanAdmission '%s' with auto-release label set to false",
					releasePlanAdmission.Name)
			}
			activeReleasePlanAdmissionFound = true
		}
	}

	if !activeReleasePlanAdmissionFound {
		return nil, fmt.Errorf("no ReleasePlanAdmission found in the target (%+v) for application '%s'",
			releasePlan.Spec.Target, releasePlan.Spec.Application)
	}

	return &releasePlanAdmissions.Items[0], nil
}

// GetActiveReleasePlanAdmissionFromRelease returns the ReleasePlanAdmission targeted by the ReleasePlan referenced by
// the given Release. Only ReleasePlanAdmissions with the 'auto-release' label set to true (or missing the label, which
// is treated the same as having the label and it being set to true) will be searched for. If a matching
// ReleasePlanAdmission is not found or the List operation fails, an error will be returned.
func GetActiveReleasePlanAdmissionFromRelease(release *v1alpha1.Release, cli client.Client, ctx context.Context) (*v1alpha1.ReleasePlanAdmission, error) {
	releasePlan, err := GetReleasePlan(release, cli, ctx)
	if err != nil {
		return nil, err
	}

	return GetActiveReleasePlanAdmission(releasePlan, cli, ctx)
}

// GetApplication returns the Application referenced by the ReleasePlanAdmission. If the Application is not found or
// the Get operation failed, an error will be returned.
func GetApplication(releasePlanAdmission *v1alpha1.ReleasePlanAdmission, cli client.Client, ctx context.Context) (*applicationapiv1alpha1.Application, error) {
	return getObject(releasePlanAdmission.Spec.Application, releasePlanAdmission.Namespace, cli, ctx, &applicationapiv1alpha1.Application{})
}

// GetApplicationComponents returns a list of all the Components associated with the given Application.
func GetApplicationComponents(application applicationapiv1alpha1.Application, cli client.Client, ctx context.Context) ([]applicationapiv1alpha1.Component, error) {
	applicationComponents := &applicationapiv1alpha1.ComponentList{}
	opts := []client.ListOption{
		client.InNamespace(application.Namespace),
		client.MatchingFields{"spec.application": application.Name},
	}

	err := cli.List(ctx, applicationComponents, opts...)
	if err != nil {
		return nil, err
	}

	return applicationComponents.Items, nil
}

// GetEnterpriseContractPolicy returns the EnterpriseContractPolicy referenced by the given ReleaseStrategy. If the
// EnterpriseContractPolicy is not found or the Get operation fails, an error is returned.
func GetEnterpriseContractPolicy(releaseStrategy *v1alpha1.ReleaseStrategy, cli client.Client, ctx context.Context) (*ecapiv1alpha1.EnterpriseContractPolicy, error) {
	return getObject(releaseStrategy.Spec.Policy, releaseStrategy.Namespace, cli, ctx, &ecapiv1alpha1.EnterpriseContractPolicy{})
}

// GetEnvironment returns the Environment referenced by the given ReleasePlanAdmission. If the Environment is not found
// or the Get operation fails, an error will be returned.
func GetEnvironment(releasePlanAdmission *v1alpha1.ReleasePlanAdmission, cli client.Client, ctx context.Context) (*applicationapiv1alpha1.Environment, error) {
	return getObject(releasePlanAdmission.Spec.Environment, releasePlanAdmission.Namespace, cli, ctx, &applicationapiv1alpha1.Environment{})
}

// GetRelease returns the Release with the given name and namespace. If the Release is not found or the Get operation
// fails, an error will be returned.
func GetRelease(name, namespace string, cli client.Client, ctx context.Context) (*v1alpha1.Release, error) {
	return getObject(name, namespace, cli, ctx, &v1alpha1.Release{})
}

// GetReleasePipelineRun returns the PipelineRun referenced by the given Release or nil if it's not found. In the case
// the List operation fails, an error will be returned.
func GetReleasePipelineRun(release *v1alpha1.Release, cli client.Client, ctx context.Context) (*v1beta1.PipelineRun, error) {
	pipelineRuns := &v1beta1.PipelineRunList{}
	opts := []client.ListOption{
		client.Limit(1),
		client.MatchingLabels{
			tekton.ReleaseNameLabel:      release.Name,
			tekton.ReleaseNamespaceLabel: release.Namespace,
		},
	}

	err := cli.List(ctx, pipelineRuns, opts...)
	if err == nil && len(pipelineRuns.Items) > 0 {
		return &pipelineRuns.Items[0], nil
	}

	return nil, err
}

// GetReleasePlan returns the ReleasePlan referenced by the given Release. If the ReleasePlan is not found or
// the Get operation fails, an error will be returned.
func GetReleasePlan(release *v1alpha1.Release, cli client.Client, ctx context.Context) (*v1alpha1.ReleasePlan, error) {
	return getObject(release.Spec.ReleasePlan, release.Namespace, cli, ctx, &v1alpha1.ReleasePlan{})
}

// GetReleaseStrategy returns the ReleaseStrategy referenced by the given ReleasePlanAdmission. If the ReleaseStrategy
// is not found or the Get operation fails, an error will be returned.
func GetReleaseStrategy(releasePlanAdmission *v1alpha1.ReleasePlanAdmission, cli client.Client, ctx context.Context) (*v1alpha1.ReleaseStrategy, error) {
	return getObject(releasePlanAdmission.Spec.ReleaseStrategy, releasePlanAdmission.Namespace, cli, ctx, &v1alpha1.ReleaseStrategy{})
}

// GetSnapshot returns the Snapshot referenced by the Release being processed. If the Snapshot is not found or the Get
// operation fails, an error is returned.
func GetSnapshot(name, namespace string, cli client.Client, ctx context.Context) (*applicationapiv1alpha1.Snapshot, error) {
	return getObject(name, namespace, cli, ctx, &applicationapiv1alpha1.Snapshot{})
}

// GetSnapshotEnvironmentBinding returns the SnapshotEnvironmentBinding associated with the given ReleasePlanAdmission.
// That association is defined by both the Environment and Application matching between the ReleasePlanAdmission and
// the SnapshotEnvironmentBinding. If the Get operation fails, an error will be returned.
func GetSnapshotEnvironmentBinding(releasePlanAdmission *v1alpha1.ReleasePlanAdmission, cli client.Client, ctx context.Context) (*applicationapiv1alpha1.SnapshotEnvironmentBinding, error) {
	bindingList := &applicationapiv1alpha1.SnapshotEnvironmentBindingList{}
	opts := []client.ListOption{
		client.InNamespace(releasePlanAdmission.Namespace),
		client.MatchingFields{"spec.environment": releasePlanAdmission.Spec.Environment},
	}

	err := cli.List(ctx, bindingList, opts...)
	if err != nil {
		return nil, err
	}

	for _, binding := range bindingList.Items {
		if binding.Spec.Application == releasePlanAdmission.Spec.Application {
			return &binding, nil
		}
	}

	return nil, nil
}

// Composite functions

// ReleasePipelineRunResources contains the required resources for creating a Release PipelineRun.
type ReleasePipelineRunResources struct {
	EnterpriseContractPolicy *ecapiv1alpha1.EnterpriseContractPolicy
	ReleasePlanAdmission     *v1alpha1.ReleasePlanAdmission
	ReleaseStrategy          *v1alpha1.ReleaseStrategy
	Snapshot                 *applicationapiv1alpha1.Snapshot
}

// GetReleasePipelineRunResources returns all the resources required to create a SnapshotEnvironmentBinding. If any of
// those resources cannot be retrieved from the cluster, an error will be returned.
func GetReleasePipelineRunResources(release *v1alpha1.Release, cli client.Client, ctx context.Context) (ReleasePipelineRunResources, error) {
	resources := ReleasePipelineRunResources{}

	releasePlan, err := GetReleasePlan(release, cli, ctx)
	if err != nil {
		return resources, err
	}

	releasePlanAdmission, err := GetActiveReleasePlanAdmission(releasePlan, cli, ctx)
	if err != nil {
		return resources, err
	}
	resources.ReleasePlanAdmission = releasePlanAdmission

	releaseStrategy, err := GetReleaseStrategy(releasePlanAdmission, cli, ctx)
	if err != nil {
		return resources, err
	}
	resources.ReleaseStrategy = releaseStrategy

	enterpriseContractPolicy, err := GetEnterpriseContractPolicy(releaseStrategy, cli, ctx)
	if err != nil {
		return resources, err
	}
	resources.EnterpriseContractPolicy = enterpriseContractPolicy

	snapshot, err := GetSnapshot(release.Spec.Snapshot, release.Namespace, cli, ctx)
	if err != nil {
		return resources, err
	}
	resources.Snapshot = snapshot

	return resources, err
}

// SnapshotEnvironmentBindingResources contains the required resources for creating a SnapshotEnvironmentBinding.
type SnapshotEnvironmentBindingResources struct {
	Application           *applicationapiv1alpha1.Application
	ApplicationComponents []applicationapiv1alpha1.Component
	Environment           *applicationapiv1alpha1.Environment
	Snapshot              *applicationapiv1alpha1.Snapshot
}

// GetSnapshotEnvironmentBindingResources returns all the resources required to create a SnapshotEnvironmentBinding.
// If any of those resources cannot be retrieved from the cluster, an error will be returned.
func GetSnapshotEnvironmentBindingResources(release *v1alpha1.Release, releasePlanAdmission *v1alpha1.ReleasePlanAdmission, cli client.Client, ctx context.Context) (SnapshotEnvironmentBindingResources, error) {
	resources := SnapshotEnvironmentBindingResources{}

	application, err := GetApplication(releasePlanAdmission, cli, ctx)
	if err != nil {
		return resources, err
	}
	resources.Application = application

	applicationComponents, err := GetApplicationComponents(*application, cli, ctx)
	if err != nil {
		return resources, err
	}
	resources.ApplicationComponents = applicationComponents

	environment, err := GetEnvironment(releasePlanAdmission, cli, ctx)
	if err != nil {
		return resources, err
	}
	resources.Environment = environment

	snapshot, err := GetSnapshot(release.Spec.Snapshot, release.Namespace, cli, ctx)
	if err != nil {
		return resources, err
	}
	resources.Snapshot = snapshot

	return resources, nil
}
