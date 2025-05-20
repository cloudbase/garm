// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.

package cmd

import (
	"fmt"
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiClientCreds "github.com/cloudbase/garm/client/credentials"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

// giteaCredentialsCmd represents the gitea credentials command
var giteaCredentialsCmd = &cobra.Command{
	Use:     "credentials",
	Aliases: []string{"creds"},
	Short:   "Manage gitea credentials",
	Long: `Manage Gitea credentials stored in GARM.

This command allows you to add, update, list and delete Gitea credentials.`,
	Run: nil,
}

var giteaCredentialsListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List configured gitea credentials",
	Long:         `List the names of the gitea personal access tokens available to the garm.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listCredsReq := apiClientCreds.NewListGiteaCredentialsParams()
		response, err := apiCli.Credentials.ListGiteaCredentials(listCredsReq, authToken)
		if err != nil {
			return err
		}
		formatGiteaCredentials(response.Payload)
		return nil
	},
}

var giteaCredentialsShowCmd = &cobra.Command{
	Use:          "show",
	Aliases:      []string{"get"},
	Short:        "Show details of a configured gitea credential",
	Long:         `Show the details of a configured gitea credential.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) < 1 {
			return fmt.Errorf("missing required argument: credential ID")
		}

		credID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid credential ID: %s", args[0])
		}
		showCredsReq := apiClientCreds.NewGetGiteaCredentialsParams().WithID(credID)
		response, err := apiCli.Credentials.GetGiteaCredentials(showCredsReq, authToken)
		if err != nil {
			return err
		}
		formatOneGiteaCredential(response.Payload)
		return nil
	},
}

var giteaCredentialsUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update a gitea credential",
	Long:         "Update a gitea credential",
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) < 1 {
			return fmt.Errorf("missing required argument: credential ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		credID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid credential ID: %s", args[0])
		}

		updateParams, err := parseGiteaCredentialsUpdateParams()
		if err != nil {
			return err
		}

		updateCredsReq := apiClientCreds.NewUpdateGiteaCredentialsParams().WithID(credID)
		updateCredsReq.Body = updateParams

		response, err := apiCli.Credentials.UpdateGiteaCredentials(updateCredsReq, authToken)
		if err != nil {
			return err
		}
		formatOneGiteaCredential(response.Payload)
		return nil
	},
}

var giteaCredentialsDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm"},
	Short:        "Delete a gitea credential",
	Long:         "Delete a gitea credential",
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) < 1 {
			return fmt.Errorf("missing required argument: credential ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		credID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid credential ID: %s", args[0])
		}

		deleteCredsReq := apiClientCreds.NewDeleteGiteaCredentialsParams().WithID(credID)
		if err := apiCli.Credentials.DeleteGiteaCredentials(deleteCredsReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

var giteaCredentialsAddCmd = &cobra.Command{
	Use:          "add",
	Short:        "Add a gitea credential",
	Long:         "Add a gitea credential",
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) > 0 {
			return fmt.Errorf("too many arguments")
		}

		addParams, err := parseGiteaCredentialsAddParams()
		if err != nil {
			return err
		}

		addCredsReq := apiClientCreds.NewCreateGiteaCredentialsParams()
		addCredsReq.Body = addParams

		response, err := apiCli.Credentials.CreateGiteaCredentials(addCredsReq, authToken)
		if err != nil {
			return err
		}
		formatOneGiteaCredential(response.Payload)
		return nil
	},
}

