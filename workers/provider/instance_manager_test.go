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
package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
)

type statusWrite struct {
	status commonParams.InstanceStatus
	force  bool
}

// fakeProviderHelper records status writes and lets a test script their
// outcome. Everything else returns zero values; consolidateState's deletion
// path only needs SetInstanceStatus and GetControllerInfo.
type fakeProviderHelper struct {
	writes      []statusWrite
	onSetStatus func(w statusWrite) error
}

func (f *fakeProviderHelper) SetInstanceStatus(_ string, status commonParams.InstanceStatus, _ []byte, force bool) error {
	w := statusWrite{status: status, force: force}
	f.writes = append(f.writes, w)
	if f.onSetStatus != nil {
		return f.onSetStatus(w)
	}
	return nil
}

func (f *fakeProviderHelper) InstanceTokenGetter() auth.InstanceTokenGetter { return nil }

func (f *fakeProviderHelper) updateArgsFromProviderInstance(_ string, _ commonParams.ProviderInstance) (params.Instance, error) {
	return params.Instance{}, nil
}

func (f *fakeProviderHelper) GetControllerInfo() (params.ControllerInfo, error) {
	return params.ControllerInfo{}, nil
}

func (f *fakeProviderHelper) GetGithubEntity(e params.ForgeEntity) (params.ForgeEntity, error) {
	return e, nil
}

func newTestManager(status commonParams.InstanceStatus, provider common.Provider, helper providerHelper) *instanceManager {
	m := &instanceManager{
		ctx: context.Background(),
		instance: params.Instance{
			Name:       "test-instance",
			ProviderID: "test-instance",
			Status:     status,
		},
		provider: provider,
		helper:   helper,
	}
	m.running.Store(true)
	return m
}

// A manager whose cached state is deleting must resume the delete.
func TestConsolidateStateResumesDeleting(t *testing.T) {
	providerMock := runnerCommonMocks.NewProvider(t)
	providerMock.On("DeleteInstance", mock.Anything, "test-instance", mock.Anything).Return(nil)
	helper := &fakeProviderHelper{}
	m := newTestManager(commonParams.InstanceDeleting, providerMock, helper)

	err := m.consolidateState()
	require.ErrorIs(t, err, ErrInstanceDeleted)
	require.Equal(t, []statusWrite{
		{status: commonParams.InstanceDeleting, force: true},
		{status: commonParams.InstanceDeleted, force: false},
	}, helper.writes)
}

// A resumed delete that fails in the provider requeues.
func TestConsolidateStateResumedDeletingRequeuesOnProviderError(t *testing.T) {
	providerMock := runnerCommonMocks.NewProvider(t)
	providerMock.On("DeleteInstance", mock.Anything, "test-instance", mock.Anything).Return(fmt.Errorf("provider exploded"))
	helper := &fakeProviderHelper{}
	m := newTestManager(commonParams.InstanceDeleting, providerMock, helper)

	err := m.consolidateState()
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrInstanceDeleted)
	require.Equal(t, []statusWrite{
		{status: commonParams.InstanceDeleting, force: true},
		{status: commonParams.InstancePendingDelete, force: true},
	}, helper.writes)
	require.Greater(t, m.deleteBackoff.Nanoseconds(), int64(0))
}

// Force delete keeps its semantics: provider errors are ignored and the
// instance is marked deleted.
func TestConsolidateStateForceDeleteIgnoresProviderError(t *testing.T) {
	providerMock := runnerCommonMocks.NewProvider(t)
	providerMock.On("DeleteInstance", mock.Anything, "test-instance", mock.Anything).Return(fmt.Errorf("provider exploded"))
	helper := &fakeProviderHelper{}
	m := newTestManager(commonParams.InstancePendingForceDelete, providerMock, helper)

	err := m.consolidateState()
	require.ErrorIs(t, err, ErrInstanceDeleted)
	require.Equal(t, []statusWrite{
		{status: commonParams.InstanceDeleting, force: true},
		{status: commonParams.InstanceDeleted, force: false},
	}, helper.writes)
}

// A deleted write refused because the row regressed onto the deletion
// lane is retried with force.
func TestConsolidateStateForcesDeletedWhenRowRegressed(t *testing.T) {
	providerMock := runnerCommonMocks.NewProvider(t)
	providerMock.On("DeleteInstance", mock.Anything, "test-instance", mock.Anything).Return(nil)
	helper := &fakeProviderHelper{}
	helper.onSetStatus = func(w statusWrite) error {
		if w.status == commonParams.InstanceDeleted && !w.force {
			return fmt.Errorf("updating instance: %w", runnerErrors.NewInstanceTransitionError(commonParams.InstancePendingDelete, commonParams.InstanceDeleted))
		}
		return nil
	}
	m := newTestManager(commonParams.InstancePendingDelete, providerMock, helper)

	err := m.consolidateState()
	require.ErrorIs(t, err, ErrInstanceDeleted)
	require.Equal(t, []statusWrite{
		{status: commonParams.InstanceDeleting, force: true},
		{status: commonParams.InstanceDeleted, force: false},
		{status: commonParams.InstanceDeleted, force: true},
	}, helper.writes)
}

// Running remains a no-op.
func TestConsolidateStateRunningIsNoop(t *testing.T) {
	providerMock := runnerCommonMocks.NewProvider(t)
	helper := &fakeProviderHelper{}
	m := newTestManager(commonParams.InstanceRunning, providerMock, helper)

	err := m.consolidateState()
	require.NoError(t, err)
	require.Empty(t, helper.writes)
}
