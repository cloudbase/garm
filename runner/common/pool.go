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
	// we spin up. We cache the tools for 5 minutes. This should save us a lot of API calls
	// in cases where we have a lot of runners spin up at the same time.
	PoolToolUpdateInterval = 5 * time.Minute

	// BackoffTimer is the time we wait before attempting to make another request
	// to the github API.
	BackoffTimer = 1 * time.Minute
)

//go:generate mockery --all
type PoolManager interface {
	// ID returns the ID of the entity (repo, org, enterprise)
	ID() string
	// WebhookSecret returns the unencrypted webhook secret associated with the webhook installed
	// in GitHub for GARM. For GARM to receive webhook events for an entity, either the operator or
	// GARM will have to create a webhook in GitHub which points to the GARM API server. To authenticate
	// the webhook, a webhook secret is used. This function returns that secret.
	WebhookSecret() string
	// GithubRunnerRegistrationToken returns a new registration token for a github runner. This is used
	// for GHES installations that have not yet upgraded to a version >= 3.10. Starting with 3.10, we use
	// just-in-time runners, which no longer require exposing a runner registration token.
	GithubRunnerRegistrationToken() (string, error)
	// HandleWorkflowJob handles a workflow job meant for a particular entity. When a webhook is fired for
	// a repo, org or enterprise, we determine the destination of that webhook, retrieve the pool manager
	// for it and call this function with the WorkflowJob as a parameter.
	HandleWorkflowJob(job params.WorkflowJob) error
	// RefreshState allows us to update webhook secrets and configuration for a pool manager.
	RefreshState(param params.UpdatePoolStateParams) error

	// DeleteRunner will attempt to remove a runner from the pool. If forceRemove is true, any error
	// received from the provider will be ignored and we will proceed to remove the runner from the database.
	// An error received while attempting to remove from GitHub (other than 404) will still stop the deletion
	// process. This can happen if the runner is already processing a job. At which point, you can simply cancel
	// the job in github. Doing so will prompt GARM to reap the runner automatically.
	DeleteRunner(runner params.Instance, forceRemove, bypassGHUnauthorizedError bool) error

	// InstallWebhook will create a webhook in github for the entity associated with this pool manager.
	InstallWebhook(ctx context.Context, param params.InstallWebhookParams) (params.HookInfo, error)
	// GetWebhookInfo will return information about the webhook installed in github for the entity associated
	GetWebhookInfo(ctx context.Context) (params.HookInfo, error)
	// UninstallWebhook will remove the webhook installed in github for the entity associated with this pool manager.
	UninstallWebhook(ctx context.Context) error

	// RootCABundle will return a CA bundle that must be installed on all runners in order to properly validate
	// x509 certificates used by various systems involved. This CA bundle is defined in the GARM config file and
	// can include multiple CA certificates for the GARM api server, GHES server and any provider API endpoint that
	// may use internal or self signed certificates.
	RootCABundle() (params.CertificateBundle, error)

	// Start will start the pool manager and all associated workers.
	Start() error
	// Stop will stop the pool manager and all associated workers.
	Stop() error
	// Status will return the current status of the pool manager.
	Status() params.PoolManagerStatus
	// Wait will block until the pool manager has stopped.
	Wait() error
}
