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

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	apiClientController "github.com/cloudbase/garm/client/controller"
	apiClientControllerInfo "github.com/cloudbase/garm/client/controller_info"
	apiClientTools "github.com/cloudbase/garm/client/tools"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var controllerCmd = &cobra.Command{
	Use:          "controller",
	Aliases:      []string{"controller-info"},
	SilenceUsage: true,
	Short:        "Controller operations",
	Long:         `Query or update information about the current controller.`,
	Run:          nil,
}

var controllerShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show information",
	Long:         `Show information about the current controller.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		showInfo := apiClientControllerInfo.NewControllerInfoParams()
		response, err := apiCli.ControllerInfo.ControllerInfo(showInfo, authToken)
		if err != nil {
			return err
		}
		return formatInfo(response.Payload)
	},
}

var controllerUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update controller information",
	Long: `Update information about the current controller.

Warning: Dragons ahead, please read carefully.

Changing the URLs for the controller metadata, callback and webhooks, will
impact the controller's ability to manage webhooks and runners.

As GARM can be set up behind a reverse proxy or through several layers of
network address translation or load balancing, we need to explicitly tell
GARM how to reach each of these URLs. Internally, GARM sets up API endpoints
as follows:

  * /webhooks - the base URL for the webhooks. Github needs to reach this URL.
  * /api/v1/metadata - the metadata URL. Your runners need to be able to reach this URL.
  * /api/v1/callbacks - the callback URL. Your runners need to be able to reach this URL.
  * /agent - the agent URL. Your runners need to be able to reach this URL, when agent mode is used.

You need to expose these endpoints to the interested parties (github or
your runners), then you need to update the controller with the URLs you set up.

For example, if you set the webhooks URL in your reverse proxy to
https://garm.example.com/garm-hooks, this still needs to point to the "/webhooks"
URL in the GARM backend, but in the controller info you need to set the URL to
https://garm.example.com/garm-hooks using:

  garm-cli controller update --webhook-url=https://garm.example.com/garm-hooks

If you expose GARM to the outside world directly, or if you don't rewrite the URLs
above in your reverse proxy config, use the above 3 endpoints without change,
substituting garm.example.com with the correct hostname or IP address.

In most cases, you will have a GARM backend (say 192.168.100.10) and a reverse
proxy in front of it exposed as https://garm.example.com. If you don't rewrite
the URLs in the reverse proxy, and you just point to your backend, you can set
up the GARM controller URLs as:

  garm-cli controller update \
    --webhook-url=https://garm.example.com/webhooks \
    --metadata-url=https://garm.example.com/api/v1/metadata \
    --callback-url=https://garm.example.com/api/v1/callbacks \
    --agent-url=https://garm.example.com/agent

Additionally, there is one URL that is not meant to expose any service on the GARM server,
but is needed if you wish GARM to automatically sync the garm-agent tooling needed for agent
mode. This url is called garm-tools-url:

garm-cli controller update \
	--garm-tools-url=https://api.github.com/repos/cloudbase/garm-agent/releases
`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		params := params.UpdateControllerParams{}
		if cmd.Flags().Changed("metadata-url") {
			params.MetadataURL = &metadataURL
		}
		if cmd.Flags().Changed("callback-url") {
			params.CallbackURL = &callbackURL
		}
		if cmd.Flags().Changed("webhook-url") {
			params.WebhookURL = &webhookURL
		}
		if cmd.Flags().Changed("agent-url") {
			params.AgentURL = &agentURL
		}
		if cmd.Flags().Changed("garm-tools-url") {
			params.GARMAgentReleasesURL = &garmToolsReleasesURL
		}
		if cmd.Flags().Changed("enable-tools-sync") {
			params.SyncGARMAgentTools = &enableToolsSync
		}

		if cmd.Flags().Changed("minimum-job-age-backoff") {
			params.MinimumJobAgeBackoff = &minimumJobAgeBackoff
		}

		if params.WebhookURL == nil && params.MetadataURL == nil && params.CallbackURL == nil && params.MinimumJobAgeBackoff == nil && params.GARMAgentReleasesURL == nil && params.SyncGARMAgentTools == nil {
			cmd.Help()
			return fmt.Errorf("at least one of minimum-job-age-backoff, metadata-url, callback-url, enable-tools-sync, garm-tools-url  or webhook-url must be provided")
		}

		updateUrlsReq := apiClientController.NewUpdateControllerParams()
		updateUrlsReq.Body = params

		info, err := apiCli.Controller.UpdateController(updateUrlsReq, authToken)
		if err != nil {
			return fmt.Errorf("error updating controller: %w", err)
		}
		formatInfo(info.Payload)
		return nil
	},
}

var controllerToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Show information about garm tools",
	Long: `Show information about GARM tools available in this controller.

GARM has two modes by which we deploy runners:

  * Black box mode
  * Agent mode

In black box mode, we are completely agentless on the runners. The only software we really
have to install besides standrd tools like jq, curl, etc is the runner software (github/gitea).
We rely on information we get from the API of GitHub/Gitea and the APIs of the various providers
to understand the state of our runner. We care both about the lifecycle of the VM/container/Bare metal
and the lifecycle state of the runner itself (idle, active, terminated, etc). In black box mode,
we do not get any status update from the instance.

In Agent mode, we install the garm-agent on the runner, which in turn starts the actual runner. The agent
also connects back to the garm server over websockets and sends back periodic heartbeats as well as the
current state of the runner. We are able to immediately know when a job is picked up, when the job is done
and whether or not the user forcefully deleted the BM/VM/container the runner was running on or the
runner registered in github/gitea. At that point we can clean up the runner without having to thech the
github/gitea API or the API of the provider in which the runner was spawned.

