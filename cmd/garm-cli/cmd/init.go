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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	apiClientFirstRun "github.com/cloudbase/garm/client/first_run"
	apiClientLogin "github.com/cloudbase/garm/client/login"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/cmd/garm-cli/config"
	"github.com/cloudbase/garm/params"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:          "init",
	SilenceUsage: true,
	Short:        "Initialize a newly installed garm",
	Long: `Initiallize a new installation of garm.

A newly installed runner manager needs to be initialized to become
functional. This command sets the administrative user and password,
generates a controller UUID which is used internally to identify runners
created by the manager and adds the profile to the local client config.

Example usage:

garm-cli init --name=dev --url=https://runner.example.com --username=admin --password=superSecretPassword
`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if cfg != nil {
			if cfg.HasManager(loginProfileName) {
				return fmt.Errorf("a manager with name %s already exists in your local config", loginProfileName)
			}
		}

		if err := promptUnsetInitVariables(); err != nil {
			return err
		}

		newUserReq := apiClientFirstRun.NewFirstRunParams()
		newUserReq.Body = params.NewUserParams{
			Username: loginUserName,
			Password: loginPassword,
			FullName: loginFullName,
			Email:    loginEmail,
		}

		url := strings.TrimSuffix(loginURL, "/")

		initAPIClient(url, "")

		response, err := apiCli.FirstRun.FirstRun(newUserReq, authToken)
		if err != nil {
			return errors.Wrap(err, "initializing manager")
		}

		newLoginParamsReq := apiClientLogin.NewLoginParams()
		newLoginParamsReq.Body = params.PasswordLoginParams{
			Username: loginUserName,
			Password: loginPassword,
		}

		token, err := apiCli.Login.Login(newLoginParamsReq, authToken)
		if err != nil {
			return errors.Wrap(err, "authenticating")
		}

		cfg.Managers = append(cfg.Managers, config.Manager{
			Name:    loginProfileName,
			BaseURL: url,
			Token:   token.Payload.Token,
		})

		cfg.ActiveManager = loginProfileName

		if err := cfg.SaveConfig(); err != nil {
			return errors.Wrap(err, "saving config")
		}

		renderUserTable(response.Payload)
		return nil
	},
}

func promptUnsetInitVariables() error {
	var err error
	if loginUserName == "" {
		loginUserName, err = common.PromptString("Username")
		if err != nil {
			return err
		}
	}

	if loginEmail == "" {
		loginEmail, err = common.PromptString("Email")
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

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&loginProfileName, "name", "n", "", "A name for this runner manager")
	initCmd.Flags().StringVarP(&loginURL, "url", "a", "", "The base URL for the runner manager API")
	initCmd.Flags().StringVarP(&loginUserName, "username", "u", "", "The desired administrative username")
	initCmd.Flags().StringVarP(&loginEmail, "email", "e", "", "Email address")
	initCmd.Flags().StringVarP(&loginFullName, "full-name", "f", "", "Full name of the user")
	initCmd.Flags().StringVarP(&loginPassword, "password", "p", "", "The admin password")
	initCmd.MarkFlagRequired("name") //nolint
	initCmd.MarkFlagRequired("url")  //nolint
}

func renderUserTable(user params.User) {
	t := table.NewWriter()
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)

	t.AppendRow(table.Row{"ID", user.ID})
	t.AppendRow(table.Row{"Username", user.Username})
	t.AppendRow(table.Row{"Email", user.Email})
	t.AppendRow(table.Row{"Enabled", user.Enabled})
	fmt.Println(t.Render())
}
