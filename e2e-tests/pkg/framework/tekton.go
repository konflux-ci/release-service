package framework

import (
	"context"
	"fmt"

	ecp "github.com/conforma/crds/api/v1alpha1"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	"github.com/konflux-ci/release-service/e2e-tests/pkg/constants"
	"github.com/konflux-ci/release-service/metadata"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TektonController handles Tekton-related operations: PipelineRun, TaskRun, EC Policy, PVC, and signing secrets.
type TektonController struct {
	*KubeClient
}

// NewTektonController creates a new TektonController.
func NewTektonController(kubeClient *KubeClient) *TektonController {
	return &TektonController{
		KubeClient: kubeClient,
	}
}

// =============================================================================
// PVC Operations
// =============================================================================

// CreatePVCInAccessMode creates a PVC with the specified access mode.
func (t *TektonController) CreatePVCInAccessMode(name, namespace string, accessMode corev1.PersistentVolumeAccessMode) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	err := t.kubeClient.Create(context.Background(), pvc)
	if err != nil {
		return nil, err
	}
	return pvc, nil
}

// =============================================================================
// Signing Secret Operations
// =============================================================================

// CreateOrUpdateSigningSecret creates or updates a signing secret.
func (t *TektonController) CreateOrUpdateSigningSecret(publicKey []byte, name, namespace string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"cosign.pub": publicKey,
		},
	}

	existingSecret := &corev1.Secret{}
	err := t.kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, existingSecret)
	if err != nil {
		// Create if not exists
		return t.kubeClient.Create(context.Background(), secret)
	}

	// Update existing
	existingSecret.Data = secret.Data
	return t.kubeClient.Update(context.Background(), existingSecret)
}

// =============================================================================
// Enterprise Contract Policy Operations
// =============================================================================

// GetEnterpriseContractPolicy gets an EnterpriseContractPolicy CR.
func (t *TektonController) GetEnterpriseContractPolicy(name, namespace string) (*ecp.EnterpriseContractPolicy, error) {
	policy := &ecp.EnterpriseContractPolicy{}
	err := t.kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, policy)
	if err != nil {
		return nil, fmt.Errorf("failed to get EnterpriseContractPolicy %s/%s: %w", namespace, name, err)
	}
	return policy, nil
}

// CreateEnterpriseContractPolicy creates an EnterpriseContractPolicy CR.
func (t *TektonController) CreateEnterpriseContractPolicy(name, namespace string, spec ecp.EnterpriseContractPolicySpec) (*ecp.EnterpriseContractPolicy, error) {
	policy := &ecp.EnterpriseContractPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}

	err := t.kubeClient.Create(context.Background(), policy)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

// =============================================================================
// PipelineRun Operations (using metadata from main project)
// =============================================================================

// GetPipelineRunInNamespace gets a PipelineRun for a release using labels from metadata package.
func (t *TektonController) GetPipelineRunInNamespace(namespace, releaseName, releaseNamespace string) (*tektonv1.PipelineRun, error) {
	pipelineRuns := &tektonv1.PipelineRunList{}
	err := t.kubeClient.List(context.Background(), pipelineRuns, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	for i := range pipelineRuns.Items {
		pr := &pipelineRuns.Items[i]
		if pr.Labels != nil {
			// Use metadata constants for label keys (reusing main project)
			if pr.Labels[metadata.ReleaseNameLabel] == releaseName &&
				pr.Labels[metadata.ReleaseNamespaceLabel] == releaseNamespace {
				return pr, nil
			}
		}
	}

	return nil, fmt.Errorf("no pipelinerun found for release %s/%s in namespace %s", releaseNamespace, releaseName, namespace)
}

// WaitForReleasePipelineToBeFinished waits for a release pipeline to finish.
// Uses knative apis.ConditionSucceeded for condition checking (same as main project).
func (t *TektonController) WaitForReleasePipelineToBeFinished(release *releaseApi.Release, managedNamespace string) error {
	return wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, constants.ReleasePipelineRunCompletionTimeout, true, func(ctx context.Context) (done bool, err error) {
		pr, err := t.GetPipelineRunInNamespace(managedNamespace, release.GetName(), release.GetNamespace())
		if err != nil {
			return false, nil
		}

		// Use knative apis package for condition checking (same pattern as main project)
		condition := pr.Status.GetCondition(apis.ConditionSucceeded)
		if condition == nil {
			return false, nil
		}

		if condition.IsTrue() {
			return true, nil
		}
		if condition.IsFalse() {
			return false, fmt.Errorf("pipelinerun %s/%s failed: %s", pr.Namespace, pr.Name, condition.Message)
		}
		return false, nil
	})
}

// =============================================================================
// TaskRun Operations
// =============================================================================

// GetTaskRunStatus gets the status of a TaskRun in a PipelineRun.
func (t *TektonController) GetTaskRunStatus(kubeClient client.Client, pr *tektonv1.PipelineRun, taskName string) (*tektonv1.PipelineRunTaskRunStatus, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName == taskName {
			// Fetch the actual TaskRun to get its status
			taskRun := &tektonv1.TaskRun{}
			err := kubeClient.Get(context.Background(), client.ObjectKey{
				Namespace: pr.Namespace,
				Name:      chr.Name,
			}, taskRun)
			if err != nil {
				return nil, fmt.Errorf("failed to get TaskRun %s: %w", chr.Name, err)
			}

			return &tektonv1.PipelineRunTaskRunStatus{
				PipelineTaskName: chr.PipelineTaskName,
				Status:           &taskRun.Status,
			}, nil
		}
	}
	return nil, fmt.Errorf("taskrun %s not found in pipelinerun %s/%s", taskName, pr.Namespace, pr.Name)
}

// DidTaskSucceed checks if a TaskRun succeeded using knative apis package.
func DidTaskSucceed(tr *tektonv1.PipelineRunTaskRunStatus) bool {
	if tr == nil || tr.Status == nil {
		return false
	}
	condition := tr.Status.GetCondition(apis.ConditionSucceeded)
	return condition != nil && condition.IsTrue()
}
