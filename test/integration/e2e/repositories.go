package e2e

import (
	"log"
	"time"

	"github.com/cloudbase/garm/params"
)

func CreateRepo(orgName, repoName, credentialsName, repoWebhookSecret string) *params.Repository {
	log.Printf("Create repository %s/%s", orgName, repoName)
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
	log.Printf("Update repo %s", id)
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
	log.Printf("Install repo %s webhook", id)
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
	log.Printf("Uninstall repo %s webhook", id)
	if err := uninstallRepoWebhook(cli, authToken, id); err != nil {
		panic(err)
	}
}

func CreateRepoPool(repoID string, poolParams params.CreatePoolParams) *params.Pool {
	log.Printf("Create repo %s pool", repoID)
	pool, err := createRepoPool(cli, authToken, repoID, poolParams)
	if err != nil {
		panic(err)
	}
	return pool
}

func GetRepoPool(repoID, repoPoolID string) *params.Pool {
	log.Printf("Get repo %s pool %s", repoID, repoPoolID)
	pool, err := getRepoPool(cli, authToken, repoID, repoPoolID)
	if err != nil {
		panic(err)
	}
	return pool
}

func UpdateRepoPool(repoID, repoPoolID string, maxRunners, minIdleRunners uint) *params.Pool {
	log.Printf("Update repo %s pool %s", repoID, repoPoolID)
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
	log.Printf("Delete repo %s pool %s", repoID, repoPoolID)
	if err := deleteRepoPool(cli, authToken, repoID, repoPoolID); err != nil {
		panic(err)
	}
}

func WaitRepoRunningIdleInstances(repoID string, timeout time.Duration) {
	repoPools, err := listRepoPools(cli, authToken, repoID)
	if err != nil {
		panic(err)
	}
	for _, pool := range repoPools {
		err := waitPoolRunningIdleInstances(pool.ID, timeout)
		if err != nil {
			_ = dumpRepoInstancesDetails(repoID)
			panic(err)
		}
	}
}

func dumpRepoInstancesDetails(repoID string) error {
	// print repo details
	log.Printf("Dumping repo %s details", repoID)
	repo, err := getRepo(cli, authToken, repoID)
	if err != nil {
		return err
	}
	if err := printJsonResponse(repo); err != nil {
		return err
	}

	// print repo instances details
	log.Printf("Dumping repo %s instances details", repoID)
	instances, err := listRepoInstances(cli, authToken, repoID)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		instance, err := getInstance(cli, authToken, instance.Name)
		if err != nil {
			return err
		}
		log.Printf("Instance %s info:", instance.Name)
		if err := printJsonResponse(instance); err != nil {
			return err
		}
	}
	return nil
}
