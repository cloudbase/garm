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
	"context"
	"time"

	"github.com/cloudbase/garm/params"
)

const (
	PoolScaleDownInterval     = 1 * time.Minute
	PoolConsilitationInterval = 5 * time.Second
	PoolReapTimeoutInterval   = 5 * time.Minute
	// Temporary tools download token is valid for 1 hour by default.
	// There is no point in making an API call to get available tools, for every runner
	// we spin up. We cache the tools for one minute. This should save us a lot of API calls
	// in cases where we have a lot of runners spin up at the same time.
	PoolToolUpdateInterval = 1 * time.Minute

	// BackoffTimer is the time we wait before attempting to make another request
	// to the github API.
	BackoffTimer = 1 * time.Minute
)

//go:generate mockery --all
type PoolManager interface {
	ID() string
	WebhookSecret() string
	GithubRunnerRegistrationToken() (string, error)
	HandleWorkflowJob(job params.WorkflowJob) error
	RefreshState(param params.UpdatePoolStateParams) error
	ForceDeleteRunner(runner params.Instance) error

	InstallWebhook(ctx context.Context, param params.InstallWebhookParams) (params.HookInfo, error)
	GetWebhookInfo(ctx context.Context) (params.HookInfo, error)
	UninstallWebhook(ctx context.Context) error

	RootCABundle() (params.CertificateBundle, error)

	// PoolManager lifecycle functions. Start/stop pool.
	Start() error
	Stop() error
	Status() params.PoolManagerStatus
	Wait() error
}
