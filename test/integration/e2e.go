package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	client "github.com/cloudbase/garm/client"
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
	"github.com/cloudbase/garm/cmd/garm-cli/config"
	"github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
	openapiRuntimeClient "github.com/go-openapi/runtime/client"
)

var (
	cli       *client.GarmAPI
	cfg       config.Config
	authToken runtime.ClientAuthInfoWriter

	credentialsName = os.Getenv("CREDENTIALS_NAME")

	repoID            string
	repoPoolID        string
	repoInstanceName  string
	repoName          = os.Getenv("REPO_NAME")
	repoWebhookSecret = os.Getenv("REPO_WEBHOOK_SECRET")

	orgID            string
	orgPoolID        string
	orgInstanceName  string
	orgName          = os.Getenv("ORG_NAME")
	orgWebhookSecret = os.Getenv("ORG_WEBHOOK_SECRET")

	username = os.Getenv("GARM_USERNAME")
	password = os.Getenv("GARM_PASSWORD")
	fullName = os.Getenv("GARM_FULLNAME")
	email    = os.Getenv("GARM_EMAIL")
	name     = os.Getenv("GARM_NAME")
	baseURL  = os.Getenv("GARM_BASE_URL")

	poolID string
)

// //////////////// //
// helper functions //
// ///////////////////
func handleError(err error) {
	if err != nil {
		log.Fatalf("error encountered: %v", err)
	}
}

func printResponse(resp interface{}) {
	b, err := json.MarshalIndent(resp, "", "  ")
	handleError(err)
	log.Println(string(b))
}

// ///////////
// Garm Init /
// ///////////
func firstRun(apiCli *client.GarmAPI, newUser params.NewUserParams) (params.User, error) {
	firstRunResponse, err := apiCli.FirstRun.FirstRun(
		clientFirstRun.NewFirstRunParams().WithBody(newUser),
		authToken)
	if err != nil {
		return params.User{}, err
	}
	return firstRunResponse.Payload, nil
}

