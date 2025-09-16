// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package cmd

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/cloudbase/garm-provider-common/util"
	apiClientRepos "github.com/cloudbase/garm/client/repositories"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	repoOwner           string
	repoName            string
	repoEndpoint        string
	repoWebhookSecret   string
	repoCreds           string
	forgeType           string
	randomWebhookSecret bool
	insecureRepoWebhook bool
	keepRepoWebhook     bool
	installRepoWebhook  bool
	repoAgentMode       bool
)

// repositoryCmd represents the repository command
var repositoryCmd = &cobra.Command{
	Use:          "repository",
	Aliases:      []string{"repo"},
	SilenceUsage: true,
	Short:        "Manage repositories",
	Long: `Add, remove or update repositories for which we manage
self hosted runners.

This command allows you to define a new repository or manage an existing
repository for which the garm maintains pools of self hosted runners.`,
	Run: nil,
}

var repoWebhookCmd = &cobra.Command{
	Use:          "webhook",
	Short:        "Manage repository webhooks",
	Long:         `Manage repository webhooks.`,
	SilenceUsage: true,
	Run:          nil,
}

var repoWebhookInstallCmd = &cobra.Command{
	Use:          "install",
	Short:        "Install webhook",
	Long:         `Install webhook for a repository.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a repository ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		repoID, err := resolveRepository(args[0], repoEndpoint)
		if err != nil {
			return err
		}

		installWebhookReq := apiClientRepos.NewInstallRepoWebhookParams()
		installWebhookReq.RepoID = repoID
		installWebhookReq.Body.InsecureSSL = insecureRepoWebhook
		installWebhookReq.Body.WebhookEndpointType = params.WebhookEndpointDirect

		response, err := apiCli.Repositories.InstallRepoWebhook(installWebhookReq, authToken)
		if err != nil {
			return err
		}
		formatOneHookInfo(response.Payload)
		return nil
	},
}

var repoHookInfoShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show webhook info",
	Long:         `Show webhook info for a repository.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a repository ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		repoID, err := resolveRepository(args[0], repoEndpoint)
		if err != nil {
			return err
		}

		showWebhookInfoReq := apiClientRepos.NewGetRepoWebhookInfoParams()
		showWebhookInfoReq.RepoID = repoID

		response, err := apiCli.Repositories.GetRepoWebhookInfo(showWebhookInfoReq, authToken)
		if err != nil {
			return err
		}
		formatOneHookInfo(response.Payload)
		return nil
	},
}

var repoWebhookUninstallCmd = &cobra.Command{
	Use:          "uninstall",
	Short:        "Uninstall webhook",
	Long:         `Uninstall webhook for a repository.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a repository ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		repoID, err := resolveRepository(args[0], repoEndpoint)
		if err != nil {
			return err
		}

		uninstallWebhookReq := apiClientRepos.NewUninstallRepoWebhookParams()
		uninstallWebhookReq.RepoID = repoID

		err = apiCli.Repositories.UninstallRepoWebhook(uninstallWebhookReq, authToken)
		if err != nil {
			return err
		}
		return nil
	},
}

var repoAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add repository",
	Long:         `Add a new repository to the manager.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if randomWebhookSecret {
			secret, err := util.GetRandomString(32)
			if err != nil {
				return err
			}
			repoWebhookSecret = secret
		}

		newRepoReq := apiClientRepos.NewCreateRepoParams()
		newRepoReq.Body = params.CreateRepoParams{
			Owner:            repoOwner,
			Name:             repoName,
			WebhookSecret:    repoWebhookSecret,
			CredentialsName:  repoCreds,
			ForgeType:        params.EndpointType(forgeType),
			PoolBalancerType: params.PoolBalancerType(poolBalancerType),
			AgentMode:        repoAgentMode,
		}
		response, err := apiCli.Repositories.CreateRepo(newRepoReq, authToken)
		if err != nil {
			return err
		}

		if installRepoWebhook {
			installWebhookReq := apiClientRepos.NewInstallRepoWebhookParams()
			installWebhookReq.RepoID = response.Payload.ID
			installWebhookReq.Body.WebhookEndpointType = params.WebhookEndpointDirect

			_, err := apiCli.Repositories.InstallRepoWebhook(installWebhookReq, authToken)
			if err != nil {
				return err
			}
		}

		getRepoReq := apiClientRepos.NewGetRepoParams()
		getRepoReq.RepoID = response.Payload.ID
		repo, err := apiCli.Repositories.GetRepo(getRepoReq, authToken)
		if err != nil {
			return err
		}
		formatOneRepository(repo.Payload)
		return nil
	},
}

var repoListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List repositories",
	Long:         `List all configured repositories that are currently managed.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listReposReq := apiClientRepos.NewListReposParams()
		listReposReq.Name = &repoName
		listReposReq.Owner = &repoOwner
		listReposReq.Endpoint = &repoEndpoint
		response, err := apiCli.Repositories.ListRepos(listReposReq, authToken)
		if err != nil {
			return err
		}
		formatRepositories(response.Payload)
		return nil
	},
}

var repoUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update repository",
	Long:         `Update repository credentials or webhook secret.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a repo ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		repoID, err := resolveRepository(args[0], repoEndpoint)
		if err != nil {
			return err
		}

		updateReposReq := apiClientRepos.NewUpdateRepoParams()
		updateReposReq.Body = params.UpdateEntityParams{
			WebhookSecret:    repoWebhookSecret,
			CredentialsName:  repoCreds,
			PoolBalancerType: params.PoolBalancerType(poolBalancerType),
		}
		if cmd.Flags().Changed("agent-mode") {
			updateReposReq.Body.AgentMode = &repoAgentMode
		}
		updateReposReq.RepoID = repoID

		response, err := apiCli.Repositories.UpdateRepo(updateReposReq, authToken)
		if err != nil {
			return err
		}
		formatOneRepository(response.Payload)
		return nil
	},
}

var repoShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for one repository",
	Long:         `Displays detailed information about a single repository.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a repository ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		repoID, err := resolveRepository(args[0], repoEndpoint)
		if err != nil {
			return err
		}

		showRepoReq := apiClientRepos.NewGetRepoParams()
		showRepoReq.RepoID = repoID
		response, err := apiCli.Repositories.GetRepo(showRepoReq, authToken)
		if err != nil {
			return err
		}
		formatOneRepository(response.Payload)
		return nil
	},
}

var repoDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Removes one repository",
	Long:         `Delete one repository from the manager.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a repository ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		repoID, err := resolveRepository(args[0], repoEndpoint)
		if err != nil {
			return err
		}

		deleteRepoReq := apiClientRepos.NewDeleteRepoParams()
		deleteRepoReq.RepoID = repoID
		deleteRepoReq.KeepWebhook = &keepRepoWebhook
		if err := apiCli.Repositories.DeleteRepo(deleteRepoReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	repoAddCmd.Flags().StringVar(&repoOwner, "owner", "", "The owner of this repository")
	repoAddCmd.Flags().StringVar(&poolBalancerType, "pool-balancer-type", string(params.PoolBalancerTypeRoundRobin), "The balancing strategy to use when creating runners in pools matching requested labels.")
	repoAddCmd.Flags().StringVar(&repoName, "name", "", "The name of the repository")
	repoAddCmd.Flags().StringVar(&forgeType, "forge-type", "", "The forge type of the repository. Supported values: github, gitea.")
	repoAddCmd.Flags().StringVar(&repoWebhookSecret, "webhook-secret", "", "The webhook secret for this repository")
	repoAddCmd.Flags().StringVar(&repoCreds, "credentials", "", "Credentials name. See credentials list.")
	repoAddCmd.Flags().BoolVar(&randomWebhookSecret, "random-webhook-secret", false, "Generate a random webhook secret for this repository.")
	repoAddCmd.Flags().BoolVar(&installRepoWebhook, "install-webhook", false, "Install the webhook as part of the add operation.")
	repoAddCmd.Flags().BoolVar(&repoAgentMode, "agent-mode", false, "Enable agent mode for runners in this repository.")
	repoAddCmd.MarkFlagsMutuallyExclusive("webhook-secret", "random-webhook-secret")
	repoAddCmd.MarkFlagsOneRequired("webhook-secret", "random-webhook-secret")

	repoListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")
	repoListCmd.Flags().StringVarP(&repoName, "name", "n", "", "Exact repo name to filter by.")
	repoListCmd.Flags().StringVarP(&repoOwner, "owner", "o", "", "Exact repo owner to filter by.")
	repoListCmd.Flags().StringVarP(&repoEndpoint, "endpoint", "e", "", "Exact endpoint name to filter by.")

	repoAddCmd.MarkFlagRequired("credentials") //nolint
	repoAddCmd.MarkFlagRequired("owner")       //nolint
	repoAddCmd.MarkFlagRequired("name")        //nolint

	repoDeleteCmd.Flags().BoolVar(&keepRepoWebhook, "keep-webhook", false, "Do not delete any existing webhook when removing the repo from GARM.")
	repoDeleteCmd.Flags().StringVar(&repoEndpoint, "endpoint", "", "When using the name of the repo, the endpoint must be specified when multiple repositories with the same name exist.")

	repoShowCmd.Flags().StringVar(&repoEndpoint, "endpoint", "", "When using the name of the repo, the endpoint must be specified when multiple repositories with the same name exist.")

	repoUpdateCmd.Flags().StringVar(&repoWebhookSecret, "webhook-secret", "", "The webhook secret for this repository. If you update this secret, you will have to manually update the secret in GitHub as well.")
	repoUpdateCmd.Flags().StringVar(&repoCreds, "credentials", "", "Credentials name. See credentials list.")
	repoUpdateCmd.Flags().StringVar(&poolBalancerType, "pool-balancer-type", "", "The balancing strategy to use when creating runners in pools matching requested labels.")
	repoUpdateCmd.Flags().BoolVar(&repoAgentMode, "agent-mode", false, "Enable agent mode for runners in this repository.")
	repoUpdateCmd.Flags().StringVar(&repoEndpoint, "endpoint", "", "When using the name of the repo, the endpoint must be specified when multiple repositories with the same name exist.")

	repoWebhookInstallCmd.Flags().BoolVar(&insecureRepoWebhook, "insecure", false, "Ignore self signed certificate errors.")
	repoWebhookInstallCmd.Flags().StringVar(&repoEndpoint, "endpoint", "", "When using the name of the repo, the endpoint must be specified when multiple repositories with the same name exist.")

	repoWebhookUninstallCmd.Flags().StringVar(&repoEndpoint, "endpoint", "", "When using the name of the repo, the endpoint must be specified when multiple repositories with the same name exist.")

	repoHookInfoShowCmd.Flags().StringVar(&repoEndpoint, "endpoint", "", "When using the name of the repo, the endpoint must be specified when multiple repositories with the same name exist.")

	repoWebhookCmd.AddCommand(
		repoWebhookInstallCmd,
		repoWebhookUninstallCmd,
		repoHookInfoShowCmd,
	)

	repositoryCmd.AddCommand(
		repoListCmd,
		repoAddCmd,
		repoShowCmd,
		repoDeleteCmd,
		repoUpdateCmd,
		repoWebhookCmd,
	)

	rootCmd.AddCommand(repositoryCmd)
}