This command lists the available tools in the controller. Tools can either sync automatically or be
manually uploaded. As long as the controller has access to the tools, agent mode can be enabled.
`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		showTools := apiClientTools.NewGarmAgentListParams()
		if cmd.Flags().Changed("page") {
			showTools.Page = &fileObjPage
		}
		if cmd.Flags().Changed("page-size") {
			showTools.PageSize = &fileObjPageSize
		}
		response, err := apiCli.Tools.GarmAgentList(showTools, authToken)
		if err != nil {
			return err
		}
		formatGARMToolsList(response.Payload)
		return nil
	},
}

func renderControllerInfoTable(info params.ControllerInfo) string {
	t := table.NewWriter()
	header := table.Row{"Field", "Value"}

	if info.WebhookURL == "" {
		info.WebhookURL = "N/A"
	}

	if info.ControllerWebhookURL == "" {
		info.ControllerWebhookURL = "N/A"
	}
	serverVersion := "v0.0.0-unknown"
	if info.Version != "" {
		serverVersion = info.Version
	}
	t.AppendHeader(header)
	t.AppendRow(table.Row{"Controller ID", info.ControllerID})
	if info.Hostname != "" {
		t.AppendRow(table.Row{"Hostname", info.Hostname})
	}
	t.AppendRow(table.Row{"Metadata URL", info.MetadataURL})
	t.AppendRow(table.Row{"Callback URL", info.CallbackURL})
	t.AppendRow(table.Row{"Webhook Base URL", info.WebhookURL})
	t.AppendRow(table.Row{"Controller Webhook URL", info.ControllerWebhookURL})
	t.AppendRow(table.Row{"Agent URL", info.AgentURL})
	t.AppendRow(table.Row{"GARM agent tools sync URL", info.GARMAgentReleasesURL})
	t.AppendRow(table.Row{"Tools sync enabled", info.SyncGARMAgentTools})
	t.AppendRow(table.Row{"Minimum Job Age Backoff", info.MinimumJobAgeBackoff})
	t.AppendRow(table.Row{"Version", serverVersion})
	return t.Render()
}

func formatInfo(info params.ControllerInfo) error {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(info)
		return nil
	}
	fmt.Println(renderControllerInfoTable(info))
	return nil
}

func init() {
	controllerUpdateCmd.Flags().StringVarP(&metadataURL, "metadata-url", "m", "", "The metadata URL for the controller (ie. https://garm.example.com/api/v1/metadata)")
	controllerUpdateCmd.Flags().StringVarP(&callbackURL, "callback-url", "c", "", "The callback URL for the controller (ie. https://garm.example.com/api/v1/callbacks)")
	controllerUpdateCmd.Flags().StringVarP(&webhookURL, "webhook-url", "w", "", "The webhook URL for the controller (ie. https://garm.example.com/webhooks)")
	controllerUpdateCmd.Flags().StringVarP(&agentURL, "agent-url", "g", "", "The agent URL for the controller (ie. https://garm.example.com/agent)")
	controllerUpdateCmd.Flags().StringVarP(&garmToolsReleasesURL, "garm-tools-url", "t", "", "The URL for the garm-agent releases page (ie. https://api.github.com/repos/cloudbase/garm-agent/releases)")
	controllerUpdateCmd.Flags().BoolVarP(&enableToolsSync, "enable-tools-sync", "s", false, "Enable or disable automatic garm tools sync.")
	controllerUpdateCmd.Flags().UintVarP(&minimumJobAgeBackoff, "minimum-job-age-backoff", "b", 0, "The minimum job age backoff for the controller")

	controllerToolsCmd.Flags().Int64Var(&fileObjPage, "page", 0, "The tools page to display")
	controllerToolsCmd.Flags().Int64Var(&fileObjPageSize, "page-size", 25, "Total number of results per page")
	controllerCmd.AddCommand(
		controllerShowCmd,
		controllerUpdateCmd,
		controllerToolsCmd,
	)

	rootCmd.AddCommand(controllerCmd)
}

func formatGARMToolsList(files params.GARMAgentToolsPaginatedResponse) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(files)
		return
	}
	t := table.NewWriter()
	// Define column count
	numCols := 8
	t.Style().Options.SeparateHeader = true
	t.Style().Options.SeparateRows = true

	// Page header - fill all columns with the same text
	pageHeaderText := fmt.Sprintf("Page %d of %d", files.CurrentPage, files.Pages)
	pageHeader := make(table.Row, numCols)
	for i := range pageHeader {
		pageHeader[i] = pageHeaderText
	}
	t.AppendHeader(pageHeader, table.RowConfig{
		AutoMerge:      true,
		AutoMergeAlign: text.AlignCenter,
	})
	// Column headers
	header := table.Row{"ID", "Name", "Size", "Version", "OS Type", "OS Architecture", "Created", "Updated"}
	t.AppendHeader(header)
	// Right-align numeric columns
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 3, Align: text.AlignRight},
	})

	for _, val := range files.Results {
		row := table.Row{val.ID, val.Name, formatSize(val.Size), val.Version, val.OSType, val.OSArch, val.CreatedAt.Format("2006-01-02 15:04:05"), val.UpdatedAt.Format("2006-01-02 15:04:05")}
		t.AppendRow(row)
	}
	fmt.Println(t.Render())
}
