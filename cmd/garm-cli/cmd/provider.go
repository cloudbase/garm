/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"garm/params"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

// providerCmd represents the provider command
var providerCmd = &cobra.Command{
	Use:          "provider",
	SilenceUsage: true,
	Short:        "Interacts with the providers API resource.",
	Long: `Run operations on the provider resource.

Currently this command only lists all available configured
providers. Providers are added to the configuration file of
the service and are referenced by name when adding repositories
and organizations. Runners will be created in these environments.`,
	Run: nil,
}

func init() {
	providerCmd.AddCommand(
		&cobra.Command{
			Use:          "list",
			Short:        "List all configured providers",
			Long:         `List all cloud providers configured with the service.`,
			SilenceUsage: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				if needsInit {
					return needsInitError
				}

				providers, err := cli.ListProviders()
				if err != nil {
					return err
				}
				formatProviders(providers)
				return nil
			},
		})

	rootCmd.AddCommand(providerCmd)
}

func formatProviders(providers []params.Provider) {
	t := table.NewWriter()
	header := table.Row{"Name", "Description", "Type"}
	t.AppendHeader(header)
	for _, val := range providers {
		t.AppendRow(table.Row{val.Name, val.Description, val.ProviderType})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}
