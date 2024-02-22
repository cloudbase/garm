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

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiClientCreds "github.com/cloudbase/garm/client/credentials"
	"github.com/cloudbase/garm/params"
)

// credentialsCmd represents the credentials command
var credentialsCmd = &cobra.Command{
	Use:     "credentials",
	Aliases: []string{"creds"},
	Short:   "List configured credentials",
	Long: `List all available credentials configured in the service
config file.

Currently, github personal tokens are configured statically in the config file
of the garm service. This command lists the names of those credentials,
which in turn can be used to define pools of runners within repositories.`,
	Run: nil,
}

func init() {
	credentialsCmd.AddCommand(
		&cobra.Command{
			Use:          "list",
			Aliases:      []string{"ls"},
			Short:        "List configured github credentials",
			Long:         `List the names of the github personal access tokens available to the garm.`,
			SilenceUsage: true,
			RunE: func(_ *cobra.Command, _ []string) error {
				if needsInit {
					return errNeedsInitError
				}

				listCredsReq := apiClientCreds.NewListCredentialsParams()
				response, err := apiCli.Credentials.ListCredentials(listCredsReq, authToken)
				if err != nil {
					return err
				}
				formatGithubCredentials(response.Payload)
				return nil
			},
		})

	rootCmd.AddCommand(credentialsCmd)
}

func formatGithubCredentials(creds []params.GithubCredentials) {
	t := table.NewWriter()
	header := table.Row{"Name", "Description", "Base URL", "API URL", "Upload URL"}
	t.AppendHeader(header)
	for _, val := range creds {
		t.AppendRow(table.Row{val.Name, val.Description, val.BaseURL, val.APIBaseURL, val.UploadBaseURL})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}
