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

	apiClientControllerInfo "github.com/cloudbase/garm/client/controller_info"
	"github.com/cloudbase/garm/util/appdefaults"
)

// runnerCmd represents the runner command
var versionCmd = &cobra.Command{
	Use:          "version",
	SilenceUsage: true,
	Short:        "Print version and exit",
	Run: func(_ *cobra.Command, _ []string) {
		serverVersion := "v0.0.0-unknown"

		if !needsInit {
			showInfo := apiClientControllerInfo.NewControllerInfoParams()
			response, err := apiCli.ControllerInfo.ControllerInfo(showInfo, authToken)
			if err == nil {
				serverVersion = response.Payload.Version
			}
		}

		fmt.Printf("garm-cli: %s\n", appdefaults.GetVersion())
		fmt.Printf("garm server: %s\n", serverVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
