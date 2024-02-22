package e2e

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/cloudbase/garm/params"
)

func ListCredentials() params.Credentials {
	slog.Info("List credentials")
	credentials, err := listCredentials(cli, authToken)
	if err != nil {
		panic(err)
	}
	return credentials
}

func ListProviders() params.Providers {
	slog.Info("List providers")
	providers, err := listProviders(cli, authToken)
	if err != nil {
		panic(err)
	}
	return providers
}

func GetMetricsToken() {
	slog.Info("Get metrics token")
	_, err := getMetricsToken(cli, authToken)
	if err != nil {
		panic(err)
	}
}

func GetControllerInfo() *params.ControllerInfo {
	slog.Info("Get controller info")
	controllerInfo, err := getControllerInfo(cli, authToken)
	if err != nil {
		panic(err)
	}
	if err := appendCtrlInfoToGitHubEnv(&controllerInfo); err != nil {
		panic(err)
	}
	if err := printJSONResponse(controllerInfo); err != nil {
		panic(err)
	}
	return &controllerInfo
}

func GracefulCleanup() {
	// disable all the pools
	pools, err := listPools(cli, authToken)
	if err != nil {
		panic(err)
	}
	enabled := false
	poolParams := params.UpdatePoolParams{Enabled: &enabled}
	for _, pool := range pools {
		if _, err := updatePool(cli, authToken, pool.ID, poolParams); err != nil {
			panic(err)
		}
		slog.Info("Pool disabled", "pool_id", pool.ID)
	}

	// delete all the instances
	for _, pool := range pools {
		poolInstances, err := listPoolInstances(cli, authToken, pool.ID)
		if err != nil {
			panic(err)
		}
		for _, instance := range poolInstances {
			if err := deleteInstance(cli, authToken, instance.Name, false); err != nil {
				panic(err)
			}
			slog.Info("Instance deletion initiated", "instance", instance.Name)
		}
	}

	// wait for all instances to be deleted
	for _, pool := range pools {
		if err := waitPoolNoInstances(pool.ID, 3*time.Minute); err != nil {
			panic(err)
		}
	}

	// delete all the pools
	for _, pool := range pools {
		if err := deletePool(cli, authToken, pool.ID); err != nil {
			panic(err)
		}
		slog.Info("Pool deleted", "pool_id", pool.ID)
	}

	// delete all the repositories
	repos, err := listRepos(cli, authToken)
	if err != nil {
		panic(err)
	}
	for _, repo := range repos {
		if err := deleteRepo(cli, authToken, repo.ID); err != nil {
			panic(err)
		}
		slog.Info("Repo deleted", "repo_id", repo.ID)
	}

	// delete all the organizations
	orgs, err := listOrgs(cli, authToken)
	if err != nil {
		panic(err)
	}
	for _, org := range orgs {
		if err := deleteOrg(cli, authToken, org.ID); err != nil {
			panic(err)
		}
		slog.Info("Org deleted", "org_id", org.ID)
	}
}

func appendCtrlInfoToGitHubEnv(controllerInfo *params.ControllerInfo) error {
	envFile, found := os.LookupEnv("GITHUB_ENV")
	if !found {
		slog.Info("GITHUB_ENV not set, skipping appending controller info")
		return nil
	}
	file, err := os.OpenFile(envFile, os.O_WRONLY|os.O_APPEND, os.ModeAppend)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.WriteString(fmt.Sprintf("GARM_CONTROLLER_ID=%s\n", controllerInfo.ControllerID)); err != nil {
		return err
	}
	return nil
}
