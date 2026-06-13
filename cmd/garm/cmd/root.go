// Copyright 2026 Cloudbase Solutions SRL
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
	"os"

	"github.com/spf13/cobra"

	"github.com/cloudbase/garm/util/appdefaults"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:          "garm",
	Short:        "GitHub Actions Runner Manager",
	Long:         "GitHub Actions Runner Manager (GARM) - A self hosted runners manager for GitHub and Gitea Actions.",
	SilenceUsage: true,
	Version:      appdefaults.GetVersion(),
	RunE: func(_ *cobra.Command, _ []string) error {
		return runServer()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", appdefaults.DefaultConfigFilePath, "path to garm config file")
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}
