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
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestReleaseLinkCreateValidatingWebhook(t *testing.T) {
	tests := []struct {
		name         string
		releaseLink  ReleaseLink
		errorMessage string
	}{
		{
			name:         "releaselink namespace cannot be the same as its target",
			errorMessage: "field spec.target and namespace cannot have the same value",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target:      "namespace1",
				},
			},
		},
		{
			name: "valid releaselink",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target:      "target1",
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
		},
		Spec: ReleaseLinkSpec{
			DisplayName: "releaselink1",
			Application: "application1",
			Target:      "target1",
		},
	}

	tests := []struct {
		name         string
		releaseLink  ReleaseLink
		errorMessage string
	}{
		{
			name:         "releaselink target cannot be changed to the resource's namespace",
			errorMessage: "field spec.target and namespace cannot have the same value",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target:      "namespace1",
				},
			},
		},
		{
			name: "valid update",
			releaseLink: ReleaseLink{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "namespace1",
				},
				Spec: ReleaseLinkSpec{
					DisplayName: "releaselink1",
					Application: "application1",
					Target:      "target2",
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