func formatRepositories(repos []params.Repository) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(repos)
		return
	}
	t := table.NewWriter()
	header := table.Row{"ID", "Owner", "Name", "Endpoint", "Credentials name", "Pool Balancer Type", "Forge type", "Pool mgr running"}
	if long {
		header = append(header, "Created At", "Updated At")
	}
	t.AppendHeader(header)
	for _, val := range repos {
		forgeType := val.Endpoint.EndpointType
		if forgeType == "" {
			forgeType = params.GithubEndpointType
		}
		row := table.Row{val.ID, val.Owner, val.Name, val.Endpoint.Name, val.GetCredentialsName(), val.GetBalancerType(), forgeType, val.PoolManagerStatus.IsRunning}
		if long {
			row = append(row, val.CreatedAt, val.UpdatedAt)
		}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneRepository(repo params.Repository) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(repo)
		return
	}
	t := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", repo.ID})
	t.AppendRow(table.Row{"Created At", repo.CreatedAt})
	t.AppendRow(table.Row{"Updated At", repo.UpdatedAt})
	t.AppendRow(table.Row{"Owner", repo.Owner})
	t.AppendRow(table.Row{"Name", repo.Name})
	t.AppendRow(table.Row{"Endpoint", repo.Endpoint.Name})
	t.AppendRow(table.Row{"Pool balancer type", repo.GetBalancerType()})
	t.AppendRow(table.Row{"Credentials", repo.GetCredentialsName()})
	t.AppendRow(table.Row{"Pool manager running", repo.PoolManagerStatus.IsRunning})
	if !repo.PoolManagerStatus.IsRunning {
		t.AppendRow(table.Row{"Failure reason", repo.PoolManagerStatus.FailureReason})
	}

	if len(repo.Pools) > 0 {
		for _, pool := range repo.Pools {
			t.AppendRow(table.Row{"Pools", pool.ID}, rowConfigAutoMerge)
		}
	}

	if len(repo.Events) > 0 {
		for _, event := range repo.Events {
			t.AppendRow(table.Row{"Events", fmt.Sprintf("%s %s: %s", event.CreatedAt.Format("2006-01-02T15:04:05"), strings.ToUpper(string(event.EventLevel)), event.Message)}, rowConfigAutoMerge)
		}
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})

	fmt.Println(t.Render())
}