func init() {
	giteaCredentialsUpdateCmd.Flags().StringVar(&credentialsName, "name", "", "Name of the credential")
	giteaCredentialsUpdateCmd.Flags().StringVar(&credentialsDescription, "description", "", "Description of the credential")
	giteaCredentialsUpdateCmd.Flags().StringVar(&credentialsOAuthToken, "pat-oauth-token", "", "If the credential is a personal access token, the OAuth token")

	giteaCredentialsListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")

	giteaCredentialsAddCmd.Flags().StringVar(&credentialsName, "name", "", "Name of the credential")
	giteaCredentialsAddCmd.Flags().StringVar(&credentialsDescription, "description", "", "Description of the credential")
	giteaCredentialsAddCmd.Flags().StringVar(&credentialsOAuthToken, "pat-oauth-token", "", "If the credential is a personal access token, the OAuth token")
	giteaCredentialsAddCmd.Flags().StringVar(&credentialsType, "auth-type", "", "The type of the credential")
	giteaCredentialsAddCmd.Flags().StringVar(&credentialsEndpoint, "endpoint", "", "The endpoint to associate the credential with")

	giteaCredentialsAddCmd.MarkFlagRequired("name")
	giteaCredentialsAddCmd.MarkFlagRequired("auth-type")
	giteaCredentialsAddCmd.MarkFlagRequired("description")
	giteaCredentialsAddCmd.MarkFlagRequired("endpoint")

	giteaCredentialsCmd.AddCommand(
		giteaCredentialsListCmd,
		giteaCredentialsShowCmd,
		giteaCredentialsUpdateCmd,
		giteaCredentialsDeleteCmd,
		giteaCredentialsAddCmd,
	)
	giteaCmd.AddCommand(giteaCredentialsCmd)
}

func parseGiteaCredentialsAddParams() (ret params.CreateGiteaCredentialsParams, err error) {
	ret.Name = credentialsName
	ret.Description = credentialsDescription
	ret.AuthType = params.ForgeAuthType(credentialsType)
	ret.Endpoint = credentialsEndpoint
	switch ret.AuthType {
	case params.ForgeAuthTypePAT:
		ret.PAT.OAuth2Token = credentialsOAuthToken
	default:
		return params.CreateGiteaCredentialsParams{}, fmt.Errorf("invalid auth type: %s (supported are: pat)", credentialsType)
	}

	return ret, nil
}

func parseGiteaCredentialsUpdateParams() (params.UpdateGiteaCredentialsParams, error) {
	var updateParams params.UpdateGiteaCredentialsParams

	if credentialsName != "" {
		updateParams.Name = &credentialsName
	}

	if credentialsDescription != "" {
		updateParams.Description = &credentialsDescription
	}

	if credentialsOAuthToken != "" {
		if updateParams.PAT == nil {
			updateParams.PAT = &params.GithubPAT{}
		}
		updateParams.PAT.OAuth2Token = credentialsOAuthToken
	}

	return updateParams, nil
}

func formatGiteaCredentials(creds []params.ForgeCredentials) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(creds)
		return
	}
	t := table.NewWriter()
	header := table.Row{"ID", "Name", "Description", "Base URL", "API URL", "Type"}
	if long {
		header = append(header, "Created At", "Updated At")
	}
	t.AppendHeader(header)
	for _, val := range creds {
		row := table.Row{val.ID, val.Name, val.Description, val.BaseURL, val.APIBaseURL, val.AuthType}
		if long {
			row = append(row, val.CreatedAt, val.UpdatedAt)
		}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneGiteaCredential(cred params.ForgeCredentials) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(cred)
		return
	}
	t := table.NewWriter()
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)

	t.AppendRow(table.Row{"ID", cred.ID})
	t.AppendRow(table.Row{"Created At", cred.CreatedAt})
	t.AppendRow(table.Row{"Updated At", cred.UpdatedAt})
	t.AppendRow(table.Row{"Name", cred.Name})
	t.AppendRow(table.Row{"Description", cred.Description})
	t.AppendRow(table.Row{"Base URL", cred.BaseURL})
	t.AppendRow(table.Row{"API URL", cred.APIBaseURL})
	t.AppendRow(table.Row{"Type", cred.AuthType})
	t.AppendRow(table.Row{"Endpoint", cred.Endpoint.Name})

	if len(cred.Repositories) > 0 {
		t.AppendRow(table.Row{"", ""})
		for _, repo := range cred.Repositories {
			t.AppendRow(table.Row{"Repositories", repo.String()})
		}
	}

	if len(cred.Organizations) > 0 {
		t.AppendRow(table.Row{"", ""})
		for _, org := range cred.Organizations {
			t.AppendRow(table.Row{"Organizations", org.Name})
		}
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}
