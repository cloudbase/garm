// Copyright 2026 Cloudbase Solutions SRL
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

//go:build testing

package pool

import (
	"github.com/stretchr/testify/mock"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (s *PoolStressTestSuite) TestJobCompletesBeforeProviderCreateReturns() {
	_, err := s.store.UpdateEntityPool(s.adminCtx, s.entity, s.pool.ID, params.UpdatePoolParams{
		ExtraSpecs: []byte(`{"runner_install_template":"IyEvYmluL3NoCg=="}`),
	})
	s.Require().NoError(err)
	s.mgr.instanceTokenGetter, err = auth.NewInstanceTokenGetter("test-secret")
	s.Require().NoError(err)
	instance, err := s.store.CreateInstance(s.adminCtx, s.pool.ID, params.CreateInstanceParams{
		Name: "fast-runner", Status: commonParams.InstanceCreating, RunnerStatus: params.RunnerActive,
		OSType: s.pool.OSType, OSArch: s.pool.OSArch,
	})
	s.Require().NoError(err)

	var completionErr error
	s.providerMock.On("CreateInstance", mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ mock.Arguments) {
			completionErr = s.mgr.handleCompletedJob(s.adminCtx, params.Job{RunnerName: instance.Name})
			creating, getErr := s.store.GetInstance(s.adminCtx, instance.Name)
			s.Require().NoError(getErr)
			s.Equal(commonParams.InstanceCreating, creating.Status)
			s.Equal(params.RunnerTerminated, creating.RunnerStatus)
		}).Return(commonParams.ProviderInstance{
		ProviderID: "provider-fast-runner", Status: commonParams.InstanceRunning,
	}, nil).Once()

	s.Require().NoError(s.mgr.addInstanceToProvider(instance))
	s.NoError(completionErr)
	created, err := s.store.GetInstance(s.adminCtx, instance.Name)
	s.Require().NoError(err)
	s.Equal("provider-fast-runner", created.ProviderID)
	s.Equal(params.RunnerTerminated, created.RunnerStatus)
	s.Equal(commonParams.InstancePendingDelete, created.Status)
}
