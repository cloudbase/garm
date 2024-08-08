//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-openapi/runtime"
	openapiRuntimeClient "github.com/go-openapi/runtime/client"
	"github.com/stretchr/testify/suite"

	"github.com/cloudbase/garm/client"
	"github.com/cloudbase/garm/params"
)

var (
	orgName          string
	repoName         string
	orgWebhookSecret string
	workflowFileName string
)

type GarmSuite struct {
	suite.Suite
	cli             *client.GarmAPI
	authToken       runtime.ClientAuthInfoWriter
	ghToken         string
	credentialsName string
	repo            *params.Repository
}

func (suite *GarmSuite) SetupSuite() {
	t := suite.T()
	suite.ghToken = os.Getenv("GH_TOKEN")
	orgWebhookSecret = os.Getenv("ORG_WEBHOOK_SECRET")
	workflowFileName = os.Getenv("WORKFLOW_FILE_NAME")
	baseURL := os.Getenv("GARM_BASE_URL")
	adminPassword := os.Getenv("GARM_PASSWORD")
	adminUsername := os.Getenv("GARM_ADMIN_USERNAME")
	adminFullName := "GARM Admin"
	adminEmail := "admin@example.com"
	garmURL, err := url.Parse(baseURL)
	suite.NoError(err, "error parsing GARM_BASE_URL")

	apiPath, err := url.JoinPath(garmURL.Path, client.DefaultBasePath)
	suite.NoError(err, "error joining path")

	transportCfg := client.DefaultTransportConfig().
		WithHost(garmURL.Host).
		WithBasePath(apiPath).
		WithSchemes([]string{garmURL.Scheme})
	suite.cli = client.NewHTTPClientWithConfig(nil, transportCfg)

	t.Log("First run")
	newUser := params.NewUserParams{
		Username: adminUsername,
		Password: adminPassword,
		FullName: adminFullName,
		Email:    adminEmail,
	}
	_, err = firstRun(suite.cli, newUser)
	suite.NoError(err, "error at first run")

	t.Log("Login")
	loginParams := params.PasswordLoginParams{
		Username: adminUsername,
		Password: adminPassword,
	}
	token, err := login(suite.cli, loginParams)
	suite.NoError(err, "error at login")
	suite.authToken = openapiRuntimeClient.BearerToken(token)
	t.Log("Log in successful")

	suite.credentialsName = os.Getenv("CREDENTIALS_NAME")
	suite.EnsureTestCredentials(suite.credentialsName, suite.ghToken, "github.com")

	t.Log("Create repository")
	orgName = os.Getenv("ORG_NAME")
	repoName = os.Getenv("REPO_NAME")
	repoWebhookSecret := os.Getenv("REPO_WEBHOOK_SECRET")
	createParams := params.CreateRepoParams{
		Owner:           orgName,
		Name:            repoName,
		CredentialsName: suite.credentialsName,
		WebhookSecret:   repoWebhookSecret,
	}
	suite.repo, err = createRepo(suite.cli, suite.authToken, createParams)
	suite.NoError(err, "error creating repository")
	suite.Equal(orgName, suite.repo.Owner, "owner name mismatch")
	suite.Equal(repoName, suite.repo.Name, "repo name mismatch")
	suite.Equal(suite.credentialsName, suite.repo.CredentialsName, "credentials name mismatch")
}

func (suite *GarmSuite) TearDownSuite() {
	t := suite.T()
	t.Log("Graceful cleanup")
	// disable all the pools
	pools, err := listPools(suite.cli, suite.authToken)
	suite.NoError(err, "error listing pools")
	enabled := false
	poolParams := params.UpdatePoolParams{Enabled: &enabled}
	for _, pool := range pools {
		_, err := updatePool(suite.cli, suite.authToken, pool.ID, poolParams)
		suite.NoError(err, "error disabling pool")
		t.Logf("Pool %s disabled during stage graceful_cleanup", pool.ID)
	}

	// delete all the instances
	for _, pool := range pools {
		poolInstances, err := listPoolInstances(suite.cli, suite.authToken, pool.ID)
		suite.NoError(err, "error listing pool instances")
		for _, instance := range poolInstances {
			err := deleteInstance(suite.cli, suite.authToken, instance.Name, false, false)
			suite.NoError(err, "error deleting instance")
			t.Logf("Instance deletion initiated for instace %s during stage graceful_cleanup", instance.Name)
		}
	}

	// wait for all instances to be deleted
	for _, pool := range pools {
		err := suite.waitPoolNoInstances(pool.ID, 3*time.Minute)
		suite.NoError(err, "error waiting for pool to have no instances")
	}

	// delete all the pools
	for _, pool := range pools {
		err := deletePool(suite.cli, suite.authToken, pool.ID)
		suite.NoError(err, "error deleting pool")
		t.Logf("Pool %s deleted during stage graceful_cleanup", pool.ID)
	}

	// delete all the repositories
	repos, err := listRepos(suite.cli, suite.authToken)
	suite.NoError(err, "error listing repositories")
	for _, repo := range repos {
		err := deleteRepo(suite.cli, suite.authToken, repo.ID)
		suite.NoError(err, "error deleting repository")
		t.Logf("Repo %s deleted during stage graceful_cleanup", repo.ID)
	}

	// delete all the organizations
	orgs, err := listOrgs(suite.cli, suite.authToken)
	suite.NoError(err, "error listing organizations")
	for _, org := range orgs {
		err := deleteOrg(suite.cli, suite.authToken, org.ID)
		suite.NoError(err, "error deleting organization")
		t.Logf("Org %s deleted during stage graceful_cleanup", org.ID)
	}
}

func TestGarmTestSuite(t *testing.T) {
	suite.Run(t, new(GarmSuite))
}

func (suite *GarmSuite) waitPoolNoInstances(id string, timeout time.Duration) error {
	t := suite.T()
	var timeWaited time.Duration // default is 0
	var pool *params.Pool
	var err error

	t.Logf("Wait until pool with id %s has no instances", id)
	for timeWaited < timeout {
		pool, err = getPool(suite.cli, suite.authToken, id)
		suite.NoError(err, "error getting pool")
		t.Logf("Current pool has %d instances", len(pool.Instances))
		if len(pool.Instances) == 0 {
			return nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	err = suite.dumpPoolInstancesDetails(pool.ID)
	suite.NoError(err, "error dumping pool instances details")

	return fmt.Errorf("failed to wait for pool %s to have no instances", pool.ID)
}

func (suite *GarmSuite) GhOrgRunnersCleanup(ghToken, orgName, controllerID string) error {
	t := suite.T()
	t.Logf("Cleanup Github runners for controller %s and org %s", controllerID, orgName)

	client := getGithubClient(ghToken)
	ghOrgRunners, _, err := client.Actions.ListOrganizationRunners(context.Background(), orgName, nil)
	if err != nil {
		return err
	}

	// Remove organization runners
	controllerLabel := fmt.Sprintf("runner-controller-id:%s", controllerID)
	for _, orgRunner := range ghOrgRunners.Runners {
		for _, label := range orgRunner.Labels {
			if label.GetName() == controllerLabel {
				if _, err := client.Actions.RemoveOrganizationRunner(context.Background(), orgName, orgRunner.GetID()); err != nil {
					// We don't fail if we can't remove a single runner. This
					// is a best effort to try and remove all the orphan runners.
					t.Logf("Failed to remove organization runner %s: %v", orgRunner.GetName(), err)
					break
				}
				t.Logf("Removed organization runner %s", orgRunner.GetName())
				break
			}
		}
	}
	return nil
}
