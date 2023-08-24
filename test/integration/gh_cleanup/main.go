package main

import (
	"log"
	"os"

	"github.com/cloudbase/garm/test/integration/e2e"
)

var (
	orgName  = os.Getenv("ORG_NAME")
	repoName = os.Getenv("REPO_NAME")

	ghToken = os.Getenv("GH_TOKEN")
)

func main() {
	controllerID, ctrlIdFound := os.LookupEnv("GARM_CONTROLLER_ID")
	if ctrlIdFound {
		_ = e2e.GhOrgRunnersCleanup(ghToken, orgName, controllerID)
		_ = e2e.GhRepoRunnersCleanup(ghToken, orgName, repoName, controllerID)
	} else {
		log.Println("Env variable GARM_CONTROLLER_ID is not set, skipping GitHub runners cleanup")
	}

	baseURL, baseUrlFound := os.LookupEnv("GARM_BASE_URL")
	if ctrlIdFound && baseUrlFound {
		log.Printf("TODO: Cleanup org & repo webhooks staring with: %s/webhooks/%s", baseURL, controllerID)
		// TODO: Cleanup org webhooks that start with "{baseURL}/webhooks/{controllerID}"
		// TODO: Cleanup repo webhooks that start with "{baseURL}/webhooks/{controllerID}"
	} else {
		log.Println("Env variables GARM_CONTROLLER_ID & GARM_BASE_URL are not set, skipping webhooks cleanup")
	}
}
