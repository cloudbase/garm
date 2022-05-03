/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"os"
	"runner-manager/cmd/run-cli/client"
	"runner-manager/cmd/run-cli/config"

	"github.com/spf13/cobra"
)

var (
	cfg            *config.Config
	mgr            config.Manager
	cli            *client.Client
	active         string
	needsInit      bool
	debug          bool
	needsInitError = fmt.Errorf("Please log into a runner-manager first")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "run-cli",
	Short: "Runner manager CLI app",
	Long:  `CLI for the github self hosted runners manager.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug on all API calls")
	cobra.OnInitialize(initConfig)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %s", err)
		os.Exit(1)
	}
	if len(cfg.Managers) == 0 {
		// config is empty.
		needsInit = true
	} else {
		mgr, err = cfg.GetActiveConfig()
		if err != nil {
			mgr = cfg.Managers[0]
		}
		active = mgr.Name
	}
	cli = client.NewClient(active, mgr, debug)
}
