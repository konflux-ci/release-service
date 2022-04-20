package tekton

import (
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreatePipelineRunFromReleaseStrategy creates a PipelineRun from a given ReleaseStrategy in the specified namespace.
// Release details are added to the annotation of the new PipelineRun to be able to reference it later on.
func CreatePipelineRunFromReleaseStrategy(strategy *v1alpha1.ReleaseStrategy,
	namespace string,
	release *v1alpha1.Release) *tektonv1beta1.PipelineRun {
	return &tektonv1beta1.PipelineRun{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: strategy.Name + "-",
			Labels: map[string]string{
				"pipelines.appstudio.openshift.io/type":    "release",
				"release.appstudio.openshift.io/release":   release.Name,
				"release.appstudio.openshift.io/workspace": release.Namespace,
			},
			Namespace: namespace,
		},
		Spec: tektonv1beta1.PipelineRunSpec{
			PipelineRef: &tektonv1beta1.PipelineRef{
				Name:   strategy.Spec.Pipeline,
				Bundle: strategy.Spec.Bundle,
			},
		},
	}
}
