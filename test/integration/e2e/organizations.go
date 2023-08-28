package e2e

import (
	"log"
	"time"

	"github.com/cloudbase/garm/params"
)

func CreateOrg(orgName, credentialsName, orgWebhookSecret string) *params.Organization {
	log.Printf("Create org %s", orgName)
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
	log.Printf("Update org %s", id)
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
	log.Printf("Install org %s webhook", id)
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
	log.Printf("Uninstall org %s webhook", id)
	if err := uninstallOrgWebhook(cli, authToken, id); err != nil {
		panic(err)
	}
}

func CreateOrgPool(orgID string, poolParams params.CreatePoolParams) *params.Pool {
	log.Printf("Create org %s pool", orgID)
	pool, err := createOrgPool(cli, authToken, orgID, poolParams)
	if err != nil {
		panic(err)
	}
	return pool
}

func GetOrgPool(orgID, orgPoolID string) *params.Pool {
	log.Printf("Get org %s pool %s", orgID, orgPoolID)
	pool, err := getOrgPool(cli, authToken, orgID, orgPoolID)
	if err != nil {
		panic(err)
	}
	return pool
}

func UpdateOrgPool(orgID, orgPoolID string, maxRunners, minIdleRunners uint) *params.Pool {
	log.Printf("Update org %s pool %s", orgID, orgPoolID)
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
	log.Printf("Delete org %s pool %s", orgID, orgPoolID)
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
		err := waitPoolRunningIdleInstances(pool.ID, timeout)
		if err != nil {
			_ = dumpOrgInstancesDetails(orgID)
			panic(err)
		}
	}
}

func dumpOrgInstancesDetails(orgID string) error {
	// print org details
	log.Printf("Dumping org %s details", orgID)
	org, err := getOrg(cli, authToken, orgID)
	if err != nil {
		return err
	}
	if err := printJsonResponse(org); err != nil {
		return err
	}

	// print org instances details
	log.Printf("Dumping org %s instances details", orgID)
	instances, err := listOrgInstances(cli, authToken, orgID)
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
