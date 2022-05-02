/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"os"
	"runner-manager/cmd/run-cli/client"
	"runner-manager/cmd/run-cli/config"

	"github.com/spf13/cobra"
)

var cfg *config.Config
var mgr config.Manager
var configErr error
var cli *client.Client
var active string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "run-cli",
	Short: "Runner manager CLI app",
	Long:  `CLI for the github self hosted runners manager.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.OnInitialize(initConfig)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	cfg, configErr = config.LoadConfig()
	if configErr == nil {
		mgr, _ = cfg.GetActiveConfig()
	}
	cli = client.NewClient(active, mgr)
}
