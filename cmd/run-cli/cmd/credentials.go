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

// credentialsCmd represents the credentials command
var credentialsCmd = &cobra.Command{
	Use:     "credentials",
	Aliases: []string{"creds"},
	Short:   "List configured credentials",
	Long: `List all available credentials configured in the service
config file.

Currently, github personal tokens are configured statically in the config file
of the runner-manager service. This command lists the names of those credentials,
which in turn can be used to define pools of runners withing repositories.`,
	Run: nil,
}

func init() {
	credentialsCmd.AddCommand(
		&cobra.Command{
			Use:          "list",
			Aliases:      []string{"ls"},
			Short:        "List configured github credentials",
			Long:         `List the names of the github personal access tokens availabe to the runner-manager.`,
			SilenceUsage: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				if needsInit {
					return needsInitError
				}

				creds, err := cli.ListCredentials()
				if err != nil {
					return err
				}
				formatGithubCredentials(creds)
				return nil
			},
		})

	rootCmd.AddCommand(credentialsCmd)
}

func formatGithubCredentials(creds []params.GithubCredentials) {
	t := table.NewWriter()
	header := table.Row{"Name", "Description"}
	t.AppendHeader(header)
	for _, val := range creds {
		t.AppendRow(table.Row{val.Name, val.Description})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}
