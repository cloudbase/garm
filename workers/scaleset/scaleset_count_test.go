// Copyright 2026 Cloudbase Solutions SRL
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
	"fmt"
	"testing"

	commonParams "github.com/cloudbase/garm-provider-common/params"

	"github.com/cloudbase/garm/params"
)

func workerWith(minIdle, desired, maxRunners uint, groups ...[]params.Instance) *Worker {
	w := &Worker{
		runners: map[string]params.Instance{},
		scaleSet: params.ScaleSet{
			MinIdleRunners:     minIdle,
			DesiredRunnerCount: int(desired),
			MaxRunners:         maxRunners,
		},
	}
	for _, group := range groups {
		for _, runner := range group {
			id := fmt.Sprintf("runner-%d", len(w.runners))
			runner.ID = id
			w.runners[id] = runner
		}
	}
	return w
}

func instances(count int, status commonParams.InstanceStatus, runnerStatus params.RunnerStatus) []params.Instance {
	out := make([]params.Instance, count)
	for i := range out {
		out[i] = params.Instance{Status: status, RunnerStatus: runnerStatus}
	}
	return out
}

func TestUsableRunnerCountExcludesRunnersThatWillNotTakeAJob(t *testing.T) {
	w := workerWith(10, 0, 100,
		instances(3, commonParams.InstanceRunning, params.RunnerIdle),
		instances(2, commonParams.InstanceRunning, params.RunnerActive),
		instances(4, commonParams.InstanceRunning, params.RunnerPending),
		instances(1, commonParams.InstanceRunning, params.RunnerInstalling),
		instances(1, commonParams.InstanceCreating, params.RunnerPending),
		instances(1, commonParams.InstancePendingCreate, params.RunnerPending),
		// Still in the provider, but they will never take another job.
		instances(5, commonParams.InstancePendingDelete, params.RunnerTerminated),
		instances(6, commonParams.InstanceDeleting, params.RunnerTerminated),
		instances(2, commonParams.InstancePendingForceDelete, params.RunnerIdle),
		instances(3, commonParams.InstanceRunning, params.RunnerTerminated),
		instances(1, commonParams.InstanceRunning, params.RunnerFailed),
		instances(2, commonParams.InstanceError, params.RunnerPending),
		// Gone from the provider.
		instances(7, commonParams.InstanceDeleted, params.RunnerTerminated),
	)

	if got := w.usableRunnerCount(); got != 12 {
		t.Fatalf("usableRunnerCount() = %d, want 12", got)
	}
	// Everything except the deleted records still occupies provider capacity.
	if got := w.runnerCount(); got != 31 {
		t.Fatalf("runnerCount() = %d, want 31", got)
	}
}

// With ephemeral runners and slow provider deletes, the runners being deleted
// can outnumber MinIdleRunners + DesiredRunnerCount. The pool must still be
// short of its target, by the number of usable runners it is missing.
func TestRunnersBeingDeletedDoNotSatisfyTheTarget(t *testing.T) {
	w := workerWith(80, 15, 500,
		instances(15, commonParams.InstanceRunning, params.RunnerActive),
		instances(2, commonParams.InstanceRunning, params.RunnerIdle),
		instances(87, commonParams.InstanceDeleting, params.RunnerTerminated),
	)

	if target, usable := w.targetRunners(), w.usableRunnerCount(); target-usable != 78 {
		t.Fatalf("target %d, usable %d: want the pool 78 runners short", target, usable)
	}
	if count := w.runnerCount(); count <= w.targetRunners() {
		t.Fatalf("runnerCount() = %d should exceed the target of %d in this scenario", count, w.targetRunners())
	}
}
