/*
Copyright 2025 Red Hat Inc.

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

import "github.com/konflux-ci/operator-toolkit/conditions"

const (
	// matchedConditionType is the type used to track the status of the ReleasePlan being matched to a
	// ReleasePlanAdmission or vice versa
	MatchedConditionType conditions.ConditionType = "Matched"
)

const (
	// MatchedReason is the reason set when a resource is matched
	MatchedReason conditions.ConditionReason = "Matched"
)
