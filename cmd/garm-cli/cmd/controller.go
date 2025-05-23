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
	"github.com/spf13/cobra"

	apiClientController "github.com/cloudbase/garm/client/controller"
	apiClientControllerInfo "github.com/cloudbase/garm/client/controller_info"
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
    --callback-url=https://garm.example.com/api/v1/callbacks
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

		if cmd.Flags().Changed("minimum-job-age-backoff") {
			params.MinimumJobAgeBackoff = &minimumJobAgeBackoff
		}

		if params.WebhookURL == nil && params.MetadataURL == nil && params.CallbackURL == nil && params.MinimumJobAgeBackoff == nil {
			cmd.Help()
			return fmt.Errorf("at least one of minimum-job-age-backoff, metadata-url, callback-url or webhook-url must be provided")
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
	controllerUpdateCmd.Flags().UintVarP(&minimumJobAgeBackoff, "minimum-job-age-backoff", "b", 0, "The minimum job age backoff for the controller")

	controllerCmd.AddCommand(
		controllerShowCmd,
		controllerUpdateCmd,
	)

	rootCmd.AddCommand(controllerCmd)
}
