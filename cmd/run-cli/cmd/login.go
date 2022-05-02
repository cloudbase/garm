/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	loginName     string
	loginURL      string
	loginPassword string
	loginUserName string
	loginFullName string
	loginEmail    string
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:          "login",
	SilenceUsage: true,
	Short:        "Log into a manager",
	Long: `Performs login for this machine on a remote runner-manager:

run-cli login --name=dev --url=https://runner.example.com`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("--> %v %v\n", cfg, configErr)
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().StringVarP(&loginName, "name", "n", "", "A name for this runner manager")
	loginCmd.Flags().StringVarP(&loginURL, "url", "a", "", "The base URL for the runner manager API")
	loginCmd.MarkFlagRequired("name")
	loginCmd.MarkFlagRequired("url")
}
