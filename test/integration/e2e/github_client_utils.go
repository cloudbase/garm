package e2e

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

func TriggerWorkflow(ghToken, orgName, repoName, workflowFileName, labelName string) {
	slog.Info("Trigger workflow", "label", labelName)

	client := getGithubClient(ghToken)
	eventReq := github.CreateWorkflowDispatchEventRequest{
		Ref: "main",
		Inputs: map[string]interface{}{
			"sleep_time":   "50",
			"runner_label": labelName,
		},
	}
	if _, err := client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), orgName, repoName, workflowFileName, eventReq); err != nil {
		panic(err)
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

func ValidateOrgWebhookInstalled(ghToken, url, orgName string) {
	hook, err := getGhOrgWebhook(url, ghToken, orgName)
	if err != nil {
		panic(err)
	}
	if hook == nil {
		panic(fmt.Errorf("github webhook with url %s, for org %s was not properly installed", url, orgName))
	}
}

func ValidateOrgWebhookUninstalled(ghToken, url, orgName string) {
	hook, err := getGhOrgWebhook(url, ghToken, orgName)
	if err != nil {
		panic(err)
	}
	if hook != nil {
		panic(fmt.Errorf("github webhook with url %s, for org %s was not properly uninstalled", url, orgName))
	}
}

func ValidateRepoWebhookInstalled(ghToken, url, orgName, repoName string) {
	hook, err := getGhRepoWebhook(url, ghToken, orgName, repoName)
	if err != nil {
		panic(err)
	}
	if hook == nil {
		panic(fmt.Errorf("github webhook with url %s, for repo %s/%s was not properly installed", url, orgName, repoName))
	}
}

func ValidateRepoWebhookUninstalled(ghToken, url, orgName, repoName string) {
	hook, err := getGhRepoWebhook(url, ghToken, orgName, repoName)
	if err != nil {
		panic(err)
	}
	if hook != nil {
		panic(fmt.Errorf("github webhook with url %s, for repo %s/%s was not properly uninstalled", url, orgName, repoName))
	}
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
