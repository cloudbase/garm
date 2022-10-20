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
	"garm/cmd/garm-cli/client"
	"garm/cmd/garm-cli/config"
	"os"

	"github.com/spf13/cobra"
)

var Version string

var (
	cfg               *config.Config
	mgr               config.Manager
	cli               *client.Client
	active            string
	needsInit         bool
	debug             bool
	errNeedsInitError = fmt.Errorf("please log into a garm installation first")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "garm-cli",
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
