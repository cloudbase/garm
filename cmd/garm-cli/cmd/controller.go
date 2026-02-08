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
	"debug/elf"
	"debug/pe"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	apiClientController "github.com/cloudbase/garm/client/controller"
	apiClientControllerInfo "github.com/cloudbase/garm/client/controller_info"
	apiClientObject "github.com/cloudbase/garm/client/objects"
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
	Short: "Manage GARM agent tools",
	Long: `Manage GARM agent tools available in this controller.

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

Tools can either sync automatically from a GitHub release URL or be manually uploaded.
As long as the controller has access to the tools, agent mode can be enabled.
`,
	SilenceUsage: true,
	Run:          nil,
}

var controllerToolsListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List GARM agent tools",
	Long:         `List all GARM agent tools available in the controller. Use --upstream to list tools from the upstream cached release.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		showTools := apiClientTools.NewAdminGarmAgentListParams()
		if cmd.Flags().Changed("page") {
			showTools.Page = &fileObjPage
		}
		if cmd.Flags().Changed("page-size") {
			showTools.PageSize = &fileObjPageSize
		}
		if cmd.Flags().Changed("upstream") {
			upstreamVal := true
			showTools.Upstream = &upstreamVal
		}
		response, err := apiCli.Tools.AdminGarmAgentList(showTools, authToken)
		if err != nil {
			return err
		}
		formatGARMToolsList(response.Payload, cmd.Flags().Changed("upstream"))
		return nil
	},
}

var controllerToolsShowCmd = &cobra.Command{
	Use:          "show <tool-id>",
	Short:        "Show details of a specific GARM agent tool",
	Long:         `Display detailed information about a specific GARM agent tool by ID.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		toolID := args[0]
		getReq := apiClientObject.NewGetFileObjectParams().WithObjectID(toolID)
		resp, err := apiCli.Objects.GetFileObject(getReq, authToken)
		if err != nil {
			return err
		}
		formatOneObject(resp.Payload)
		return nil
	},
}

var controllerToolsDeleteCmd = &cobra.Command{
	Use:          "delete <tool-id>",
	Aliases:      []string{"remove", "rm"},
	Short:        "Delete a GARM agent tool",
	Long:         `Delete a specific GARM agent tool from the object store by ID.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		toolID := args[0]
		delReq := apiClientObject.NewDeleteFileObjectParams().WithObjectID(toolID)
		err := apiCli.Objects.DeleteFileObject(delReq, authToken)
		if err != nil {
			return err
		}

		fmt.Printf("Tool %s deleted successfully\n", toolID)
		return nil
	},
}

var controllerToolsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Force immediate sync of GARM agent tools",
	Long: `Force an immediate sync of GARM agent tools from the configured release URL.

This command triggers the background worker to fetch the latest tools from the
configured GARM agent release URL and sync them to the object store.

Note: This command requires that GARM agent tools sync is enabled in the controller
configuration. If sync is disabled, the command will return an error.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		// POST to /controller/tools/sync endpoint
		// Since this is not auto-generated, we'll make a direct HTTP request
		apiURL := fmt.Sprintf("%s/api/v1/controller/tools/sync", mgr.BaseURL)

		req, err := http.NewRequest("POST", apiURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Add auth token
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", mgr.Token))
		req.Header.Add("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("sync failed with status %d: %s", resp.StatusCode, string(body))
		}

		var ctrlInfo params.ControllerInfo
		if err := json.NewDecoder(resp.Body).Decode(&ctrlInfo); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		fmt.Println("Tools sync initiated successfully")
		formatInfo(ctrlInfo)
		return nil
	},
}

var (
	toolFilePath string
	toolOSType   string
	toolOSArch   string
	toolVersion  string
	toolName     string
)

var controllerToolsUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload a GARM agent tool binary",
	Long: `Upload a GARM agent tool binary for a specific OS and architecture.

This command uploads a tool and automatically:
- Sets origin=manual tag
- Overwrites any existing auto-synced tool for the same OS/architecture
- Ensures only one tool version per OS/architecture combination`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		// Default name if not provided
		if toolName == "" {
			toolName = fmt.Sprintf("garm-agent-%s-%s", toolOSType, toolOSArch)
			if toolOSType == "windows" {
				toolName += ".exe"
			}
		}

		// Get file info for size
		stat, err := os.Stat(toolFilePath)
		if err != nil {
			return fmt.Errorf("failed to access file: %w", err)
		}

		// Open the file
		file, err := os.Open(toolFilePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		// Validate file type and architecture matches OS using standard library
		if toolOSType == "linux" {
			elfFile, err := elf.NewFile(file)
			if err != nil {
				return fmt.Errorf("file is not a valid ELF binary (required for Linux): %w", err)
			}
			defer elfFile.Close()

			// Check file type is executable (ET_EXEC or ET_DYN for PIE)
			if elfFile.Type != elf.ET_EXEC && elfFile.Type != elf.ET_DYN {
				return fmt.Errorf("file is not a valid ELF executable (required for Linux): type is %v (must be ET_EXEC or ET_DYN)", elfFile.Type)
			}

			// Check architecture matches
			var expectedMachine elf.Machine
			var archName string
			switch toolOSArch {
			case "amd64":
				expectedMachine = elf.EM_X86_64
				archName = "x86-64"
			case "arm64":
				expectedMachine = elf.EM_AARCH64
				archName = "ARM64"
			}
			if elfFile.Machine != expectedMachine {
				return fmt.Errorf("file is ELF binary for %v, but %s (%s) was specified", elfFile.Machine, toolOSArch, archName)
			}
		}

		if toolOSType == "windows" {
			peFile, err := pe.NewFile(file)
			if err != nil {
				return fmt.Errorf("file is not a valid PE executable (required for Windows): %w", err)
			}
			defer peFile.Close()

			// Check architecture matches
			var expectedMachine uint16
			var archName string
			switch toolOSArch {
			case "amd64":
				expectedMachine = pe.IMAGE_FILE_MACHINE_AMD64
				archName = "x86-64"
			case "arm64":
				expectedMachine = pe.IMAGE_FILE_MACHINE_ARM64
				archName = "ARM64"
			}
			if peFile.Machine != expectedMachine {
				return fmt.Errorf("file is PE executable for machine type 0x%x, but %s (%s) was specified", peFile.Machine, toolOSArch, archName)
			}
		}

		// Seek back to beginning for upload
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to seek to beginning of file: %w", err)
		}

		// Show initial progress
		fmt.Printf("Uploading %s (%.2f MB)...\n", toolName, float64(stat.Size())/1024/1024)

		// Create request to tools endpoint using custom headers
		description := fmt.Sprintf("GARM Agent %s for %s/%s (manually uploaded)", toolVersion, toolOSType, toolOSArch)

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/tools/garm-agent", mgr.BaseURL), file)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Set auth and metadata headers
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", mgr.Token))
		req.Header.Set("X-Tool-Name", toolName)
		req.Header.Set("X-Tool-Description", description)
		req.Header.Set("X-Tool-OS-Type", toolOSType)
		req.Header.Set("X-Tool-OS-Arch", toolOSArch)
		req.Header.Set("X-Tool-Version", toolVersion)
		req.ContentLength = stat.Size()

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to upload: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		// Check for non-2xx status codes
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
		}

		var uploadedTool params.FileObject
		if err := json.Unmarshal(data, &uploadedTool); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		fmt.Printf("\nTool uploaded successfully\n")
		fmt.Printf("ID: %d\n", uploadedTool.ID)
		fmt.Printf("Name: %s\n", uploadedTool.Name)
		fmt.Printf("Size: %s\n", formatSize(uploadedTool.Size))
		fmt.Printf("SHA256: %s\n", uploadedTool.SHA256)
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

	controllerToolsListCmd.Flags().Int64Var(&fileObjPage, "page", 0, "The tools page to display")
	controllerToolsListCmd.Flags().Int64Var(&fileObjPageSize, "page-size", 25, "Total number of results per page")
	controllerToolsListCmd.Flags().Bool("upstream", false, "List tools from the upstream cached release instead of the local object store")

	controllerToolsUploadCmd.Flags().StringVar(&toolFilePath, "file", "", "Path to the garm-agent binary file (required)")
	controllerToolsUploadCmd.Flags().StringVar(&toolOSType, "os", "", "Operating system: linux or windows (required)")
	controllerToolsUploadCmd.Flags().StringVar(&toolOSArch, "arch", "", "Architecture: amd64 or arm64 (required)")
	controllerToolsUploadCmd.Flags().StringVar(&toolVersion, "version", "", "Version string, e.g., v1.0.0 (required)")
	controllerToolsUploadCmd.Flags().StringVar(&toolName, "name", "", "Custom name for the tool (optional, defaults to garm-agent-{os}-{arch})")

	controllerToolsUploadCmd.MarkFlagRequired("file")
	controllerToolsUploadCmd.MarkFlagRequired("os")
	controllerToolsUploadCmd.MarkFlagRequired("arch")
	controllerToolsUploadCmd.MarkFlagRequired("version")

	controllerToolsCmd.AddCommand(
		controllerToolsListCmd,
		controllerToolsShowCmd,
		controllerToolsDeleteCmd,
		controllerToolsSyncCmd,
		controllerToolsUploadCmd,
	)

	controllerCmd.AddCommand(
		controllerShowCmd,
		controllerUpdateCmd,
		controllerToolsCmd,
	)

	rootCmd.AddCommand(controllerCmd)
}

func formatGARMToolsList(files params.GARMAgentToolsPaginatedResponse, upstream bool) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(files)
		return
	}
	t := table.NewWriter()
	t.Style().Options.SeparateHeader = true
	t.Style().Options.SeparateRows = true

	var numCols int
	var header table.Row
	if upstream {
		numCols = 6
		header = table.Row{"Name", "Size", "Version", "OS Type", "OS Architecture", "Download URL"}
	} else {
		numCols = 8
		header = table.Row{"ID", "Name", "Size", "Version", "OS Type", "OS Architecture", "Created", "Updated"}
	}

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
	t.AppendHeader(header)

	if upstream {
		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 2, Align: text.AlignRight},
		})
		for _, val := range files.Results {
			t.AppendRow(table.Row{val.Name, formatSize(val.Size), val.Version, val.OSType, val.OSArch, val.DownloadURL})
		}
	} else {
		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 1, Align: text.AlignRight},
			{Number: 3, Align: text.AlignRight},
		})
		for _, val := range files.Results {
			t.AppendRow(table.Row{val.ID, val.Name, formatSize(val.Size), val.Version, val.OSType, val.OSArch, val.CreatedAt, val.UpdatedAt})
		}
	}
	fmt.Println(t.Render())
}
