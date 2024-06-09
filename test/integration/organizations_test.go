//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v57/github"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

func (suite *GarmSuite) TestOrganizations() {
	organization := suite.CreateOrg(orgName, suite.credentialsName, orgWebhookSecret)
	org := suite.UpdateOrg(organization.ID, fmt.Sprintf("%s-clone", suite.credentialsName))
	suite.NotEqual(organization, org, "organization not updated")
	orgHookInfo := suite.InstallOrgWebhook(org.ID)
	suite.ValidateOrgWebhookInstalled(suite.ghToken, orgHookInfo.URL, orgName)
	suite.UninstallOrgWebhook(org.ID)
	suite.ValidateOrgWebhookUninstalled(suite.ghToken, orgHookInfo.URL, orgName)
	_ = suite.InstallOrgWebhook(org.ID)
	suite.ValidateOrgWebhookInstalled(suite.ghToken, orgHookInfo.URL, orgName)

	orgPoolParams := params.CreatePoolParams{
		MaxRunners:     2,
		MinIdleRunners: 0,
		Flavor:         "default",
		Image:          "ubuntu:22.04",
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		ProviderName:   "lxd_local",
		Tags:           []string{"org-runner"},
		Enabled:        true,
	}
	orgPool := suite.CreateOrgPool(org.ID, orgPoolParams)
	orgPoolGot := suite.GetOrgPool(org.ID, orgPool.ID)
	suite.Equal(orgPool, orgPoolGot, "organization pool mismatch")
	suite.DeleteOrgPool(org.ID, orgPool.ID)

	orgPool = suite.CreateOrgPool(org.ID, orgPoolParams)
	orgPoolUpdated := suite.UpdateOrgPool(org.ID, orgPool.ID, orgPoolParams.MaxRunners, 1)
	suite.NotEqual(orgPool, orgPoolUpdated, "organization pool not updated")

	suite.WaitOrgRunningIdleInstances(org.ID, 6*time.Minute)
}

func (suite *GarmSuite) CreateOrg(orgName, credentialsName, orgWebhookSecret string) *params.Organization {
	t := suite.T()
	t.Logf("Create org with org_name %s", orgName)
	orgParams := params.CreateOrgParams{
		Name:            orgName,
		CredentialsName: credentialsName,
		WebhookSecret:   orgWebhookSecret,
	}
	org, err := createOrg(suite.cli, suite.authToken, orgParams)
	suite.NoError(err, "error creating organization")
	return org
}

func (suite *GarmSuite) UpdateOrg(id, credentialsName string) *params.Organization {
	t := suite.T()
	t.Logf("Update org with org_id %s", id)
	updateParams := params.UpdateEntityParams{
		CredentialsName: credentialsName,
	}
	org, err := updateOrg(suite.cli, suite.authToken, id, updateParams)
	suite.NoError(err, "error updating organization")
	return org
}

func (suite *GarmSuite) InstallOrgWebhook(id string) *params.HookInfo {
	t := suite.T()
	t.Logf("Install org webhook with org_id %s", id)
	webhookParams := params.InstallWebhookParams{
		WebhookEndpointType: params.WebhookEndpointDirect,
	}
	_, err := installOrgWebhook(suite.cli, suite.authToken, id, webhookParams)
	suite.NoError(err, "error installing organization webhook")
	webhookInfo, err := getOrgWebhook(suite.cli, suite.authToken, id)
	suite.NoError(err, "error getting organization webhook")
	return webhookInfo
}

func (suite *GarmSuite) ValidateOrgWebhookInstalled(ghToken, url, orgName string) {
	hook, err := getGhOrgWebhook(url, ghToken, orgName)
	suite.NoError(err, "error getting github webhook")
	suite.NotNil(hook, "github webhook with url %s, for org %s was not properly installed", url, orgName)
}

func getGhOrgWebhook(url, ghToken, orgName string) (*github.Hook, error) {
	client := getGithubClient(ghToken)
	ghOrgHooks, _, err := client.Organizations.ListHooks(context.Background(), orgName, nil)
	if err != nil {
		return nil, err
	}

	for _, hook := range ghOrgHooks {
		hookURL, ok := hook.Config["url"].(string)
		if ok && hookURL == url {
			return hook, nil
		}
	}

	return nil, nil
}

