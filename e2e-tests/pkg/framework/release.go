package framework

import (
	"context"
	"fmt"
	"strconv"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/loader"
	releaseMetadata "github.com/konflux-ci/release-service/metadata"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
)

// ReleaseController handles Release, ReleasePlan, and ReleasePlanAdmission CRD operations.
type ReleaseController struct {
	*KubeClient
	loader loader.ObjectLoader
}

// NewReleaseController creates a new ReleaseController with the loader.
func NewReleaseController(kubeClient *KubeClient) *ReleaseController {
	return &ReleaseController{
		KubeClient: kubeClient,
		loader:     loader.NewLoader(),
	}
}

// =============================================================================
// Release Operations
// =============================================================================

// CreateRelease creates a Release.
func (r *ReleaseController) CreateRelease(name, namespace, snapshot, releasePlan string) (*releaseApi.Release, error) {
	release := &releaseApi.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: releaseApi.ReleaseSpec{
			Snapshot:    snapshot,
			ReleasePlan: releasePlan,
		},
	}

	err := r.kubeClient.Create(context.Background(), release)
	if err != nil {
		return nil, err
	}
	return release, nil
}

// GetRelease gets a Release using the loader package (reuses main project code).
func (r *ReleaseController) GetRelease(name, namespace string) (*releaseApi.Release, error) {
	return r.loader.GetRelease(context.Background(), r.kubeClient, name, namespace)
}

// GetReleasesInNamespace gets the list of Releases in a namespace.
func (r *ReleaseController) GetReleasesInNamespace(namespace string) (*releaseApi.ReleaseList, error) {
	releaseList := &releaseApi.ReleaseList{}
	err := r.kubeClient.List(context.Background(), releaseList, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	return releaseList, nil
}

// GetFirstReleaseInNamespace gets the first Release in a namespace.
func (r *ReleaseController) GetFirstReleaseInNamespace(namespace string) (*releaseApi.Release, error) {
	releaseList, err := r.GetReleasesInNamespace(namespace)
	if err != nil {
		return nil, err
	}
	if len(releaseList.Items) == 0 {
		return nil, &NotFoundError{Resource: "Release", Namespace: namespace}
	}
	return &releaseList.Items[0], nil
}

// WaitForRelease polls until the Release's "Released" condition shows it has finished.
// Returns an error if the release failed.
func (r *ReleaseController) WaitForRelease(release *releaseApi.Release) (*releaseApi.Release, error) {
	current := release
	err := wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, constants.ReleasePipelineRunCompletionTimeout, true, func(ctx context.Context) (bool, error) {
		fetched, getErr := r.GetRelease(release.Name, release.Namespace)
		if getErr != nil {
			return false, nil
		}
		current = fetched

		condition := meta.FindStatusCondition(current.Status.Conditions, "Released")
		if condition == nil {
			return false, nil
		}
		if condition.Status == metav1.ConditionTrue {
			return true, nil
		}
		if condition.Status == metav1.ConditionFalse && condition.Reason != "Progressing" {
			return false, fmt.Errorf("release %s/%s failed: %s", current.Namespace, current.Name, condition.Message)
		}
		return false, nil
	})
	return current, err
}

// GetManagedPipelineRetryCount returns the number of retries performed for the managed pipeline
// (attempts minus the initial run) same as main project code.
func GetManagedPipelineRetryCount(release *releaseApi.Release) int {
	if count := len(release.Status.ManagedPipelineAttempts) - 1; count > 0 {
		return count
	}
	return 0
}

// CreateReleasePipelineRoleBindingForServiceAccount creates a RoleBinding for the release pipeline SA.
func (r *ReleaseController) CreateReleasePipelineRoleBindingForServiceAccount(namespace string, serviceAccount *corev1.ServiceAccount) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "release-pipeline-rolebinding",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "release-pipeline-resource-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	err := r.kubeClient.Create(context.Background(), rb)
	if err != nil {
		return nil, err
	}
	return rb, nil
}

// =============================================================================
// ReleaseServiceConfig Operations
// =============================================================================

// CreateReleaseServiceConfig creates a ReleaseServiceConfig.
func (r *ReleaseController) CreateReleaseServiceConfig(name, namespace string, retryablePipelines []releaseApi.RetryablePipeline) error {
	rsc := &releaseApi.ReleaseServiceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: releaseApi.ReleaseServiceConfigSpec{
			RetryablePipelines: retryablePipelines,
		},
	}
	return r.kubeClient.Create(context.Background(), rsc)
}

