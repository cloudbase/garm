package main

import (
	"fmt"
	"log/slog"
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
		slog.Warn("Env variable GARM_CONTROLLER_ID is not set, skipping GitHub runners cleanup")
	}

	baseURL, baseUrlFound := os.LookupEnv("GARM_BASE_URL")
	if ctrlIdFound && baseUrlFound {
		webhookURL := fmt.Sprintf("%s/webhooks/%s", baseURL, controllerID)
		_ = e2e.GhOrgWebhookCleanup(ghToken, webhookURL, orgName)
		_ = e2e.GhRepoWebhookCleanup(ghToken, webhookURL, orgName, repoName)
	} else {
		slog.Warn("Env variables GARM_CONTROLLER_ID & GARM_BASE_URL are not set, skipping webhooks cleanup")
	}
}
