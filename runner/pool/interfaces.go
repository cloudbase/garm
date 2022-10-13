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

package pool

import (
	"garm/params"

	"github.com/google/go-github/v47/github"
)

type poolHelper interface {
	GetGithubToken() string
	GetGithubRunners() ([]*github.Runner, error)
	FetchTools() ([]*github.RunnerApplicationDownload, error)
	FetchDbInstances() ([]params.Instance, error)
	RemoveGithubRunner(runnerID int64) (*github.Response, error)
	ListPools() ([]params.Pool, error)
	GithubURL() string
	JwtToken() string
	GetGithubRegistrationToken() (string, error)
	String() string
	GetCallbackURL() string
	FindPoolByTags(labels []string) (params.Pool, error)
	GetPoolByID(poolID string) (params.Pool, error)
	ValidateOwner(job params.WorkflowJob) error
	UpdateState(param params.UpdatePoolStateParams) error
	WebhookSecret() string
	GetRunnerNameFromWorkflow(job params.WorkflowJob) (string, error)
	ID() string
}
