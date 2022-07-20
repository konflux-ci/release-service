//
// Copyright 2022 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReleaseLinkDefaultingWebhook(t *testing.T) {
	tests := []struct {
		name        string
		releaseLink ReleaseLink
		labelValue  string
	}{
		{
			name:       fmt.Sprintf("valid releaselink without %s label", autoReleaseLabel),
			labelValue: "true",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace1",
						Workspace: "target-workspace",
					},
				},
			},
		},
		{
			name:       fmt.Sprintf("valid releaselink with %s label set", autoReleaseLabel),
			labelValue: "false",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "false",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace1",
						Workspace: "target-workspace",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.releaseLink.Default()

			assert.Contains(t, test.releaseLink.Labels[autoReleaseLabel], test.labelValue)
		})
	}
}
func TestReleaseLinkCreateValidatingWebhook(t *testing.T) {
	tests := []struct {
		name         string
		releaseLink  ReleaseLink
		errorMessage string
	}{
		{
			name:         "releaselink namespace cannot be the same as its target namespace",
			errorMessage: "field spec.target.namespace and namespace cannot have the same value",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "namespace1",
						Workspace: "target-workspace",
					},
				},
			},
		},
		{
			name:         "releaselink auto-release label must be true or false",
			errorMessage: fmt.Sprintf("%s label can only be set to true or false", autoReleaseLabel),
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "test",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace1",
						Workspace: "target-workspace",
					},
				},
			},
		},
		{
			name: fmt.Sprintf("valid releaselink without %s label", autoReleaseLabel),
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace1",
						Workspace: "target-workspace",
					},
				},
			},
		},
		{
			name: fmt.Sprintf("valid releaselink with %s label", autoReleaseLabel),
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "true",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace1",
						Workspace: "target-workspace",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.releaseLink.ValidateCreate()

			if test.errorMessage == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.errorMessage)
			}
		})
	}
}

func TestReleaseLinkUpdateValidatingWebhook(t *testing.T) {
	originalReleaseLink := ReleaseLink{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "namespace1",
			Labels: map[string]string{
				"release.appstudio.openshift.io/auto-release": "true",
			},
		},
		Spec: ReleaseLinkSpec{
			DisplayName: "releaselink1",
			Application: "application1",
			Target: Target{
				Namespace: "target-namespace1",
				Workspace: "target-workspace",
			},
		},
	}

	tests := []struct {
		name         string
		releaseLink  ReleaseLink
		errorMessage string
	}{
		{
			name:         "releaselink target cannot be changed to the resource's namespace",
			errorMessage: "field spec.target.namespace and namespace cannot have the same value",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "true",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "namespace1",
						Workspace: "target-workspace",
					},
				},
			},
		},
		{
			name:         fmt.Sprintf("set %s label to invalid value", autoReleaseLabel),
			errorMessage: fmt.Sprintf("%s label can only be set to true or false", autoReleaseLabel),
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "test",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace1",
						Workspace: "target-workspace",
					},
				},
			},
		},
		{
			name: "valid update of target",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "true",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace2",
						Workspace: "target-workspace",
					},
				},
			},
		},
		{
			name: fmt.Sprintf("valid update of %s label", autoReleaseLabel),
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
					Labels: map[string]string{
						"release.appstudio.openshift.io/auto-release": "false",
					},
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace2",
						Workspace: "target-workspace",
					},
				},
			},
		},
		{
			name: fmt.Sprintf("valid removal of %s label", autoReleaseLabel),
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target: Target{
						Namespace: "target-namespace2",
						Workspace: "target-workspace",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.releaseLink.ValidateUpdate(&originalReleaseLink)

			if test.errorMessage == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.errorMessage)
			}
		})
	}
}

func TestReleaseLinkDeleteValidatingWebhook(t *testing.T) {
	tests := []struct {
		name         string
		releaseLink  ReleaseLink
		errorMessage string
	}{
		{
			name:         "ValidateDelete should return nil, it's unimplimented",
			errorMessage: "",
			releaseLink:  ReleaseLink{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.releaseLink.ValidateDelete()

			if test.errorMessage == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.errorMessage)
			}
		})
	}
}
