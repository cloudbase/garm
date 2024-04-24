package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/test/integration/e2e"
)

var (
	adminPassword = os.Getenv("GARM_PASSWORD")
	adminUsername = os.Getenv("GARM_ADMIN_USERNAME")
	adminFullName = "GARM Admin"
	adminEmail    = "admin@example.com"

	baseURL         = os.Getenv("GARM_BASE_URL")
	credentialsName = os.Getenv("CREDENTIALS_NAME")

	repoName          = os.Getenv("REPO_NAME")
	repoWebhookSecret = os.Getenv("REPO_WEBHOOK_SECRET")
	repoPoolParams    = params.CreatePoolParams{
		MaxRunners:     2,
		MinIdleRunners: 0,
		Flavor:         "default",
		Image:          "ubuntu:22.04",
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		ProviderName:   "lxd_local",
		Tags:           []string{"repo-runner"},
		Enabled:        true,
	}
	repoPoolParams2 = params.CreatePoolParams{
		MaxRunners:     2,
		MinIdleRunners: 0,
		Flavor:         "default",
		Image:          "ubuntu:22.04",
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		ProviderName:   "test_external",
		Tags:           []string{"repo-runner-2"},
		Enabled:        true,
	}

	orgName          = os.Getenv("ORG_NAME")
	orgWebhookSecret = os.Getenv("ORG_WEBHOOK_SECRET")
	orgPoolParams    = params.CreatePoolParams{
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

	ghToken          = os.Getenv("GH_TOKEN")
	workflowFileName = os.Getenv("WORKFLOW_FILE_NAME")
)

func main() {
	/////////////
	// Cleanup //
	/////////////
	defer e2e.GracefulCleanup()

	///////////////
	// garm init //
	///////////////
	e2e.InitClient(baseURL)
	e2e.FirstRun(adminUsername, adminPassword, adminFullName, adminEmail)
	e2e.Login(adminUsername, adminPassword)

	// Ensure that the default "github.com" endpoint is automatically created.
	e2e.MustDefaultGithubEndpoint()
	// Create test credentials
	e2e.EnsureTestCredentials(credentialsName, ghToken, "github.com")

	// Test endpoint operations
	e2e.TestGithubEndpointOperations()

	// //////////////////
	// controller info //
	// //////////////////
	e2e.GetControllerInfo()

	// ////////////////////////////
	// credentials and providers //
	// ////////////////////////////
	e2e.ListCredentials()
	e2e.ListProviders()

	////////////////////
	/// metrics token //
	////////////////////
	e2e.GetMetricsToken()

	//////////////////
	// repositories //
	//////////////////
	repo := e2e.CreateRepo(orgName, repoName, credentialsName, repoWebhookSecret)
	repo = e2e.UpdateRepo(repo.ID, fmt.Sprintf("%s-clone", credentialsName))
	hookRepoInfo := e2e.InstallRepoWebhook(repo.ID)
	e2e.ValidateRepoWebhookInstalled(ghToken, hookRepoInfo.URL, orgName, repoName)
	e2e.UninstallRepoWebhook(repo.ID)
	e2e.ValidateRepoWebhookUninstalled(ghToken, hookRepoInfo.URL, orgName, repoName)
	_ = e2e.InstallRepoWebhook(repo.ID)
	e2e.ValidateRepoWebhookInstalled(ghToken, hookRepoInfo.URL, orgName, repoName)

	repoPool := e2e.CreateRepoPool(repo.ID, repoPoolParams)
	repoPool = e2e.GetRepoPool(repo.ID, repoPool.ID)
	e2e.DeleteRepoPool(repo.ID, repoPool.ID)

	repoPool = e2e.CreateRepoPool(repo.ID, repoPoolParams)
	_ = e2e.UpdateRepoPool(repo.ID, repoPool.ID, repoPoolParams.MaxRunners, 1)

	/////////////////////////////
	// Test external provider ///
	/////////////////////////////
	slog.Info("Testing external provider")
	repoPool2 := e2e.CreateRepoPool(repo.ID, repoPoolParams2)
	newParams := e2e.UpdateRepoPool(repo.ID, repoPool2.ID, repoPoolParams2.MaxRunners, 1)
	slog.Info("Updated repo pool", "new_params", newParams)
	err := e2e.WaitPoolInstances(repoPool2.ID, commonParams.InstanceRunning, params.RunnerPending, 1*time.Minute)
	if err != nil {
		slog.With(slog.Any("error", err)).Error("Failed to wait for instance to be running", "pool_id", repoPool2.ID, "provider_name", repoPoolParams2.ProviderName)
	}
	repoPool2 = e2e.GetRepoPool(repo.ID, repoPool2.ID)
	e2e.DisableRepoPool(repo.ID, repoPool2.ID)
	e2e.DeleteInstance(repoPool2.Instances[0].Name, false, false)
	err = e2e.WaitPoolInstances(repoPool2.ID, commonParams.InstancePendingDelete, params.RunnerPending, 1*time.Minute)
	if err != nil {
		slog.With(slog.Any("error", err)).Error("Failed to wait for instance to be running")
	}
	e2e.DeleteInstance(repoPool2.Instances[0].Name, true, false) // delete instance with forceRemove
	err = e2e.WaitInstanceToBeRemoved(repoPool2.Instances[0].Name, 1*time.Minute)
	if err != nil {
		slog.With(slog.Any("error", err)).Error("Failed to wait for instance to be removed")
	}
	e2e.DeleteRepoPool(repo.ID, repoPool2.ID)

	///////////////////
	// organizations //
	///////////////////
	org := e2e.CreateOrg(orgName, credentialsName, orgWebhookSecret)
	org = e2e.UpdateOrg(org.ID, fmt.Sprintf("%s-clone", credentialsName))
	orgHookInfo := e2e.InstallOrgWebhook(org.ID)
	e2e.ValidateOrgWebhookInstalled(ghToken, orgHookInfo.URL, orgName)
	e2e.UninstallOrgWebhook(org.ID)
	e2e.ValidateOrgWebhookUninstalled(ghToken, orgHookInfo.URL, orgName)
	_ = e2e.InstallOrgWebhook(org.ID)
	e2e.ValidateOrgWebhookInstalled(ghToken, orgHookInfo.URL, orgName)

	orgPool := e2e.CreateOrgPool(org.ID, orgPoolParams)
	orgPool = e2e.GetOrgPool(org.ID, orgPool.ID)
	e2e.DeleteOrgPool(org.ID, orgPool.ID)

	orgPool = e2e.CreateOrgPool(org.ID, orgPoolParams)
	_ = e2e.UpdateOrgPool(org.ID, orgPool.ID, orgPoolParams.MaxRunners, 1)

	///////////////
	// instances //
	///////////////
	e2e.WaitRepoRunningIdleInstances(repo.ID, 6*time.Minute)
	e2e.WaitOrgRunningIdleInstances(org.ID, 6*time.Minute)

	//////////
	// jobs //
	//////////
	e2e.TriggerWorkflow(ghToken, orgName, repoName, workflowFileName, "org-runner")
	e2e.ValidateJobLifecycle("org-runner")

	e2e.TriggerWorkflow(ghToken, orgName, repoName, workflowFileName, "repo-runner")
	e2e.ValidateJobLifecycle("repo-runner")
}
