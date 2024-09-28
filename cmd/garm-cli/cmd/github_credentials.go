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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiClientCreds "github.com/cloudbase/garm/client/credentials"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	credentialsName              string
	credentialsDescription       string
	credentialsOAuthToken        string
	credentialsAppInstallationID int64
	credentialsAppID             int64
	credentialsPrivateKeyPath    string
	credentialsType              string
	credentialsEndpoint          string
)

// credentialsCmd represents the credentials command
var credentialsCmd = &cobra.Command{
	Use:     "credentials",
	Aliases: []string{"creds"},
	Short:   "List configured credentials. This is an alias for the github credentials command.",
	Long: `List all available github credentials.

This command is an alias for the garm-cli github credentials command.`,
	Run: nil,
}

// githubCredentialsCmd represents the github credentials command
var githubCredentialsCmd = &cobra.Command{
	Use:     "credentials",
	Aliases: []string{"creds"},
	Short:   "Manage github credentials",
	Long: `Manage GitHub credentials stored in GARM.

This command allows you to add, update, list and delete GitHub credentials.`,
	Run: nil,
}

var githubCredentialsListCmd = &cobra.Command{
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
}

var githubCredentialsShowCmd = &cobra.Command{
	Use:          "show",
	Aliases:      []string{"get"},
	Short:        "Show details of a configured github credential",
	Long:         `Show the details of a configured github credential.`,
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
		showCredsReq := apiClientCreds.NewGetCredentialsParams().WithID(credID)
		response, err := apiCli.Credentials.GetCredentials(showCredsReq, authToken)
		if err != nil {
			return err
		}
		formatOneGithubCredential(response.Payload)
		return nil
	},
}

var githubCredentialsUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update a github credential",
	Long:         "Update a github credential",
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

		updateParams, err := parseCredentialsUpdateParams()
		if err != nil {
			return err
		}

		updateCredsReq := apiClientCreds.NewUpdateCredentialsParams().WithID(credID)
		updateCredsReq.Body = updateParams

		response, err := apiCli.Credentials.UpdateCredentials(updateCredsReq, authToken)
		if err != nil {
			return err
		}
		formatOneGithubCredential(response.Payload)
		return nil
	},
}

var githubCredentialsDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm"},
	Short:        "Delete a github credential",
	Long:         "Delete a github credential",
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

		deleteCredsReq := apiClientCreds.NewDeleteCredentialsParams().WithID(credID)
		if err := apiCli.Credentials.DeleteCredentials(deleteCredsReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

var githubCredentialsAddCmd = &cobra.Command{
	Use:          "add",
	Short:        "Add a github credential",
	Long:         "Add a github credential",
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) > 0 {
			return fmt.Errorf("too many arguments")
		}

		addParams, err := parseCredentialsAddParams()
		if err != nil {
			return err
		}

		addCredsReq := apiClientCreds.NewCreateCredentialsParams()
		addCredsReq.Body = addParams

		response, err := apiCli.Credentials.CreateCredentials(addCredsReq, authToken)
		if err != nil {
			return err
		}
		formatOneGithubCredential(response.Payload)
		return nil
	},
}

func init() {
	githubCredentialsUpdateCmd.Flags().StringVar(&credentialsName, "name", "", "Name of the credential")
	githubCredentialsUpdateCmd.Flags().StringVar(&credentialsDescription, "description", "", "Description of the credential")
	githubCredentialsUpdateCmd.Flags().StringVar(&credentialsOAuthToken, "pat-oauth-token", "", "If the credential is a personal access token, the OAuth token")
	githubCredentialsUpdateCmd.Flags().Int64Var(&credentialsAppInstallationID, "app-installation-id", 0, "If the credential is an app, the installation ID")
	githubCredentialsUpdateCmd.Flags().Int64Var(&credentialsAppID, "app-id", 0, "If the credential is an app, the app ID")
	githubCredentialsUpdateCmd.Flags().StringVar(&credentialsPrivateKeyPath, "private-key-path", "", "If the credential is an app, the path to the private key file")

	githubCredentialsUpdateCmd.MarkFlagsMutuallyExclusive("pat-oauth-token", "app-installation-id")
	githubCredentialsUpdateCmd.MarkFlagsMutuallyExclusive("pat-oauth-token", "app-id")
	githubCredentialsUpdateCmd.MarkFlagsMutuallyExclusive("pat-oauth-token", "private-key-path")
	githubCredentialsUpdateCmd.MarkFlagsRequiredTogether("app-installation-id", "app-id", "private-key-path")

	githubCredentialsAddCmd.Flags().StringVar(&credentialsName, "name", "", "Name of the credential")
	githubCredentialsAddCmd.Flags().StringVar(&credentialsDescription, "description", "", "Description of the credential")
	githubCredentialsAddCmd.Flags().StringVar(&credentialsOAuthToken, "pat-oauth-token", "", "If the credential is a personal access token, the OAuth token")
	githubCredentialsAddCmd.Flags().Int64Var(&credentialsAppInstallationID, "app-installation-id", 0, "If the credential is an app, the installation ID")
	githubCredentialsAddCmd.Flags().Int64Var(&credentialsAppID, "app-id", 0, "If the credential is an app, the app ID")
	githubCredentialsAddCmd.Flags().StringVar(&credentialsPrivateKeyPath, "private-key-path", "", "If the credential is an app, the path to the private key file")
	githubCredentialsAddCmd.Flags().StringVar(&credentialsType, "auth-type", "", "The type of the credential")
	githubCredentialsAddCmd.Flags().StringVar(&credentialsEndpoint, "endpoint", "", "The endpoint to associate the credential with")

	githubCredentialsAddCmd.MarkFlagsMutuallyExclusive("pat-oauth-token", "app-installation-id")
	githubCredentialsAddCmd.MarkFlagsMutuallyExclusive("pat-oauth-token", "app-id")
	githubCredentialsAddCmd.MarkFlagsMutuallyExclusive("pat-oauth-token", "private-key-path")
	githubCredentialsAddCmd.MarkFlagsRequiredTogether("app-installation-id", "app-id", "private-key-path")

	githubCredentialsAddCmd.MarkFlagRequired("name")
	githubCredentialsAddCmd.MarkFlagRequired("auth-type")
	githubCredentialsAddCmd.MarkFlagRequired("description")
	githubCredentialsAddCmd.MarkFlagRequired("endpoint")

	githubCredentialsCmd.AddCommand(
		githubCredentialsListCmd,
		githubCredentialsShowCmd,
		githubCredentialsUpdateCmd,
		githubCredentialsDeleteCmd,
		githubCredentialsAddCmd,
	)
	githubCmd.AddCommand(githubCredentialsCmd)

	credentialsCmd.AddCommand(githubCredentialsListCmd)
	rootCmd.AddCommand(credentialsCmd)
}

