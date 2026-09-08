// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package scaleset

import (
	"testing"

	"github.com/stretchr/testify/require"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

// Deletion-lane records do not count toward the runner count, so a scale
// set holding only such records still scales up.
func TestRunnerCountExcludesDeletionLane(t *testing.T) {
	w := &Worker{
		scaleSet: params.ScaleSet{
			MaxRunners:         48,
			MinIdleRunners:     0,
			DesiredRunnerCount: 1,
		},
		runners: map[string]params.Instance{
			"a": {Name: "phantom-1", Status: commonParams.InstancePendingDelete},
			"b": {Name: "phantom-2", Status: commonParams.InstancePendingDelete},
		},
	}

	require.Equal(t, 0, w.runnerCount())
	require.Less(t, w.runnerCount(), w.targetRunners())
}

func TestRunnerCountMixedStatuses(t *testing.T) {
	w := &Worker{
		runners: map[string]params.Instance{
			"a": {Status: commonParams.InstanceRunning},
			"b": {Status: commonParams.InstancePendingCreate},
			"c": {Status: commonParams.InstanceCreating},
			"d": {Status: commonParams.InstanceError},
			"e": {Status: commonParams.InstancePendingDelete},
			"f": {Status: commonParams.InstancePendingForceDelete},
			"g": {Status: commonParams.InstanceDeleting},
			"h": {Status: commonParams.InstanceDeleted},
		},
	}

	require.Equal(t, 4, w.runnerCount())
}
