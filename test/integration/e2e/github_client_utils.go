package e2e

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-github/v54/github"
	"golang.org/x/oauth2"
)

func TriggerWorkflow(ghToken, orgName, repoName, workflowFileName, labelName string) {
	log.Printf("Trigger workflow with label %s", labelName)

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
	log.Printf("Cleanup Github runners, labelled with controller ID %s, from org %s", controllerID, orgName)

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
					log.Printf("Failed to remove organization runner %s: %v", orgRunner.GetName(), err)
					break
				}
				log.Printf("Removed organization runner %s", orgRunner.GetName())
				break
			}
		}
	}

	return nil
}

func GhRepoRunnersCleanup(ghToken, orgName, repoName, controllerID string) error {
	log.Printf("Cleanup Github runners, labelled with controller ID %s, from repo %s/%s", controllerID, orgName, repoName)

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
					log.Printf("Failed to remove repository runner %s: %v", repoRunner.GetName(), err)
					break
				}
				log.Printf("Removed repository runner %s", repoRunner.GetName())
				break
			}
		}
	}

	return nil
}

func getGithubClient(oauthToken string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: oauthToken})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}
