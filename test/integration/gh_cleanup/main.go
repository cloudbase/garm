package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

var (
	orgName  = os.Getenv("ORG_NAME")
	repoName = os.Getenv("REPO_NAME")

	ghToken = os.Getenv("GH_TOKEN")
)

func main() {
	controllerID, ctrlIDFound := os.LookupEnv("GARM_CONTROLLER_ID")
	if ctrlIDFound {
		_ = GhOrgRunnersCleanup(ghToken, orgName, controllerID)
		_ = GhRepoRunnersCleanup(ghToken, orgName, repoName, controllerID)
	} else {
		slog.Warn("Env variable GARM_CONTROLLER_ID is not set, skipping GitHub runners cleanup")
	}

	baseURL, baseURLFound := os.LookupEnv("GARM_BASE_URL")
	if ctrlIDFound && baseURLFound {
		webhookURL := fmt.Sprintf("%s/webhooks/%s", baseURL, controllerID)
		_ = GhOrgWebhookCleanup(ghToken, webhookURL, orgName)
		_ = GhRepoWebhookCleanup(ghToken, webhookURL, orgName, repoName)
	} else {
		slog.Warn("Env variables GARM_CONTROLLER_ID & GARM_BASE_URL are not set, skipping webhooks cleanup")
	}
}

func GhOrgRunnersCleanup(ghToken, orgName, controllerID string) error {
	slog.Info("Cleanup Github runners", "controller_id", controllerID, "org_name", orgName)

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
					slog.With(slog.Any("error", err)).Info("Failed to remove organization runner", "org_runner", orgRunner.GetName())
					break
				}
				slog.Info("Removed organization runner", "org_runner", orgRunner.GetName())
				break
			}
		}
	}

	return nil
}

func GhRepoRunnersCleanup(ghToken, orgName, repoName, controllerID string) error {
	slog.Info("Cleanup Github runners", "controller_id", controllerID, "org_name", orgName, "repo_name", repoName)

	client := getGithubClient(ghToken)
	ghRepoRunners, _, err := client.Actions.ListRunners(context.Background(), orgName, repoName, nil)
	if err != nil {
		return err
	}

	// Remove repository runners
	controllerLabel := fmt.Sprintf("runner-controller-id:%s", controllerID)
	for _, repoRunner := range ghRepoRunners.Runners {
		for _, label := range repoRunner.Labels {
			if label.GetName() == controllerLabel {
				if _, err := client.Actions.RemoveRunner(context.Background(), orgName, repoName, repoRunner.GetID()); err != nil {
					// We don't fail if we can't remove a single runner. This
					// is a best effort to try and remove all the orphan runners.
					slog.With(slog.Any("error", err)).Error("Failed to remove repository runner", "runner_name", repoRunner.GetName())
					break
				}
				slog.Info("Removed repository runner", "runner_name", repoRunner.GetName())
				break
			}
		}
	}

	return nil
}

func GhOrgWebhookCleanup(ghToken, webhookURL, orgName string) error {
	slog.Info("Cleanup Github webhook", "webhook_url", webhookURL, "org_name", orgName)
	hook, err := getGhOrgWebhook(webhookURL, ghToken, orgName)
	if err != nil {
		return err
	}

	// Remove organization webhook
	if hook != nil {
		client := getGithubClient(ghToken)
		if _, err := client.Organizations.DeleteHook(context.Background(), orgName, hook.GetID()); err != nil {
			return err
		}
		slog.Info("Github webhook removed", "webhook_url", webhookURL, "org_name", orgName)
	}

	return nil
}

func GhRepoWebhookCleanup(ghToken, webhookURL, orgName, repoName string) error {
	slog.Info("Cleanup Github webhook", "webhook_url", webhookURL, "org_name", orgName, "repo_name", repoName)

	hook, err := getGhRepoWebhook(webhookURL, ghToken, orgName, repoName)
	if err != nil {
		return err
	}

	// Remove repository webhook
	if hook != nil {
		client := getGithubClient(ghToken)
		if _, err := client.Repositories.DeleteHook(context.Background(), orgName, repoName, hook.GetID()); err != nil {
			return err
		}
		slog.Info("Github webhook with", "webhook_url", webhookURL, "org_name", orgName, "repo_name", repoName)
	}

	return nil
}

func getGhOrgWebhook(url, ghToken, orgName string) (*github.Hook, error) {
	client := getGithubClient(ghToken)
	ghOrgHooks, _, err := client.Organizations.ListHooks(context.Background(), orgName, nil)
	if err != nil {
		return nil, err
	}

	for _, hook := range ghOrgHooks {
		hookURL, ok := hook.Config["url"].(string)
		if ok && hookURL == url {
			return hook, nil
		}
	}

	return nil, nil
}

func getGhRepoWebhook(url, ghToken, orgName, repoName string) (*github.Hook, error) {
	client := getGithubClient(ghToken)
	ghRepoHooks, _, err := client.Repositories.ListHooks(context.Background(), orgName, repoName, nil)
	if err != nil {
		return nil, err
	}

	for _, hook := range ghRepoHooks {
		hookURL, ok := hook.Config["url"].(string)
		if ok && hookURL == url {
			return hook, nil
		}
	}

	return nil, nil
}

func getGithubClient(oauthToken string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: oauthToken})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}
