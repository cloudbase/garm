package e2e

import (
	"github.com/cloudbase/garm/client"
	clientControllerInfo "github.com/cloudbase/garm/client/controller_info"
	clientCredentials "github.com/cloudbase/garm/client/credentials"
	clientFirstRun "github.com/cloudbase/garm/client/first_run"
	clientInstances "github.com/cloudbase/garm/client/instances"
	clientJobs "github.com/cloudbase/garm/client/jobs"
	clientLogin "github.com/cloudbase/garm/client/login"
	clientMetricsToken "github.com/cloudbase/garm/client/metrics_token"
	clientOrganizations "github.com/cloudbase/garm/client/organizations"
	clientPools "github.com/cloudbase/garm/client/pools"
	clientProviders "github.com/cloudbase/garm/client/providers"
	clientRepositories "github.com/cloudbase/garm/client/repositories"
	"github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
)

// ///////////
// Garm Init /
// ///////////
func firstRun(apiCli *client.GarmAPI, newUser params.NewUserParams) (params.User, error) {
	firstRunResponse, err := apiCli.FirstRun.FirstRun(
		clientFirstRun.NewFirstRunParams().WithBody(newUser),
		nil)
	if err != nil {
		return params.User{}, err
	}
	return firstRunResponse.Payload, nil
}

func login(apiCli *client.GarmAPI, params params.PasswordLoginParams) (string, error) {
	loginResponse, err := apiCli.Login.Login(
		clientLogin.NewLoginParams().WithBody(params),
		nil)
	if err != nil {
		return "", err
	}
	return loginResponse.Payload.Token, nil
}

