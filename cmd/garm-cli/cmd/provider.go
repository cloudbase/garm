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

	"github.com/cloudbase/garm/params"

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
					return errNeedsInitError
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
