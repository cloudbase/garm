/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"strings"

	"runner-manager/cmd/run-cli/common"
	"runner-manager/cmd/run-cli/config"
	"runner-manager/params"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:          "init",
	SilenceUsage: true,
	Short:        "Initialize a newly installed runner-manager",
	Long: `Initiallize a new installation of runner-manager.

A newly installed runner manager needs to be initialized to become
functional. This command sets the administrative user and password,
generates a controller UUID which is used internally to identify runners
created by the manager and enables the service.

Example usage:

run-cli login --name=dev --url=https://runner.example.com --username=admin --password=superSecretPassword
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg != nil {
			if cfg.HasManager(loginProfileName) {
				return fmt.Errorf("a manager with name %s already exists in your local config", loginProfileName)
			}
		}

		if err := promptUnsetInitVariables(); err != nil {
			return err
		}

		newUser := params.NewUserParams{
			Username: loginUserName,
			Password: loginPassword,
			FullName: loginFullName,
			Email:    loginEmail,
		}

		url := strings.TrimSuffix(loginURL, "/")
		response, err := cli.InitManager(url, newUser)
		if err != nil {
			return errors.Wrap(err, "initializing manager")
		}

		loginParams := params.PasswordLoginParams{
			Username: loginUserName,
			Password: loginPassword,
		}

		token, err := cli.Login(url, loginParams)
		if err != nil {
			return errors.Wrap(err, "authenticating")
		}

		cfg.Managers = append(cfg.Managers, config.Manager{
			Name:    loginProfileName,
			BaseURL: url,
			Token:   token,
		})

		cfg.ActiveManager = loginProfileName

		if err := cfg.SaveConfig(); err != nil {
			return errors.Wrap(err, "saving config")
		}

		renderUserTable(response)
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
	initCmd.MarkFlagRequired("name")
	initCmd.MarkFlagRequired("url")
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
