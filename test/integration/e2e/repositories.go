package e2e

import (
	"log/slog"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

func CreateRepo(orgName, repoName, credentialsName, repoWebhookSecret string) *params.Repository {
	slog.Info("Create repository", "owner_name", orgName, "repo_name", repoName)
	createParams := params.CreateRepoParams{
		Owner:           orgName,
		Name:            repoName,
		CredentialsName: credentialsName,
		WebhookSecret:   repoWebhookSecret,
	}
	repo, err := createRepo(cli, authToken, createParams)
	if err != nil {
		panic(err)
	}
	return repo
}

func UpdateRepo(id, credentialsName string) *params.Repository {
	slog.Info("Update repo", "repo_id", id)
	updateParams := params.UpdateEntityParams{
		CredentialsName: credentialsName,
	}
	repo, err := updateRepo(cli, authToken, id, updateParams)
	if err != nil {
		panic(err)
	}
	return repo
}

func InstallRepoWebhook(id string) *params.HookInfo {
	slog.Info("Install repo webhook", "repo_id", id)
	webhookParams := params.InstallWebhookParams{
		WebhookEndpointType: params.WebhookEndpointDirect,
	}
	_, err := installRepoWebhook(cli, authToken, id, webhookParams)
	if err != nil {
		panic(err)
	}
	webhookInfo, err := getRepoWebhook(cli, authToken, id)
	if err != nil {
		panic(err)
	}
	return webhookInfo
}

func UninstallRepoWebhook(id string) {
	slog.Info("Uninstall repo webhook", "repo_id", id)
	if err := uninstallRepoWebhook(cli, authToken, id); err != nil {
		panic(err)
	}
}

func CreateRepoPool(repoID string, poolParams params.CreatePoolParams) *params.Pool {
	slog.Info("Create repo pool", "repo_id", repoID)
	pool, err := createRepoPool(cli, authToken, repoID, poolParams)
	if err != nil {
		panic(err)
	}
	return pool
}

func GetRepoPool(repoID, repoPoolID string) *params.Pool {
	slog.Info("Get repo pool", "repo_id", repoID, "pool_id", repoPoolID)
	pool, err := getRepoPool(cli, authToken, repoID, repoPoolID)
	if err != nil {
		panic(err)
	}
	return pool
}

func UpdateRepoPool(repoID, repoPoolID string, maxRunners, minIdleRunners uint) *params.Pool {
	slog.Info("Update repo pool", "repo_id", repoID, "pool_id", repoPoolID)
	poolParams := params.UpdatePoolParams{
		MinIdleRunners: &minIdleRunners,
		MaxRunners:     &maxRunners,
	}
	pool, err := updateRepoPool(cli, authToken, repoID, repoPoolID, poolParams)
	if err != nil {
		panic(err)
	}
	return pool
}

func DeleteRepoPool(repoID, repoPoolID string) {
	slog.Info("Delete repo pool", "repo_id", repoID, "pool_id", repoPoolID)
	if err := deleteRepoPool(cli, authToken, repoID, repoPoolID); err != nil {
		panic(err)
	}
}

func DisableRepoPool(repoID, repoPoolID string) {
	slog.Info("Disable repo pool", "repo_id", repoID, "pool_id", repoPoolID)
	enabled := false
	poolParams := params.UpdatePoolParams{Enabled: &enabled}
	if _, err := updateRepoPool(cli, authToken, repoID, repoPoolID, poolParams); err != nil {
		panic(err)
	}
}

func WaitRepoRunningIdleInstances(repoID string, timeout time.Duration) {
	repoPools, err := listRepoPools(cli, authToken, repoID)
	if err != nil {
		panic(err)
	}
	for _, pool := range repoPools {
		err := WaitPoolInstances(pool.ID, commonParams.InstanceRunning, params.RunnerIdle, timeout)
		if err != nil {
			_ = dumpRepoInstancesDetails(repoID)
			panic(err)
		}
	}
}

func dumpRepoInstancesDetails(repoID string) error {
	// print repo details
	slog.Info("Dumping repo details", "repo_id", repoID)
	repo, err := getRepo(cli, authToken, repoID)
	if err != nil {
		return err
	}
	if err := printJsonResponse(repo); err != nil {
		return err
	}

	// print repo instances details
	slog.Info("Dumping repo instances details", "repo_id", repoID)
	instances, err := listRepoInstances(cli, authToken, repoID)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		instance, err := getInstance(cli, authToken, instance.Name)
		if err != nil {
			return err
		}
		slog.Info("Instance info", "instance_name", instance.Name)
		if err := printJsonResponse(instance); err != nil {
			return err
		}
	}
	return nil
}