func parsePrivateKeyFromPath(path string) ([]byte, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("private key file not found: %s", credentialsPrivateKeyPath)
	}
	keyContents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}
	pemBlock, _ := pem.Decode(keyContents)
	if pemBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	if _, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes); err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return keyContents, nil
}

func parseCredentialsAddParams() (ret params.CreateGithubCredentialsParams, err error) {
	ret.Name = credentialsName
	ret.Description = credentialsDescription
	ret.AuthType = params.GithubAuthType(credentialsType)
	ret.Endpoint = credentialsEndpoint
	switch ret.AuthType {
	case params.GithubAuthTypePAT:
		ret.PAT.OAuth2Token = credentialsOAuthToken
	case params.GithubAuthTypeApp:
		ret.App.InstallationID = credentialsAppInstallationID
		ret.App.AppID = credentialsAppID
		keyContents, err := parsePrivateKeyFromPath(credentialsPrivateKeyPath)
		if err != nil {
			return params.CreateGithubCredentialsParams{}, err
		}
		ret.App.PrivateKeyBytes = keyContents
	default:
		return params.CreateGithubCredentialsParams{}, fmt.Errorf("invalid auth type: %s (supported are: app, pat)", credentialsType)
	}

	return ret, nil
}

func parseCredentialsUpdateParams() (params.UpdateGithubCredentialsParams, error) {
	var updateParams params.UpdateGithubCredentialsParams

	if credentialsAppInstallationID != 0 || credentialsAppID != 0 || credentialsPrivateKeyPath != "" {
		updateParams.App = &params.GithubApp{}
	}

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

	if credentialsAppInstallationID != 0 {
		updateParams.App.InstallationID = credentialsAppInstallationID
	}

	if credentialsAppID != 0 {
		updateParams.App.AppID = credentialsAppID
	}

	if credentialsPrivateKeyPath != "" {
		keyContents, err := parsePrivateKeyFromPath(credentialsPrivateKeyPath)
		if err != nil {
			return params.UpdateGithubCredentialsParams{}, err
		}
		updateParams.App.PrivateKeyBytes = keyContents
	}

	return updateParams, nil
}

func formatGithubCredentials(creds []params.GithubCredentials) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(creds)
		return
	}
	t := table.NewWriter()
	header := table.Row{"ID", "Name", "Description", "Base URL", "API URL", "Upload URL", "Type"}
	t.AppendHeader(header)
	for _, val := range creds {
		t.AppendRow(table.Row{val.ID, val.Name, val.Description, val.BaseURL, val.APIBaseURL, val.UploadBaseURL, val.AuthType})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneGithubCredential(cred params.GithubCredentials) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(cred)
		return
	}
	t := table.NewWriter()
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)

	t.AppendRow(table.Row{"ID", cred.ID})
	t.AppendRow(table.Row{"Name", cred.Name})
	t.AppendRow(table.Row{"Description", cred.Description})
	t.AppendRow(table.Row{"Base URL", cred.BaseURL})
	t.AppendRow(table.Row{"API URL", cred.APIBaseURL})
	t.AppendRow(table.Row{"Upload URL", cred.UploadBaseURL})
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

	if len(cred.Enterprises) > 0 {
		t.AppendRow(table.Row{"", ""})
		for _, ent := range cred.Enterprises {
			t.AppendRow(table.Row{"Enterprises", ent.Name})
		}
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}
