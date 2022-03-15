package tekton

import (
	hasv1alpha1 "github.com/redhat-appstudio/application-service/api/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create a PipelineRun from a given ReleaseStrategy in the specified namespace. Component details are added
// to the annotation of the new PipelineRun to be able to reference it later on.
func CreatePipelineRunFromReleaseStrategy(strategy *v1alpha1.ReleaseStrategy,
	namespace string,
	component *hasv1alpha1.Component) *tektonv1beta1.PipelineRun {
	return &tektonv1beta1.PipelineRun{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				"pipelines.appstudio.openshift.io/type":    "release",
				"release.appstudio.openshift.io/component": component.Name,
				"release.appstudio.openshift.io/project":   component.Namespace,
			},
			GenerateName: strategy.Name + "-",
			Namespace:    namespace,
		},
		Spec: tektonv1beta1.PipelineRunSpec{
			PipelineRef: &tektonv1beta1.PipelineRef{
				Name:   strategy.Spec.Pipeline,
				Bundle: strategy.Spec.Bundle,
			},
		},
	}
}
