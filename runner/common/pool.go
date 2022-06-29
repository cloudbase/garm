// Copyright 2022 Cloudbase Solutions SRL
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

package common

import (
	"garm/params"
)

type PoolType string

const (
	RepositoryPool   PoolType = "repository"
	OrganizationPool PoolType = "organization"
)

type PoolManager interface {
	ID() string
	WebhookSecret() string
	HandleWorkflowJob(job params.WorkflowJob) error
	RefreshState(param params.UpdatePoolStateParams) error
	ForceDeleteRunner(runner params.Instance) error
	// AddPool(ctx context.Context, pool params.Pool) error

	// PoolManager lifecycle functions. Start/stop pool.
	Start() error
	Stop() error
	Wait() error
}