func login(apiCli *client.GarmAPI, params params.PasswordLoginParams) (string, error) {
	loginResponse, err := apiCli.Login.Login(
		clientLogin.NewLoginParams().WithBody(params),
		authToken)
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

// /////////////////
// Main functions //
// /////////////////
//
// /////////////
// Garm Init //
// /////////////
func Login() {
	log.Println(">>> Login")
	loginParams := params.PasswordLoginParams{
		Username: username,
		Password: password,
	}
	token, err := login(cli, loginParams)
	handleError(err)
	printResponse(token)
	authToken = openapiRuntimeClient.BearerToken(token)
	cfg.Managers = []config.Manager{
		{
			Name:    name,
			BaseURL: baseURL,
			Token:   token,
		},
	}
	cfg.ActiveManager = name
	err = cfg.SaveConfig()
	handleError(err)
}

func FirstRun() {
	existingCfg, err := config.LoadConfig()
	handleError(err)
	if existingCfg != nil {
		if existingCfg.HasManager(name) {
			log.Println(">>> Already initialized")
			return
		}
	}

	log.Println(">>> First run")
	newUser := params.NewUserParams{
		Username: username,
		Password: password,
		FullName: fullName,
		Email:    email,
	}
	user, err := firstRun(cli, newUser)
	handleError(err)
	printResponse(user)
}

// ////////////////////////////
// Credentials and Providers //
// ////////////////////////////
func ListCredentials() {
	log.Println(">>> List credentials")
	credentials, err := listCredentials(cli, authToken)
	handleError(err)
	printResponse(credentials)
}

func ListProviders() {
	log.Println(">>> List providers")
	providers, err := listProviders(cli, authToken)
	handleError(err)
	printResponse(providers)
}

// ////////
// Jobs //
// ////////
func ListJobs() {
	log.Println(">>> List jobs")
	jobs, err := listJobs(cli, authToken)
	handleError(err)
	printResponse(jobs)
}

// //////////////////
// / Metrics Token //
// //////////////////
func GetMetricsToken() {
	log.Println(">>> Get metrics token")
	token, err := getMetricsToken(cli, authToken)
	handleError(err)
	printResponse(token)
}

// ///////////////
// Repositories //
// ///////////////
func CreateRepo() {
	repos, err := listRepos(cli, authToken)
	handleError(err)
	if len(repos) > 0 {
		log.Println(">>> Repo already exists, skipping create")
		repoID = repos[0].ID
		return
	}
	log.Println(">>> Create repo")
	createParams := params.CreateRepoParams{
		Owner:           orgName,
		Name:            repoName,
		CredentialsName: credentialsName,
		WebhookSecret:   repoWebhookSecret,
	}
	repo, err := createRepo(cli, authToken, createParams)
	handleError(err)
	printResponse(repo)
	repoID = repo.ID
}

func ListRepos() {
	log.Println(">>> List repos")
	repos, err := listRepos(cli, authToken)
	handleError(err)
	printResponse(repos)
}

func UpdateRepo() {
	log.Println(">>> Update repo")
	updateParams := params.UpdateEntityParams{
		CredentialsName: fmt.Sprintf("%s-clone", credentialsName),
	}
	repo, err := updateRepo(cli, authToken, repoID, updateParams)
	handleError(err)
	printResponse(repo)
}

func GetRepo() {
	log.Println(">>> Get repo")
	repo, err := getRepo(cli, authToken, repoID)
	handleError(err)
	printResponse(repo)
}

func CreateRepoPool() {
	pools, err := listRepoPools(cli, authToken, repoID)
	handleError(err)
	if len(pools) > 0 {
		log.Println(">>> Repo pool already exists, skipping create")
		repoPoolID = pools[0].ID
		return
	}
	log.Println(">>> Create repo pool")
	poolParams := params.CreatePoolParams{
		MaxRunners:     2,
		MinIdleRunners: 0,
		Flavor:         "default",
		Image:          "ubuntu:22.04",
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		ProviderName:   "lxd_local",
		Tags:           []string{"ubuntu", "simple-runner"},
		Enabled:        true,
	}
	repo, err := createRepoPool(cli, authToken, repoID, poolParams)
	handleError(err)
	printResponse(repo)
	repoPoolID = repo.ID
}

func ListRepoPools() {
	log.Println(">>> List repo pools")
	pools, err := listRepoPools(cli, authToken, repoID)
	handleError(err)
	printResponse(pools)
}

func GetRepoPool() {
	log.Println(">>> Get repo pool")
	pool, err := getRepoPool(cli, authToken, repoID, repoPoolID)
	handleError(err)
	printResponse(pool)
}

func UpdateRepoPool() {
	log.Println(">>> Update repo pool")
	var maxRunners uint = 5
	var idleRunners uint = 1
	poolParams := params.UpdatePoolParams{
		MinIdleRunners: &idleRunners,
		MaxRunners:     &maxRunners,
	}
	pool, err := updateRepoPool(cli, authToken, repoID, repoPoolID, poolParams)
	handleError(err)
	printResponse(pool)
}

func DisableRepoPool() {
	enabled := false
	_, err := updateRepoPool(cli, authToken, repoID, repoPoolID, params.UpdatePoolParams{Enabled: &enabled})
	handleError(err)
	log.Printf("repo pool %s disabled", repoPoolID)
}

func WaitRepoPoolNoInstances() {
	for {
		log.Println(">>> Wait until repo pool has no instances")
		pool, err := getRepoPool(cli, authToken, repoID, repoPoolID)
		handleError(err)
		if len(pool.Instances) == 0 {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func WaitRepoInstance(timeout time.Duration) {
	var timeWaited time.Duration = 0
	var instance params.Instance

	for timeWaited < timeout {
		instances, err := listRepoInstances(cli, authToken, repoID)
		handleError(err)
		if len(instances) > 0 {
			instance = instances[0]
			log.Printf("instance %s status: %s", instance.Name, instance.Status)
			if instance.Status == commonParams.InstanceRunning && instance.RunnerStatus == params.RunnerIdle {
				repoInstanceName = instance.Name
				log.Printf("Repo instance %s is in running state", repoInstanceName)
				return
			}
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5
	}
	instanceDetails, err := getInstance(cli, authToken, instance.Name)
	handleError(err)
	printResponse(instanceDetails)

	log.Fatalf("Failed to wait for repo instance to be ready")
}

func ListRepoInstances() {
	log.Println(">>> List repo instances")
	instances, err := listRepoInstances(cli, authToken, repoID)
	handleError(err)
	printResponse(instances)
}

func DeleteRepo() {
	log.Println(">>> Delete repo")
	err := deleteRepo(cli, authToken, repoID)
	handleError(err)
	log.Printf("repo %s deleted", repoID)
}

func DeleteRepoPool() {
	log.Println(">>> Delete repo pool")
	err := deleteRepoPool(cli, authToken, repoID, repoPoolID)
	handleError(err)
	log.Printf("repo pool %s deleted", repoPoolID)
}

// ////////////////
// Organizations //
// ////////////////
func CreateOrg() {
	orgs, err := listOrgs(cli, authToken)
	handleError(err)
	if len(orgs) > 0 {
		log.Println(">>> Org already exists, skipping create")
		orgID = orgs[0].ID
		return
	}
	log.Println(">>> Create org")
	orgParams := params.CreateOrgParams{
		Name:            orgName,
		CredentialsName: credentialsName,
		WebhookSecret:   orgWebhookSecret,
	}
	org, err := createOrg(cli, authToken, orgParams)
	handleError(err)
	printResponse(org)
	orgID = org.ID
}

func ListOrgs() {
	log.Println(">>> List orgs")
	orgs, err := listOrgs(cli, authToken)
	handleError(err)
	printResponse(orgs)
}

func UpdateOrg() {
	log.Println(">>> Update org")
	updateParams := params.UpdateEntityParams{
		CredentialsName: fmt.Sprintf("%s-clone", credentialsName),
	}
	org, err := updateOrg(cli, authToken, orgID, updateParams)
	handleError(err)
	printResponse(org)
}

func GetOrg() {
	log.Println(">>> Get org")
	org, err := getOrg(cli, authToken, orgID)
	handleError(err)
	printResponse(org)
}

func CreateOrgPool() {
	pools, err := listOrgPools(cli, authToken, orgID)
	handleError(err)
	if len(pools) > 0 {
		log.Println(">>> Org pool already exists, skipping create")
		orgPoolID = pools[0].ID
		return
	}
	log.Println(">>> Create org pool")
	poolParams := params.CreatePoolParams{
		MaxRunners:     2,
		MinIdleRunners: 0,
		Flavor:         "default",
		Image:          "ubuntu:22.04",
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		ProviderName:   "lxd_local",
		Tags:           []string{"ubuntu", "simple-runner"},
		Enabled:        true,
	}
	org, err := createOrgPool(cli, authToken, orgID, poolParams)
	handleError(err)
	printResponse(org)
	orgPoolID = org.ID
}

func ListOrgPools() {
	log.Println(">>> List org pools")
	pools, err := listOrgPools(cli, authToken, orgID)
	handleError(err)
	printResponse(pools)
}

func GetOrgPool() {
	log.Println(">>> Get org pool")
	pool, err := getOrgPool(cli, authToken, orgID, orgPoolID)
	handleError(err)
	printResponse(pool)
}

func UpdateOrgPool() {
	log.Println(">>> Update org pool")
	var maxRunners uint = 5
	var idleRunners uint = 1
	poolParams := params.UpdatePoolParams{
		MinIdleRunners: &idleRunners,
		MaxRunners:     &maxRunners,
	}
	pool, err := updateOrgPool(cli, authToken, orgID, orgPoolID, poolParams)
	handleError(err)
	printResponse(pool)
}

func DisableOrgPool() {
	enabled := false
	_, err := updateOrgPool(cli, authToken, orgID, orgPoolID, params.UpdatePoolParams{Enabled: &enabled})
	handleError(err)
	log.Printf("org pool %s disabled", orgPoolID)
}

func WaitOrgPoolNoInstances() {
	for {
		log.Println(">>> Wait until org pool has no instances")
		pool, err := getOrgPool(cli, authToken, orgID, orgPoolID)
		handleError(err)
		if len(pool.Instances) == 0 {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func WaitOrgInstance(timeout time.Duration) {
	var timeWaited time.Duration = 0
	var instance params.Instance

	for timeWaited < timeout {
		instances, err := listOrgInstances(cli, authToken, orgID)
		handleError(err)
		if len(instances) > 0 {
			instance = instances[0]
			log.Printf("instance %s status: %s", instance.Name, instance.Status)
			if instance.Status == commonParams.InstanceRunning && instance.RunnerStatus == params.RunnerIdle {
				orgInstanceName = instance.Name
				log.Printf("Org instance %s is in running state", orgInstanceName)
				return
			}
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5
	}
	instanceDetails, err := getInstance(cli, authToken, instance.Name)
	handleError(err)
	printResponse(instanceDetails)

	log.Fatalf("Failed to wait for org instance to be ready")
}

func ListOrgInstances() {
	log.Println(">>> List org instances")
	instances, err := listOrgInstances(cli, authToken, orgID)
	handleError(err)
	printResponse(instances)
}

func DeleteOrg() {
	log.Println(">>> Delete org")
	err := deleteOrg(cli, authToken, orgID)
	handleError(err)
	log.Printf("org %s deleted", orgID)
}

func DeleteOrgPool() {
	log.Println(">>> Delete org pool")
	err := deleteOrgPool(cli, authToken, orgID, orgPoolID)
	handleError(err)
	log.Printf("org pool %s deleted", orgPoolID)
}

// ////////////
// Instances //
// ////////////
func ListInstances() {
	log.Println(">>> List instances")
	instances, err := listInstances(cli, authToken)
	handleError(err)
	printResponse(instances)
}

func GetInstance() {
	log.Println(">>> Get instance")
	instance, err := getInstance(cli, authToken, orgInstanceName)
	handleError(err)
	printResponse(instance)
}

func DeleteInstance(name string) {
	err := deleteInstance(cli, authToken, name)
	for {
		log.Printf(">>> Wait until instance %s is deleted", name)
		instances, err := listInstances(cli, authToken)
		handleError(err)
		for _, instance := range instances {
			if instance.Name == name {
				time.Sleep(5 * time.Second)

				continue
			}
		}
		break
	}
	handleError(err)
	log.Printf("instance %s deleted", name)
}

// ////////
// Pools //
// ////////
func CreatePool() {
	pools, err := listPools(cli, authToken)
	handleError(err)
	for _, pool := range pools {
		if pool.Image == "ubuntu:20.04" {
			// this is the extra pool to be deleted, later, via [DELETE] pools dedicated API.
			poolID = pool.ID
			return
		}
	}
	log.Println(">>> Create pool")
	poolParams := params.CreatePoolParams{
		MaxRunners:     2,
		MinIdleRunners: 0,
		Flavor:         "default",
		Image:          "ubuntu:20.04",
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		ProviderName:   "lxd_local",
		Tags:           []string{"ubuntu", "simple-runner"},
		Enabled:        true,
	}
	pool, err := createRepoPool(cli, authToken, repoID, poolParams)
	handleError(err)
	printResponse(pool)
	poolID = pool.ID
}

func ListPools() {
	log.Println(">>> List pools")
	pools, err := listPools(cli, authToken)
	handleError(err)
	printResponse(pools)
}

func UpdatePool() {
	log.Println(">>> Update pool")
	var maxRunners uint = 5
	var idleRunners uint = 0
	poolParams := params.UpdatePoolParams{
		MinIdleRunners: &idleRunners,
		MaxRunners:     &maxRunners,
	}
	pool, err := updatePool(cli, authToken, poolID, poolParams)
	handleError(err)
	printResponse(pool)
}

func GetPool() {
	log.Println(">>> Get pool")
	pool, err := getPool(cli, authToken, poolID)
	handleError(err)
	printResponse(pool)
}

func DeletePool() {
	log.Println(">>> Delete pool")
	err := deletePool(cli, authToken, poolID)
	handleError(err)
	log.Printf("pool %s deleted", poolID)
}

func ListPoolInstances() {
	log.Println(">>> List pool instances")
	instances, err := listPoolInstances(cli, authToken, repoPoolID)
	handleError(err)
	printResponse(instances)
}

func main() {
	//////////////////
	// initialize cli /
	//////////////////
	garmUrl, err := url.Parse(baseURL)
	handleError(err)
	apiPath, err := url.JoinPath(garmUrl.Path, client.DefaultBasePath)
	handleError(err)
	transportCfg := client.DefaultTransportConfig().
		WithHost(garmUrl.Host).
		WithBasePath(apiPath).
		WithSchemes([]string{garmUrl.Scheme})
	cli = client.NewHTTPClientWithConfig(nil, transportCfg)

	//////////////////
	// garm init //
	//////////////////
	FirstRun()
	Login()

	// ////////////////////////////
	// credentials and providers //
	// ////////////////////////////
	ListCredentials()
	ListProviders()

	//////////
	// jobs //
	//////////
	ListJobs()

	////////////////////
	/// metrics token //
	////////////////////
	GetMetricsToken()

	//////////////////
	// repositories //
	//////////////////
	CreateRepo()
	ListRepos()
	UpdateRepo()
	GetRepo()

	CreateRepoPool()
	ListRepoPools()
	GetRepoPool()
	UpdateRepoPool()

	//////////////////
	// organizations //
	//////////////////
	CreateOrg()
	ListOrgs()
	UpdateOrg()
	GetOrg()

	CreateOrgPool()
	ListOrgPools()
	GetOrgPool()
	UpdateOrgPool()

	///////////////
	// instances //
	///////////////
	WaitRepoInstance(180)
	ListRepoInstances()

	WaitOrgInstance(180)
	ListOrgInstances()

	ListInstances()
	GetInstance()

	///////////////
	// pools //
	///////////////
	CreatePool()
	ListPools()
	UpdatePool()
	GetPool()
	ListPoolInstances()

	/////////////
	// Cleanup //
	/////////////
	DisableRepoPool()
	DisableOrgPool()

	DeleteInstance(repoInstanceName)
	DeleteInstance(orgInstanceName)

	WaitRepoPoolNoInstances()
	WaitOrgPoolNoInstances()

	DeleteRepoPool()
	DeleteOrgPool()
	DeletePool()

	DeleteRepo()
	DeleteOrg()
}
