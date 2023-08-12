// Copyright 2023 Cloudbase Solutions SRL
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

	apiClientControllerInfo "github.com/cloudbase/garm/client/controller_info"
	"github.com/cloudbase/garm/params"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:          "controller-info",
	SilenceUsage: true,
	Short:        "Information about controller",
	Long:         `Query information about the current controller.`,
	Run:          nil,
}

var infoShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show information",
	Long:         `Show information about the current controller.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		showInfo := apiClientControllerInfo.NewControllerInfoParams()
		response, err := apiCli.ControllerInfo.ControllerInfo(showInfo, authToken)
		if err != nil {
			return err
		}
		formatInfo(response.Payload)
		return nil
	},
}

func formatInfo(info params.ControllerInfo) {
	t := table.NewWriter()

	header := table.Row{"Field", "Value"}

	t.AppendHeader(header)
	t.AppendRow(table.Row{"Controller ID", info.ControllerID})
	t.AppendRow(table.Row{"Hostname", info.Hostname})
	t.AppendRow(table.Row{"Metadata URL", info.MetadataURL})
	t.AppendRow(table.Row{"Callback URL", info.CallbackURL})
	fmt.Println(t.Render())
}

func init() {
	infoCmd.AddCommand(
		infoShowCmd,
	)

	rootCmd.AddCommand(infoCmd)
}
