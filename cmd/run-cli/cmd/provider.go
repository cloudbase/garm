/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

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
				fmt.Println("provider list called")
				return fmt.Errorf("I failed :(")
			},
		})

	rootCmd.AddCommand(providerCmd)
}
