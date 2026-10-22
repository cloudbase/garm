// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

//go:build testing
// +build testing

package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

type statusWrite struct {
	name   string
	status commonParams.InstanceStatus
	fault  []byte
	force  bool
}

// storeStub stands in for the database. It records the status writes the
// manager attempts and can be told to reject them, which is how the real store
// reports a status the state machine refuses.
type storeStub struct {
	writes   []statusWrite
	writeErr error
}

func (s *storeStub) SetInstanceStatus(_ context.Context, name string, status commonParams.InstanceStatus, fault []byte, force bool) error {
	s.writes = append(s.writes, statusWrite{name: name, status: status, fault: fault, force: force})
	return s.writeErr
}

func (s *storeStub) InstanceTokenGetter() auth.InstanceTokenGetter { return nil }

func (s *storeStub) persistProviderInstanceState(_ context.Context, _ string, _ commonParams.ProviderInstance) (params.Instance, error) {
	return params.Instance{}, nil
}

func (s *storeStub) GetControllerInfo(_ context.Context) (params.ControllerInfo, error) {
	return params.ControllerInfo{}, nil
}

func (s *storeStub) GetGithubEntity(_ context.Context, _ params.ForgeEntity) (params.ForgeEntity, error) {
	return params.ForgeEntity{}, nil
}

func newTestManager(helper providerHelper, instance params.Instance) *instanceManager {
	m := &instanceManager{
		ctx:      context.Background(),
		instance: instance,
		helper:   helper,
	}
	m.running.Store(true)
	return m
}

// An instance left in creating has no owner. The create call that wrote that
// status returned without recording an outcome, and nothing else in GARM acts
// on creating. Consolidation must fail it so the error lane can clean it up.
func TestConsolidateStateFailsOrphanedCreating(t *testing.T) {
	store := &storeStub{}
	m := newTestManager(store, params.Instance{
		Name:   "runner-1",
		Status: commonParams.InstanceCreating,
	})

	require.NoError(t, m.consolidateState())

	require.Len(t, store.writes, 1)
	write := store.writes[0]
	require.Equal(t, "runner-1", write.name)
	require.Equal(t, commonParams.InstanceError, write.status)
	require.NotEmpty(t, write.fault, "the fault must say why the instance was failed")

	// The write must not be forced. A runner that booted and registered while
	// the manager was stuck is already running in the database, and running
	// does not permit a move to error, so an unforced write is what stops a
	// stale cached status from killing a live runner.
	require.False(t, write.force)
}

// The cached status can lag the database. If the row has moved on, the state
// machine refuses the write, and that refusal is not an error worth retrying.
// The watcher delivers the newer state on its own.
func TestConsolidateStateIgnoresRefusedTransition(t *testing.T) {
	store := &storeStub{
		writeErr: runnerErrors.NewInstanceTransitionError(
			commonParams.InstanceRunning, commonParams.InstanceError),
	}
	m := newTestManager(store, params.Instance{
		Name:   "runner-1",
		Status: commonParams.InstanceCreating,
	})

	require.NoError(t, m.consolidateState())
	require.Len(t, store.writes, 1)
}

// A record deleted underneath us is equally benign.
func TestConsolidateStateIgnoresMissingInstance(t *testing.T) {
	store := &storeStub{writeErr: runnerErrors.ErrNotFound}
	m := newTestManager(store, params.Instance{
		Name:   "runner-1",
		Status: commonParams.InstanceCreating,
	})

	require.NoError(t, m.consolidateState())
}

// Any other store failure must surface, so the loop logs it and the next tick
// retries. This is what turns the status write from a one-shot into a retry.
func TestConsolidateStateReturnsUnexpectedStoreError(t *testing.T) {
	store := &storeStub{writeErr: runnerErrors.ErrUnauthorized}
	m := newTestManager(store, params.Instance{
		Name:   "runner-1",
		Status: commonParams.InstanceCreating,
	})

	require.Error(t, m.consolidateState())
}

// A running instance is finished. Consolidation must leave it alone.
func TestConsolidateStateLeavesRunningAlone(t *testing.T) {
	store := &storeStub{}
	m := newTestManager(store, params.Instance{
		Name:   "runner-1",
		Status: commonParams.InstanceRunning,
	})

	require.NoError(t, m.consolidateState())
	require.Empty(t, store.writes)
}
