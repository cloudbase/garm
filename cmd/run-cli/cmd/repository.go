/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"runner-manager/params"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var (
	repoOwner         string
	repoName          string
	repoWebhookSecret string
	repoCreds         string
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
repository for which the runner-manager maintains pools of self hosted runners.`,
	Run: nil,
}

var repoAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add repository",
	Long:         `Add a new repository to the manager.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}

		newRepoReq := params.CreateRepoParams{
			Owner:           repoOwner,
			Name:            repoName,
			WebhookSecret:   repoWebhookSecret,
			CredentialsName: repoCreds,
		}
		repo, err := cli.CreateRepository(newRepoReq)
		if err != nil {
			return err
		}
		formatOneRepository(repo)
		return nil
	},
}

var repoListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List repositories",
	Long:         `List all configured respositories that are currently managed.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}

		repos, err := cli.ListRepositories()
		if err != nil {
			return err
		}
		formatRepositories(repos)
		return nil
	},
}

var repoShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for one repository",
	Long:         `Displays detailed information about a single repository.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a repository ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		repo, err := cli.GetRepository(args[0])
		if err != nil {
			return err
		}
		formatOneRepository(repo)
		return nil
	},
}

var repoDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Removes one repository",
	Long:         `Delete one repository from the manager.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a repository ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		if err := cli.DeleteRepository(args[0]); err != nil {
			return err
		}
		return nil
	},
}

func init() {

	repoAddCmd.Flags().StringVar(&repoOwner, "owner", "", "The owner of this repository")
	repoAddCmd.Flags().StringVar(&repoName, "name", "", "The name of the repository")
	repoAddCmd.Flags().StringVar(&repoWebhookSecret, "webhook-secret", "", "The webhook secret for this repository")
	repoAddCmd.Flags().StringVar(&repoCreds, "credentials", "", "Credentials name. See credentials list.")
	repoAddCmd.MarkFlagRequired("credentials")
	repoAddCmd.MarkFlagRequired("owner")
	repoAddCmd.MarkFlagRequired("name")

	repositoryCmd.AddCommand(
		repoListCmd,
		repoAddCmd,
		repoShowCmd,
		repoDeleteCmd,
	)

	rootCmd.AddCommand(repositoryCmd)
}

func formatRepositories(repos []params.Repository) {
	t := table.NewWriter()
	header := table.Row{"ID", "Owner", "Name", "Credentials name"}
	t.AppendHeader(header)
	for _, val := range repos {
		t.AppendRow(table.Row{val.ID, val.Owner, val.Name, val.CredentialsName})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneRepository(repo params.Repository) {
	t := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", repo.ID})
	t.AppendRow(table.Row{"Owner", repo.Owner})
	t.AppendRow(table.Row{"Name", repo.Name})
	t.AppendRow(table.Row{"Credentials", repo.CredentialsName})

	if len(repo.Pools) > 0 {
		for _, pool := range repo.Pools {
			t.AppendRow(table.Row{"Pools", pool.ID}, rowConfigAutoMerge)
		}
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: true},
	})

	fmt.Println(t.Render())
}
