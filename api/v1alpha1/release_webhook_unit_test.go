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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReleaseCreateValidatingWebhook(t *testing.T) {
	tests := []struct {
		name         string
		release      Release
		errorMessage string
	}{
		{
			name:         "ValidateCreate should return nil, it's unimplimented",
			errorMessage: "",
			release:      Release{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.release.ValidateCreate()

			if test.errorMessage == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.errorMessage)
			}
		})
	}
}

func TestReleaseUpdateValidatingWebhook(t *testing.T) {
	originalRelease := Release{
		Spec: ReleaseSpec{
			ApplicationSnapshot: "snapshot1",
			ReleaseLink:         "releaselink",
		},
	}

	tests := []struct {
		name         string
		release      Release
		errorMessage string
	}{
		{
			name:         "trying to update a release fails",
			errorMessage: "release resources spec cannot be updated",
			release: Release{
				Spec: ReleaseSpec{
					ApplicationSnapshot: "snapshot2",
					ReleaseLink:         "releaselink",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.release.ValidateUpdate(&originalRelease)

			if test.errorMessage == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.errorMessage)
			}
		})
	}
}

func TestReleaseDeleteValidatingWebhook(t *testing.T) {
	tests := []struct {
		name         string
		release      Release
		errorMessage string
	}{
		{
			name:         "ValidateDelete should return nil, it's unimplimented",
			errorMessage: "",
			release:      Release{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.release.ValidateDelete()

			if test.errorMessage == "" {
				assert.Nil(t, err)
			} else {
				assert.Contains(t, err.Error(), test.errorMessage)
			}
		})
	}
}