func (suite *GarmSuite) UninstallOrgWebhook(id string) {
	t := suite.T()
	t.Logf("Uninstall org webhook with org_id %s", id)
	err := uninstallOrgWebhook(suite.cli, suite.authToken, id)
	suite.NoError(err, "error uninstalling organization webhook")
}

func (suite *GarmSuite) ValidateOrgWebhookUninstalled(ghToken, url, orgName string) {
	hook, err := getGhOrgWebhook(url, ghToken, orgName)
	suite.NoError(err, "error getting github webhook")
	suite.Nil(hook, "github webhook with url %s, for org %s was not properly uninstalled", url, orgName)
}

func (suite *GarmSuite) CreateOrgPool(orgID string, poolParams params.CreatePoolParams) *params.Pool {
	t := suite.T()
	t.Logf("Create org pool with org_id %s", orgID)
	pool, err := createOrgPool(suite.cli, suite.authToken, orgID, poolParams)
	suite.NoError(err, "error creating organization pool")
	return pool
}

func (suite *GarmSuite) GetOrgPool(orgID, orgPoolID string) *params.Pool {
	t := suite.T()
	t.Logf("Get org pool with org_id %s and pool_id %s", orgID, orgPoolID)
	pool, err := getOrgPool(suite.cli, suite.authToken, orgID, orgPoolID)
	suite.NoError(err, "error getting organization pool")
	return pool
}

func (suite *GarmSuite) DeleteOrgPool(orgID, orgPoolID string) {
	t := suite.T()
	t.Logf("Delete org pool with org_id %s and pool_id %s", orgID, orgPoolID)
	err := deleteOrgPool(suite.cli, suite.authToken, orgID, orgPoolID)
	suite.NoError(err, "error deleting organization pool")
}

func (suite *GarmSuite) UpdateOrgPool(orgID, orgPoolID string, maxRunners, minIdleRunners uint) *params.Pool {
	t := suite.T()
	t.Logf("Update org pool with org_id %s and pool_id %s", orgID, orgPoolID)
	poolParams := params.UpdatePoolParams{
		MinIdleRunners: &minIdleRunners,
		MaxRunners:     &maxRunners,
	}
	pool, err := updateOrgPool(suite.cli, suite.authToken, orgID, orgPoolID, poolParams)
	suite.NoError(err, "error updating organization pool")
	return pool
}

func (suite *GarmSuite) WaitOrgRunningIdleInstances(orgID string, timeout time.Duration) {
	t := suite.T()
	orgPools, err := listOrgPools(suite.cli, suite.authToken, orgID)
	suite.NoError(err, "error listing organization pools")
	for _, pool := range orgPools {
		err := suite.WaitPoolInstances(pool.ID, commonParams.InstanceRunning, params.RunnerIdle, timeout)
		if err != nil {
			suite.dumpOrgInstancesDetails(orgID)
			t.Errorf("timeout waiting for organization %s instances to reach status: %s and runner status: %s", orgID, commonParams.InstanceRunning, params.RunnerIdle)
		}
	}
}

func (suite *GarmSuite) dumpOrgInstancesDetails(orgID string) {
	t := suite.T()
	// print org details
	t.Logf("Dumping org details with org_id %s", orgID)
	org, err := getOrg(suite.cli, suite.authToken, orgID)
	suite.NoError(err, "error getting organization")
	err = printJSONResponse(org)
	suite.NoError(err, "error printing organization")

	// print org instances details
	t.Logf("Dumping org instances details for org %s", orgID)
	instances, err := listOrgInstances(suite.cli, suite.authToken, orgID)
	suite.NoError(err, "error listing organization instances")
	for _, instance := range instances {
		instance, err := getInstance(suite.cli, suite.authToken, instance.Name)
		suite.NoError(err, "error getting instance")
		t.Logf("Instance info for instace %s", instance.Name)
		err = printJSONResponse(instance)
		suite.NoError(err, "error printing instance")
	}
}
