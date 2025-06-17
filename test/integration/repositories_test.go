//go:build integration
// +build integration

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
package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

func (suite *GarmSuite) EnsureTestCredentials(name string, oauthToken string, endpointName string) {
	t := suite.T()
	t.Log("Ensuring test credentials exist")
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: oauthToken,
		},
	}
	suite.CreateGithubCredentials(createCredsParams)

	createCredsParams.Name = fmt.Sprintf("%s-clone", name)
	suite.CreateGithubCredentials(createCredsParams)
}

func (suite *GarmSuite) TestRepositories() {
	t := suite.T()

	t.Logf("Update repo with repo_id %s", suite.repo.ID)
	updateParams := params.UpdateEntityParams{
		CredentialsName: fmt.Sprintf("%s-clone", suite.credentialsName),
	}
	repo, err := updateRepo(suite.cli, suite.authToken, suite.repo.ID, updateParams)
	suite.NoError(err, "error updating repository")
	suite.Equal(fmt.Sprintf("%s-clone", suite.credentialsName), repo.CredentialsName, "credentials name mismatch")
	suite.repo = repo

	hookRepoInfo := suite.InstallRepoWebhook(suite.repo.ID)
	suite.ValidateRepoWebhookInstalled(suite.ghToken, hookRepoInfo.URL, orgName, repoName)
	suite.UninstallRepoWebhook(suite.repo.ID)
	suite.ValidateRepoWebhookUninstalled(suite.ghToken, hookRepoInfo.URL, orgName, repoName)

	suite.InstallRepoWebhook(suite.repo.ID)
	suite.ValidateRepoWebhookInstalled(suite.ghToken, hookRepoInfo.URL, orgName, repoName)

	repoPoolParams := params.CreatePoolParams{
		MaxRunners:     2,
		MinIdleRunners: 0,
		Flavor:         "default",
		Image:          "ubuntu:24.04",
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		ProviderName:   "lxd_local",
		Tags:           []string{"repo-runner"},
		Enabled:        true,
	}

	repoPool := suite.CreateRepoPool(suite.repo.ID, repoPoolParams)
	suite.Equal(repoPool.MaxRunners, repoPoolParams.MaxRunners, "max runners mismatch")
	suite.Equal(repoPool.MinIdleRunners, repoPoolParams.MinIdleRunners, "min idle runners mismatch")

	repoPoolGet := suite.GetRepoPool(suite.repo.ID, repoPool.ID)
	suite.Equal(*repoPool, *repoPoolGet, "pool get mismatch")

	suite.DeleteRepoPool(suite.repo.ID, repoPool.ID)

	repoPool = suite.CreateRepoPool(suite.repo.ID, repoPoolParams)
	updatedRepoPool := suite.UpdateRepoPool(suite.repo.ID, repoPool.ID, repoPoolParams.MaxRunners, 1)
	suite.NotEqual(updatedRepoPool.MinIdleRunners, repoPool.MinIdleRunners, "min idle runners mismatch")

	suite.WaitRepoRunningIdleInstances(suite.repo.ID, 6*time.Minute)
}

func (suite *GarmSuite) InstallRepoWebhook(id string) *params.HookInfo {
	t := suite.T()
	t.Logf("Install repo webhook with repo_id %s", id)
	webhookParams := params.InstallWebhookParams{
		WebhookEndpointType: params.WebhookEndpointDirect,
	}
	_, err := installRepoWebhook(suite.cli, suite.authToken, id, webhookParams)
	suite.NoError(err, "error installing repository webhook")

	webhookInfo, err := getRepoWebhook(suite.cli, suite.authToken, id)
	suite.NoError(err, "error getting repository webhook")
	return webhookInfo
}

func (suite *GarmSuite) ValidateRepoWebhookInstalled(ghToken, url, orgName, repoName string) {
	hook, err := getGhRepoWebhook(url, ghToken, orgName, repoName)
	suite.NoError(err, "error getting github webhook")
	suite.NotNil(hook, "github webhook with url %s, for repo %s/%s was not properly installed", url, orgName, repoName)
}

