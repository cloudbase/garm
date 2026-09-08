// Copyright 2025 Cloudbase Solutions SRL
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

package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type AgentTestSuite struct {
	suite.Suite

	adminCtx context.Context
	store    dbCommon.Store
	runner   *Runner
	instance params.Instance
}

func (s *AgentTestSuite) SetupTest() {
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.store = db

	s.adminCtx = garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())

	githubEndpoint := garmTesting.CreateDefaultGithubEndpoint(s.adminCtx, db, s.T())
	testCreds := garmTesting.CreateTestGithubCredentials(s.adminCtx, "test-creds", db, s.T(), githubEndpoint)

	org, err := db.CreateOrganization(s.adminCtx, "test-org", testCreds, "test-webhook-secret", params.PoolBalancerTypeRoundRobin, false)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test org: %s", err))
	}

	entity, err := org.GetEntity()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get entity: %s", err))
	}

	pool, err := db.CreateEntityPool(s.adminCtx, entity, params.CreatePoolParams{
		ProviderName:           "test-provider",
		MaxRunners:             2,
		MinIdleRunners:         0,
		Image:                  "ubuntu:22.04",
		Flavor:                 "medium",
		OSType:                 commonParams.Linux,
		OSArch:                 commonParams.Amd64,
		Tags:                   []string{"linux", "amd64"},
		RunnerBootstrapTimeout: 10,
	})
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test pool: %s", err))
	}

	instance, err := db.CreateInstance(s.adminCtx, pool.ID, params.CreateInstanceParams{
		Name:   "test-agent-instance",
		OSType: commonParams.Linux,
		OSArch: commonParams.Amd64,
	})
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create test instance: %s", err))
	}
	s.instance = instance

	s.runner = &Runner{
		ctx:   s.adminCtx,
		store: db,
	}
}

func (s *AgentTestSuite) seedStatus(status commonParams.InstanceStatus) {
	_, err := s.store.ForceUpdateInstance(s.adminCtx, s.instance.Name, params.UpdateInstanceParams{
		Status: status,
	})
	s.Require().NoError(err)
}

func (s *AgentTestSuite) instanceCtx() context.Context {
	return auth.SetInstanceParams(context.Background(), s.instance)
}

// A terminated report on a running instance marks it pending_delete.
func (s *AgentTestSuite) TestSetInstanceToPendingDeleteFromRunning() {
	s.seedStatus(commonParams.InstanceRunning)

	err := s.runner.SetInstanceToPendingDelete(s.instanceCtx())
	s.Require().NoError(err)

	updated, err := s.store.GetInstance(s.adminCtx, s.instance.Name)
	s.Require().NoError(err)
	s.Require().Equal(commonParams.InstancePendingDelete, updated.Status)
}

// A terminated report must not move a deleting instance backwards.
func (s *AgentTestSuite) TestSetInstanceToPendingDeleteDoesNotRegressDeleting() {
	s.seedStatus(commonParams.InstanceDeleting)

	err := s.runner.SetInstanceToPendingDelete(s.instanceCtx())
	s.Require().NoError(err)

	updated, err := s.store.GetInstance(s.adminCtx, s.instance.Name)
	s.Require().NoError(err)
	s.Require().Equal(commonParams.InstanceDeleting, updated.Status)
}

// A terminated report on an already deleted record is a no-op.
func (s *AgentTestSuite) TestSetInstanceToPendingDeleteToleratesDeleted() {
	s.seedStatus(commonParams.InstanceDeleted)

	err := s.runner.SetInstanceToPendingDelete(s.instanceCtx())
	s.Require().NoError(err)

	updated, err := s.store.GetInstance(s.adminCtx, s.instance.Name)
	s.Require().NoError(err)
	s.Require().Equal(commonParams.InstanceDeleted, updated.Status)
}

func TestAgentTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTestSuite))
}
