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
	"time"

	"github.com/cloudbase/garm/params"
)

const (
	PoolScaleDownInterval     = 1 * time.Minute
	PoolConsilitationInterval = 5 * time.Second
	PoolReapTimeoutInterval   = 5 * time.Minute
	// Temporary tools download token is valid for 1 hour by default.
	// Set this to 15 minutes. This should allow enough time even on slow
	// clouds for the instance to spin up, download the tools and join gh.
	PoolToolUpdateInterval = 15 * time.Minute

	// UnauthorizedBackoffTimer is the time we wait before making another request
	// after getting an unauthorized error from github. It is unlikely that a second
	// request will not receive the same error, unless the config is changed with new
	// credentials and garm is restarted.
	UnauthorizedBackoffTimer = 3 * time.Hour
)

//go:generate mockery --all
type PoolManager interface {
	ID() string
	WebhookSecret() string
	GithubRunnerRegistrationToken() (string, error)
	HandleWorkflowJob(job params.WorkflowJob) error
	RefreshState(param params.UpdatePoolStateParams) error
	ForceDeleteRunner(runner params.Instance) error
	// AddPool(ctx context.Context, pool params.Pool) error

	// PoolManager lifecycle functions. Start/stop pool.
	Start() error
	Stop() error
	Status() params.PoolManagerStatus
	Wait() error
}
