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

	apiClientLogin "github.com/cloudbase/garm/client/login"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/cmd/garm-cli/config"
	"github.com/cloudbase/garm/params"
)

var (
	loginProfileName string
	loginURL         string
	loginPassword    string
	loginUserName    string
	loginFullName    string
	loginEmail       string
)

// runnerCmd represents the runner command
var profileCmd = &cobra.Command{
	Use:          "profile",
	SilenceUsage: false,
	Short:        "Add, delete or update profiles",
	Long:         `Creates, deletes or updates bearer tokens for profiles.`,
	Run:          nil,
}

var profileListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List profiles",
	Long: `List profiles.

This command will list all currently defined profiles in the local configuration
file of the garm client.
`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if cfg == nil {
			return nil
		}

		formatProfiles(cfg.Managers)

		return nil
	},
}

var profileDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Delete profile",
	Long:         `Delete a profile from the local CLI configuration.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a profile name")
		}

		if err := cfg.DeleteProfile(args[0]); err != nil {
			return err
		}

		if err := cfg.SaveConfig(); err != nil {
			return err
		}
		return nil
	},
}

var poolSwitchCmd = &cobra.Command{
	Use:          "switch",
	Short:        "Switch to a different profile",
	Long:         `Switch the CLI to a different profile.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a profile name")
		}

		if cfg != nil {
			if !cfg.HasManager(args[0]) {
				return fmt.Errorf("a profile with name %s does not exist", args[0])
			}
		}

		cfg.ActiveManager = args[0]

		if err := cfg.SaveConfig(); err != nil {
			return fmt.Errorf("error saving config: %s", err)
		}

		return nil
	},
}

var profileAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add profile",
	Long:         `Create a profile for a new garm installation.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if cfg != nil {
			if cfg.HasManager(loginProfileName) {
				return fmt.Errorf("a manager with name %s already exists in your local config", loginProfileName)
			}
		}

		if err := promptUnsetLoginVariables(); err != nil {
			return err
		}

		url := strings.TrimSuffix(loginURL, "/")

		initAPIClient(url, "")

		newLoginParamsReq := apiClientLogin.NewLoginParams()
		newLoginParamsReq.Body = params.PasswordLoginParams{
			Username: loginUserName,
			Password: loginPassword,
		}
		resp, err := apiCli.Login.Login(newLoginParamsReq, authToken)
		if err != nil {
			return err
		}

		cfg.Managers = append(cfg.Managers, config.Manager{
			Name:    loginProfileName,
			BaseURL: url,
			Token:   resp.Payload.Token,
		})
		cfg.ActiveManager = loginProfileName

		if err := cfg.SaveConfig(); err != nil {
			return err
		}
		return nil
	},
}

var profileLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Refresh bearer token for profile",
	Long: `Logs into an existing garm installation.

This command will refresh the bearer token associated with an already defined garm
installation, by performing a login.
	`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if cfg == nil {
			// We should probably error out here
			return nil
		}

		if err := promptUnsetLoginVariables(); err != nil {
			return err
		}

		newLoginParamsReq := apiClientLogin.NewLoginParams()
		newLoginParamsReq.Body = params.PasswordLoginParams{
			Username: loginUserName,
			Password: loginPassword,
		}

		resp, err := apiCli.Login.Login(newLoginParamsReq, authToken)
		if err != nil {
			return err
		}
		if err := cfg.SetManagerToken(mgr.Name, resp.Payload.Token); err != nil {
			return fmt.Errorf("error saving new token: %s", err)
		}

		if err := cfg.SaveConfig(); err != nil {
			return fmt.Errorf("error saving config: %s", err)
		}

		return nil
	},
}

func init() {
	profileLoginCmd.Flags().StringVarP(&loginUserName, "username", "u", "", "Username to log in as")
	profileLoginCmd.Flags().StringVarP(&loginPassword, "password", "p", "", "The user passowrd")

	profileAddCmd.Flags().StringVarP(&loginProfileName, "name", "n", "", "A name for this runner manager")
	profileAddCmd.Flags().StringVarP(&loginURL, "url", "a", "", "The base URL for the runner manager API")
	profileAddCmd.Flags().StringVarP(&loginUserName, "username", "u", "", "Username to log in as")
	profileAddCmd.Flags().StringVarP(&loginPassword, "password", "p", "", "The user passowrd")
	profileAddCmd.MarkFlagRequired("name") //nolint
	profileAddCmd.MarkFlagRequired("url")  //nolint

	profileCmd.AddCommand(
		profileListCmd,
		profileLoginCmd,
		poolSwitchCmd,
		profileDeleteCmd,
		profileAddCmd,
	)

	rootCmd.AddCommand(profileCmd)
}

func formatProfiles(profiles []config.Manager) {
	t := table.NewWriter()
	header := table.Row{"Name", "Base URL"}
	t.AppendHeader(header)

	for _, profile := range profiles {
		name := profile.Name
		if profile.Name == mgr.Name {
			name = fmt.Sprintf("%s (current)", name)
		}
		t.AppendRow(table.Row{name, profile.BaseURL})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func promptUnsetLoginVariables() error {
	var err error
	if loginUserName == "" {
		loginUserName, err = common.PromptString("Username")
		if err != nil {
			return err
		}
	}

	if loginPassword == "" {
		loginPassword, err = common.PromptPassword("Password")
		if err != nil {
			return err
		}
	}
	return nil
}