// DeleteReleaseServiceConfig deletes a ReleaseServiceConfig.
func (r *ReleaseController) DeleteReleaseServiceConfig(name, namespace string) error {
	rsc := &releaseApi.ReleaseServiceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return r.kubeClient.Delete(context.Background(), rsc)
}

// =============================================================================
// ReleasePlan Operations
// =============================================================================

// CreateReleasePlan creates a ReleasePlan.
func (r *ReleaseController) CreateReleasePlan(name, namespace, application, targetNamespace, autoRelease string, data *runtime.RawExtension, tenantPipeline, finalpipeline *tektonutils.ParameterizedPipeline, serviceAccount *string) (*releaseApi.ReleasePlan, error) {
	// Set auto-release label - default to "true" if empty (matching original behavior)
	autoReleaseValue := autoRelease
	if autoReleaseValue == "" {
		autoReleaseValue = "true"
	}

	rp := &releaseApi.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				releaseMetadata.AutoReleaseLabel: autoReleaseValue,
				releaseMetadata.AttributionLabel: "true",
			},
		},
		Spec: releaseApi.ReleasePlanSpec{
			Application:    application,
			Target:         targetNamespace,
			Data:           data,
			TenantPipeline: tenantPipeline,
			FinalPipeline:  finalpipeline,
		},
	}

	err := r.kubeClient.Create(context.Background(), rp)
	if err != nil {
		return nil, err
	}
	return rp, nil
}

// GetReleasePlan gets a ReleasePlan by name and namespace.
func (r *ReleaseController) GetReleasePlan(name, namespace string) (*releaseApi.ReleasePlan, error) {
	rp := &releaseApi.ReleasePlan{}
	err := r.kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, rp)
	return rp, err
}

// DeleteReleasePlan deletes a ReleasePlan.
func (r *ReleaseController) DeleteReleasePlan(name, namespace string, wait bool) error {
	rp := &releaseApi.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return r.kubeClient.Delete(context.Background(), rp)
}

// =============================================================================
// ReleasePlanAdmission Operations
// =============================================================================

// CreateReleasePlanAdmission creates a ReleasePlanAdmission.
func (r *ReleaseController) CreateReleasePlanAdmission(name, namespace, environment, origin, policy, serviceAccount string, applications []string, blockReleases bool, pipelineRef *tektonutils.PipelineRef, data *runtime.RawExtension, maxRetries *int, taskRunSpecs []tektonv1.PipelineTaskRunSpec, timeouts *tektonv1.TimeoutFields) (*releaseApi.ReleasePlanAdmission, error) {
	rpa := &releaseApi.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				releaseMetadata.BlockReleasesLabel: strconv.FormatBool(blockReleases),
			},
		},
		Spec: releaseApi.ReleasePlanAdmissionSpec{
			Applications: applications,
			Origin:       origin,
			Policy:       policy,
			Pipeline: &tektonutils.Pipeline{
				PipelineRef:        *pipelineRef,
				ServiceAccountName: serviceAccount,
				MaxRetries:         maxRetries,
				TaskRunSpecs:       taskRunSpecs,
			},
			Data: data,
		},
	}

	if timeouts != nil {
		rpa.Spec.Pipeline.Timeouts = *timeouts
	}

	err := r.kubeClient.Create(context.Background(), rpa)
	if err != nil {
		return nil, err
	}
	return rpa, nil
}

// GetReleasePlanAdmission gets a ReleasePlanAdmission by name and namespace.
func (r *ReleaseController) GetReleasePlanAdmission(name, namespace string) (*releaseApi.ReleasePlanAdmission, error) {
	rpa := &releaseApi.ReleasePlanAdmission{}
	err := r.kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, rpa)
	return rpa, err
}

// DeleteReleasePlanAdmission deletes a ReleasePlanAdmission.
func (r *ReleaseController) DeleteReleasePlanAdmission(name, namespace string, waitForDeletion bool) error {
	rpa := &releaseApi.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return r.kubeClient.Delete(context.Background(), rpa)
}

// =============================================================================
// Error Types
// =============================================================================

// NotFoundError represents a resource not found error.
type NotFoundError struct {
	Resource  string
	Namespace string
}

func (e *NotFoundError) Error() string {
	return "no " + e.Resource + " found in namespace " + e.Namespace
}
