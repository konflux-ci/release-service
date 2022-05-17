/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseStrategySpec defines the desired state of ReleaseStrategy
type ReleaseStrategySpec struct {
	// Release Tekton Pipeline to execute
	Pipeline string `json:"pipeline"`

	// Bundle is a reference to the Tekton bundle where to find the pipeline
	Bundle string `json:"bundle,omitempty"`

	// Params to pass to the pipeline
	Params []tektonv1beta1.Param `json:"params,omitempty"`

	// Policy to validate before releasing an artifact
	Policy string `json:"policy,omitempty"`
}

// ReleaseStrategyStatus defines the observed state of ReleaseStrategy
type ReleaseStrategyStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ReleaseStrategy is the Schema for the releasestrategies API
type ReleaseStrategy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseStrategySpec   `json:"spec,omitempty"`
	Status ReleaseStrategyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReleaseStrategyList contains a list of ReleaseStrategy
type ReleaseStrategyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReleaseStrategy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReleaseStrategy{}, &ReleaseStrategyList{})
}
