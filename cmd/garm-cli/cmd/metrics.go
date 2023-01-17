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

	"github.com/spf13/cobra"
)

// orgPoolCmd represents the pool command
var metricsTokenCMD = &cobra.Command{
	Use:          "metrics-token",
	SilenceUsage: true,
	Short:        "Handle metrics tokens",
	Long:         `Allows you to create metrics tokens.`,
	Run:          nil,
}

var metricsTokenCreateCmd = &cobra.Command{
	Use:          "create",
	Short:        "Create a metrics token",
	Long:         `Create a metrics token.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		token, err := cli.CreateMetricsToken()
		if err != nil {
			return err
		}
		fmt.Println(token)

		return nil
	},
}

func init() {
	metricsTokenCMD.AddCommand(
		metricsTokenCreateCmd,
	)

	rootCmd.AddCommand(metricsTokenCMD)
}
