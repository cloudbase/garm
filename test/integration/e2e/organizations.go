package e2e

import (
	"log/slog"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

func CreateOrg(orgName, credentialsName, orgWebhookSecret string) *params.Organization {
	slog.Info("Create org", "org_name", orgName)
	orgParams := params.CreateOrgParams{
		Name:            orgName,
		CredentialsName: credentialsName,
		WebhookSecret:   orgWebhookSecret,
	}
	org, err := createOrg(cli, authToken, orgParams)
	if err != nil {
		panic(err)
	}
	return org
}

func UpdateOrg(id, credentialsName string) *params.Organization {
	slog.Info("Update org", "org_id", id)
	updateParams := params.UpdateEntityParams{
		CredentialsName: credentialsName,
	}
	org, err := updateOrg(cli, authToken, id, updateParams)
	if err != nil {
		panic(err)
	}
	return org
}

func InstallOrgWebhook(id string) *params.HookInfo {
	slog.Info("Install org webhook", "org_id", id)
	webhookParams := params.InstallWebhookParams{
		WebhookEndpointType: params.WebhookEndpointDirect,
	}
	_, err := installOrgWebhook(cli, authToken, id, webhookParams)
	if err != nil {
		panic(err)
	}
	webhookInfo, err := getOrgWebhook(cli, authToken, id)
	if err != nil {
		panic(err)
	}
	return webhookInfo
}

func UninstallOrgWebhook(id string) {
	slog.Info("Uninstall org webhook", "org_id", id)
	if err := uninstallOrgWebhook(cli, authToken, id); err != nil {
		panic(err)
	}
}

func CreateOrgPool(orgID string, poolParams params.CreatePoolParams) *params.Pool {
	slog.Info("Create org pool", "org_id", orgID)
	pool, err := createOrgPool(cli, authToken, orgID, poolParams)
	if err != nil {
		panic(err)
	}
	return pool
}

func GetOrgPool(orgID, orgPoolID string) *params.Pool {
	slog.Info("Get org pool", "org_id", orgID, "pool_id", orgPoolID)
	pool, err := getOrgPool(cli, authToken, orgID, orgPoolID)
	if err != nil {
		panic(err)
	}
	return pool
}

func UpdateOrgPool(orgID, orgPoolID string, maxRunners, minIdleRunners uint) *params.Pool {
	slog.Info("Update org pool", "org_id", orgID, "pool_id", orgPoolID)
	poolParams := params.UpdatePoolParams{
		MinIdleRunners: &minIdleRunners,
		MaxRunners:     &maxRunners,
	}
	pool, err := updateOrgPool(cli, authToken, orgID, orgPoolID, poolParams)
	if err != nil {
		panic(err)
	}
	return pool
}

func DeleteOrgPool(orgID, orgPoolID string) {
	slog.Info("Delete org pool", "org_id", orgID, "pool_id", orgPoolID)
	if err := deleteOrgPool(cli, authToken, orgID, orgPoolID); err != nil {
		panic(err)
	}
}

func WaitOrgRunningIdleInstances(orgID string, timeout time.Duration) {
	orgPools, err := listOrgPools(cli, authToken, orgID)
	if err != nil {
		panic(err)
	}
	for _, pool := range orgPools {
		err := WaitPoolInstances(pool.ID, commonParams.InstanceRunning, params.RunnerIdle, timeout)
		if err != nil {
			_ = dumpOrgInstancesDetails(orgID)
			panic(err)
		}
	}
}

func dumpOrgInstancesDetails(orgID string) error {
	// print org details
	slog.Info("Dumping org details", "org_id", orgID)
	org, err := getOrg(cli, authToken, orgID)
	if err != nil {
		return err
	}
	if err := printJsonResponse(org); err != nil {
		return err
	}

	// print org instances details
	slog.Info("Dumping org instances details", "org_id", orgID)
	instances, err := listOrgInstances(cli, authToken, orgID)
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