func getGhRepoWebhook(url, ghToken, orgName, repoName string) (*github.Hook, error) {
	client := getGithubClient(ghToken)
	ghRepoHooks, _, err := client.Repositories.ListHooks(context.Background(), orgName, repoName, nil)
	if err != nil {
		return nil, err
	}

	for _, hook := range ghRepoHooks {
		hookURL := hook.Config.GetURL()
		if hookURL == url {
			return hook, nil
		}
	}

	return nil, nil
}

func getGithubClient(oauthToken string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: oauthToken})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}

func (suite *GarmSuite) UninstallRepoWebhook(id string) {
	t := suite.T()
	t.Logf("Uninstall repo webhook with repo_id %s", id)
	err := uninstallRepoWebhook(suite.cli, suite.authToken, id)
	suite.NoError(err, "error uninstalling repository webhook")
}

func (suite *GarmSuite) ValidateRepoWebhookUninstalled(ghToken, url, orgName, repoName string) {
	hook, err := getGhRepoWebhook(url, ghToken, orgName, repoName)
	suite.NoError(err, "error getting github webhook")
	suite.Nil(hook, "github webhook with url %s, for repo %s/%s was not properly uninstalled", url, orgName, repoName)
}

func (suite *GarmSuite) CreateRepoPool(repoID string, poolParams params.CreatePoolParams) *params.Pool {
	t := suite.T()
	t.Logf("Create repo pool with repo_id %s and pool_params %+v", repoID, poolParams)
	pool, err := createRepoPool(suite.cli, suite.authToken, repoID, poolParams)
	suite.NoError(err, "error creating repository pool")
	return pool
}

func (suite *GarmSuite) GetRepoPool(repoID, repoPoolID string) *params.Pool {
	t := suite.T()
	t.Logf("Get repo pool repo_id %s and pool_id %s", repoID, repoPoolID)
	pool, err := getRepoPool(suite.cli, suite.authToken, repoID, repoPoolID)
	suite.NoError(err, "error getting repository pool")
	return pool
}

func (suite *GarmSuite) DeleteRepoPool(repoID, repoPoolID string) {
	t := suite.T()
	t.Logf("Delete repo pool with repo_id %s and pool_id %s", repoID, repoPoolID)
	err := deleteRepoPool(suite.cli, suite.authToken, repoID, repoPoolID)
	suite.NoError(err, "error deleting repository pool")
}

func (suite *GarmSuite) UpdateRepoPool(repoID, repoPoolID string, maxRunners, minIdleRunners uint) *params.Pool {
	t := suite.T()
	t.Logf("Update repo pool with repo_id %s and pool_id %s", repoID, repoPoolID)
	poolParams := params.UpdatePoolParams{
		MinIdleRunners: &minIdleRunners,
		MaxRunners:     &maxRunners,
	}
	pool, err := updateRepoPool(suite.cli, suite.authToken, repoID, repoPoolID, poolParams)
	suite.NoError(err, "error updating repository pool")
	return pool
}

func (suite *GarmSuite) WaitRepoRunningIdleInstances(repoID string, timeout time.Duration) {
	t := suite.T()
	repoPools, err := listRepoPools(suite.cli, suite.authToken, repoID)
	suite.NoError(err, "error listing repo pools")
	for _, pool := range repoPools {
		err := suite.WaitPoolInstances(pool.ID, commonParams.InstanceRunning, params.RunnerIdle, timeout)
		if err != nil {
			suite.dumpRepoInstancesDetails(repoID)
			t.Errorf("error waiting for pool instances to be running idle: %v", err)
		}
	}
}

func (suite *GarmSuite) dumpRepoInstancesDetails(repoID string) {
	t := suite.T()
	// print repo details
	t.Logf("Dumping repo details for repo %s", repoID)
	repo, err := getRepo(suite.cli, suite.authToken, repoID)
	suite.NoError(err, "error getting repo")
	err = printJSONResponse(repo)
	suite.NoError(err, "error printing repo")

	// print repo instances details
	t.Logf("Dumping repo instances details for repo %s", repoID)
	instances, err := listRepoInstances(suite.cli, suite.authToken, repoID)
	suite.NoError(err, "error listing repo instances")
	for _, instance := range instances {
		instance, err := getInstance(suite.cli, suite.authToken, instance.Name)
		suite.NoError(err, "error getting instance")
		t.Logf("Instance info for instance %s", instance.Name)
		err = printJSONResponse(instance)
		suite.NoError(err, "error printing instance")
	}
}