// ////////////////////////////
// Credentials and Providers //
// ////////////////////////////
func listCredentials(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (params.Credentials, error) {
	listCredentialsResponse, err := apiCli.Credentials.ListCredentials(
		clientCredentials.NewListCredentialsParams(),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listCredentialsResponse.Payload, nil
}

func listProviders(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (params.Providers, error) {
	listProvidersResponse, err := apiCli.Providers.ListProviders(
		clientProviders.NewListProvidersParams(),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listProvidersResponse.Payload, nil
}

// ////////////////////////
// // Controller info ////
// ////////////////////////
func getControllerInfo(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (params.ControllerInfo, error) {
	controllerInfoResponse, err := apiCli.ControllerInfo.ControllerInfo(
		clientControllerInfo.NewControllerInfoParams(),
		apiAuthToken)
	if err != nil {
		return params.ControllerInfo{}, err
	}
	return controllerInfoResponse.Payload, nil
}

// ////////
// Jobs //
// ////////
func listJobs(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (params.Jobs, error) {
	listJobsResponse, err := apiCli.Jobs.ListJobs(
		clientJobs.NewListJobsParams(),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listJobsResponse.Payload, nil
}

// //////////////////
// / Metrics Token //
// //////////////////
func getMetricsToken(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (string, error) {
	getMetricsTokenResponse, err := apiCli.MetricsToken.GetMetricsToken(
		clientMetricsToken.NewGetMetricsTokenParams(),
		apiAuthToken)
	if err != nil {
		return "", err
	}
	return getMetricsTokenResponse.Payload.Token, nil
}

// ///////////////
// Repositories //
// ///////////////
func createRepo(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoParams params.CreateRepoParams) (*params.Repository, error) {
	createRepoResponse, err := apiCli.Repositories.CreateRepo(
		clientRepositories.NewCreateRepoParams().WithBody(repoParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &createRepoResponse.Payload, nil
}

func listRepos(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (params.Repositories, error) {
	listReposResponse, err := apiCli.Repositories.ListRepos(
		clientRepositories.NewListReposParams(),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listReposResponse.Payload, nil
}

func updateRepo(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID string, repoParams params.UpdateEntityParams) (*params.Repository, error) {
	updateRepoResponse, err := apiCli.Repositories.UpdateRepo(
		clientRepositories.NewUpdateRepoParams().WithRepoID(repoID).WithBody(repoParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &updateRepoResponse.Payload, nil
}

func getRepo(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID string) (*params.Repository, error) {
	getRepoResponse, err := apiCli.Repositories.GetRepo(
		clientRepositories.NewGetRepoParams().WithRepoID(repoID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &getRepoResponse.Payload, nil
}

func installRepoWebhook(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID string, webhookParams params.InstallWebhookParams) (*params.HookInfo, error) {
	installRepoWebhookResponse, err := apiCli.Repositories.InstallRepoWebhook(
		clientRepositories.NewInstallRepoWebhookParams().WithRepoID(repoID).WithBody(webhookParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &installRepoWebhookResponse.Payload, nil
}

func getRepoWebhook(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID string) (*params.HookInfo, error) {
	getRepoWebhookResponse, err := apiCli.Repositories.GetRepoWebhookInfo(
		clientRepositories.NewGetRepoWebhookInfoParams().WithRepoID(repoID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &getRepoWebhookResponse.Payload, nil
}

func createRepoPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID string, poolParams params.CreatePoolParams) (*params.Pool, error) {
	createRepoPoolResponse, err := apiCli.Repositories.CreateRepoPool(
		clientRepositories.NewCreateRepoPoolParams().WithRepoID(repoID).WithBody(poolParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &createRepoPoolResponse.Payload, nil
}

func listRepoPools(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID string) (params.Pools, error) {
	listRepoPoolsResponse, err := apiCli.Repositories.ListRepoPools(
		clientRepositories.NewListRepoPoolsParams().WithRepoID(repoID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listRepoPoolsResponse.Payload, nil
}

func getRepoPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID, poolID string) (*params.Pool, error) {
	getRepoPoolResponse, err := apiCli.Repositories.GetRepoPool(
		clientRepositories.NewGetRepoPoolParams().WithRepoID(repoID).WithPoolID(poolID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &getRepoPoolResponse.Payload, nil
}

func updateRepoPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID, poolID string, poolParams params.UpdatePoolParams) (*params.Pool, error) {
	updateRepoPoolResponse, err := apiCli.Repositories.UpdateRepoPool(
		clientRepositories.NewUpdateRepoPoolParams().WithRepoID(repoID).WithPoolID(poolID).WithBody(poolParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &updateRepoPoolResponse.Payload, nil
}

func listRepoInstances(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID string) (params.Instances, error) {
	listRepoInstancesResponse, err := apiCli.Repositories.ListRepoInstances(
		clientRepositories.NewListRepoInstancesParams().WithRepoID(repoID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listRepoInstancesResponse.Payload, nil
}

func deleteRepo(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID string) error {
	return apiCli.Repositories.DeleteRepo(
		clientRepositories.NewDeleteRepoParams().WithRepoID(repoID),
		apiAuthToken)
}

func deleteRepoPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, repoID, poolID string) error {
	return apiCli.Repositories.DeleteRepoPool(
		clientRepositories.NewDeleteRepoPoolParams().WithRepoID(repoID).WithPoolID(poolID),
		apiAuthToken)
}

// ////////////////
// Organizations //
// ////////////////
func createOrg(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgParams params.CreateOrgParams) (*params.Organization, error) {
	createOrgResponse, err := apiCli.Organizations.CreateOrg(
		clientOrganizations.NewCreateOrgParams().WithBody(orgParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &createOrgResponse.Payload, nil
}

func listOrgs(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (params.Organizations, error) {
	listOrgsResponse, err := apiCli.Organizations.ListOrgs(
		clientOrganizations.NewListOrgsParams(),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listOrgsResponse.Payload, nil
}

func updateOrg(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID string, orgParams params.UpdateEntityParams) (*params.Organization, error) {
	updateOrgResponse, err := apiCli.Organizations.UpdateOrg(
		clientOrganizations.NewUpdateOrgParams().WithOrgID(orgID).WithBody(orgParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &updateOrgResponse.Payload, nil
}

func getOrg(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID string) (*params.Organization, error) {
	getOrgResponse, err := apiCli.Organizations.GetOrg(
		clientOrganizations.NewGetOrgParams().WithOrgID(orgID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &getOrgResponse.Payload, nil
}

func installOrgWebhook(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID string, webhookParams params.InstallWebhookParams) (*params.HookInfo, error) {
	installOrgWebhookResponse, err := apiCli.Organizations.InstallOrgWebhook(
		clientOrganizations.NewInstallOrgWebhookParams().WithOrgID(orgID).WithBody(webhookParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &installOrgWebhookResponse.Payload, nil
}

func getOrgWebhook(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID string) (*params.HookInfo, error) {
	getOrgWebhookResponse, err := apiCli.Organizations.GetOrgWebhookInfo(
		clientOrganizations.NewGetOrgWebhookInfoParams().WithOrgID(orgID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &getOrgWebhookResponse.Payload, nil
}

func createOrgPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID string, poolParams params.CreatePoolParams) (*params.Pool, error) {
	createOrgPoolResponse, err := apiCli.Organizations.CreateOrgPool(
		clientOrganizations.NewCreateOrgPoolParams().WithOrgID(orgID).WithBody(poolParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &createOrgPoolResponse.Payload, nil
}

func listOrgPools(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID string) (params.Pools, error) {
	listOrgPoolsResponse, err := apiCli.Organizations.ListOrgPools(
		clientOrganizations.NewListOrgPoolsParams().WithOrgID(orgID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listOrgPoolsResponse.Payload, nil
}

func getOrgPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID, poolID string) (*params.Pool, error) {
	getOrgPoolResponse, err := apiCli.Organizations.GetOrgPool(
		clientOrganizations.NewGetOrgPoolParams().WithOrgID(orgID).WithPoolID(poolID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &getOrgPoolResponse.Payload, nil
}

func updateOrgPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID, poolID string, poolParams params.UpdatePoolParams) (*params.Pool, error) {
	updateOrgPoolResponse, err := apiCli.Organizations.UpdateOrgPool(
		clientOrganizations.NewUpdateOrgPoolParams().WithOrgID(orgID).WithPoolID(poolID).WithBody(poolParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &updateOrgPoolResponse.Payload, nil
}

func listOrgInstances(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID string) (params.Instances, error) {
	listOrgInstancesResponse, err := apiCli.Organizations.ListOrgInstances(
		clientOrganizations.NewListOrgInstancesParams().WithOrgID(orgID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listOrgInstancesResponse.Payload, nil
}

func deleteOrg(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID string) error {
	return apiCli.Organizations.DeleteOrg(
		clientOrganizations.NewDeleteOrgParams().WithOrgID(orgID),
		apiAuthToken)
}

func deleteOrgPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, orgID, poolID string) error {
	return apiCli.Organizations.DeleteOrgPool(
		clientOrganizations.NewDeleteOrgPoolParams().WithOrgID(orgID).WithPoolID(poolID),
		apiAuthToken)
}

// ////////////
// Instances //
// ////////////
func listInstances(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (params.Instances, error) {
	listInstancesResponse, err := apiCli.Instances.ListInstances(
		clientInstances.NewListInstancesParams(),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listInstancesResponse.Payload, nil
}

func getInstance(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, instanceID string) (*params.Instance, error) {
	getInstancesResponse, err := apiCli.Instances.GetInstance(
		clientInstances.NewGetInstanceParams().WithInstanceName(instanceID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &getInstancesResponse.Payload, nil
}

func deleteInstance(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, instanceID string) error {
	return apiCli.Instances.DeleteInstance(
		clientInstances.NewDeleteInstanceParams().WithInstanceName(instanceID),
		apiAuthToken)
}

// ////////
// Pools //
// ////////
func listPools(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter) (params.Pools, error) {
	listPoolsResponse, err := apiCli.Pools.ListPools(
		clientPools.NewListPoolsParams(),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listPoolsResponse.Payload, nil
}

func getPool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, poolID string) (*params.Pool, error) {
	getPoolResponse, err := apiCli.Pools.GetPool(
		clientPools.NewGetPoolParams().WithPoolID(poolID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &getPoolResponse.Payload, nil
}

func updatePool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, poolID string, poolParams params.UpdatePoolParams) (*params.Pool, error) {
	updatePoolResponse, err := apiCli.Pools.UpdatePool(
		clientPools.NewUpdatePoolParams().WithPoolID(poolID).WithBody(poolParams),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return &updatePoolResponse.Payload, nil
}

func listPoolInstances(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, poolID string) (params.Instances, error) {
	listPoolInstancesResponse, err := apiCli.Instances.ListPoolInstances(
		clientInstances.NewListPoolInstancesParams().WithPoolID(poolID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listPoolInstancesResponse.Payload, nil
}

func deletePool(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, poolID string) error {
	return apiCli.Pools.DeletePool(
		clientPools.NewDeletePoolParams().WithPoolID(poolID),
		apiAuthToken)
}
