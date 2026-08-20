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
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
)

type wedgeTokenGetter struct{}

func (wedgeTokenGetter) NewInstanceJWTToken(_ params.Instance, _ params.ForgeEntity, _ uint) (string, error) {
	return "test-token", nil
}

func (wedgeTokenGetter) NewAgentJWTToken(_ params.Instance, _ params.ForgeEntity) (string, error) {
	return "test-token", nil
}

// A slow provider delete is raced by a forced pending_delete write, and
// the manager's own status update arrives only after the delete returns.
// The record must still converge to deleted instead of staying in
// pending_delete.
func TestPendingDeleteClobberWedgeConverges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher.InitWatcher(ctx)
	t.Cleanup(func() { watcher.CloseWatcher() })

	dbCfg := garmTesting.GetTestSqliteDBConfig(t)
	db, err := database.NewDatabase(ctx, dbCfg)
	require.NoError(t, err)

	adminCtx := garmTesting.ImpersonateAdminContext(ctx, db, t)

	_, err = db.InitController()
	require.NoError(t, err)

	githubEndpoint := garmTesting.CreateDefaultGithubEndpoint(adminCtx, db, t)
	testCreds := garmTesting.CreateTestGithubCredentials(adminCtx, "test-creds", db, t, githubEndpoint)

	org, err := db.CreateOrganization(adminCtx, "test-org", testCreds, "test-webhook-secret", params.PoolBalancerTypeRoundRobin, false)
	require.NoError(t, err)
	entity, err := org.GetEntity()
	require.NoError(t, err)

	scaleSet, err := db.CreateEntityScaleSet(adminCtx, entity, params.CreateScaleSetParams{
		Name:                   "test-scale-set",
		ScaleSetID:             1,
		ProviderName:           "test-provider",
		MaxRunners:             5,
		MinIdleRunners:         0,
		Image:                  "ubuntu:22.04",
		Flavor:                 "medium",
		OSType:                 commonParams.Linux,
		OSArch:                 commonParams.Amd64,
		Enabled:                true,
		RunnerBootstrapTimeout: 10,
	})
	require.NoError(t, err)

	instance, err := db.CreateScaleSetInstance(adminCtx, scaleSet.ID, params.CreateInstanceParams{
		Name:   "test-wedge-instance",
		Status: commonParams.InstancePendingCreate,
		OSType: commonParams.Linux,
		OSArch: commonParams.Amd64,
	})
	require.NoError(t, err)
	_, err = db.ForceUpdateInstance(adminCtx, instance.Name, params.UpdateInstanceParams{
		Status: commonParams.InstanceRunning,
	})
	require.NoError(t, err)

	deleteEntered := make(chan struct{})
	releaseDelete := make(chan struct{})
	providerMock := runnerCommonMocks.NewProvider(t)
	// The first delete call signals the test and blocks until released;
	// later calls return immediately.
	providerMock.On("DeleteInstance", mock.Anything, instance.Name, mock.Anything).Run(func(_ mock.Arguments) {
		close(deleteEntered)
		<-releaseDelete
	}).Return(nil).Once()
	providerMock.On("DeleteInstance", mock.Anything, instance.Name, mock.Anything).Return(nil).Maybe()

	worker, err := NewWorker(ctx, db, map[string]common.Provider{"test-provider": providerMock}, wedgeTokenGetter{})
	require.NoError(t, err)
	require.NoError(t, worker.Start())
	t.Cleanup(func() {
		if err := worker.Stop(); err != nil {
			t.Logf("stopping worker: %v", err)
		}
	})

	// running -> pending_delete, as on job completion.
	_, err = db.UpdateInstance(adminCtx, instance.Name, params.UpdateInstanceParams{
		Status: commonParams.InstancePendingDelete,
	})
	require.NoError(t, err)

	// Wait for the manager to start the provider delete.
	select {
	case <-deleteEntered:
	case <-time.After(30 * time.Second):
		t.Fatal("provider delete was never called")
	}

	// Let the deleting event reach the manager before the write below; the
	// watcher does not guarantee delivery order.
	time.Sleep(2 * time.Second)

	// Force pending_delete over deleting while the delete is in flight.
	_, err = db.ForceUpdateInstance(adminCtx, instance.Name, params.UpdateInstanceParams{
		Status: commonParams.InstancePendingDelete,
	})
	require.NoError(t, err)
	regressed, err := db.GetInstance(adminCtx, instance.Name)
	require.NoError(t, err)
	require.Equal(t, commonParams.InstancePendingDelete, regressed.Status)

	// Hold the delete past the 10s update-send timeout so the second event
	// is dropped, then let it finish.
	time.Sleep(12 * time.Second)
	close(releaseDelete)

	// The record must converge to deleted.
	require.Eventually(t, func() bool {
		inst, err := db.GetInstance(adminCtx, instance.Name)
		if err != nil {
			// The record was fully removed; also convergence.
			return true
		}
		return inst.Status == commonParams.InstanceDeleted
	}, 60*time.Second, time.Second, "instance never converged to deleted")

	if inst, err := db.GetInstance(adminCtx, instance.Name); err == nil {
		fmt.Printf("converged instance status: %s\n", inst.Status)
	}
}
