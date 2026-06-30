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
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/cloudbase/garm-provider-common/util"
	apiClientForgeInstances "github.com/cloudbase/garm/client/forge_instances"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	forgeInstanceEndpoint       string
	forgeInstanceWebhookSecret  string
	forgeInstanceRandomSecret   bool
	forgeInstanceCreds          string
	forgeInstanceForgeType      string
	forgeInstanceAgentMode      bool
	installForgeInstanceWebhook    bool
	insecureForgeInstanceWebhook  bool
)

var forgeInstanceCmd = &cobra.Command{
	Use:          "forge-instance",
	Aliases:      []string{"fi"},
	SilenceUsage: true,
	Short:        "Manage forge instances",
	Long: `Add, remove or update forge instances for which we manage
self hosted runners.

A forge instance represents a Gitea (or compatible) server
for which garm manages instance-level runner pools.`,
	Run: nil,
}

var forgeInstanceAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add forge instance",
	Long:         `Add a new forge instance to the manager.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if forgeInstanceRandomSecret {
			secret, err := util.GetRandomString(32)
			if err != nil {
				return err
			}
			forgeInstanceWebhookSecret = secret
		}

		newReq := apiClientForgeInstances.NewCreateForgeInstanceParams()
		newReq.Body = params.CreateForgeInstanceParams{
			EndpointName:     forgeInstanceEndpoint,
			WebhookSecret:    forgeInstanceWebhookSecret,
			CredentialsName:  forgeInstanceCreds,
			ForgeType:        params.EndpointType(forgeInstanceForgeType),
			PoolBalancerType: params.PoolBalancerType(poolBalancerType),
			AgentMode:        forgeInstanceAgentMode,
		}
		response, err := apiCli.ForgeInstances.CreateForgeInstance(newReq, authToken)
		if err != nil {
			return err
		}

		if installForgeInstanceWebhook {
			installWebhookReq := apiClientForgeInstances.NewInstallForgeInstanceWebhookParams()
			installWebhookReq.ForgeInstanceID = response.Payload.ID
			installWebhookReq.Body.WebhookEndpointType = params.WebhookEndpointDirect

			_, err = apiCli.ForgeInstances.InstallForgeInstanceWebhook(installWebhookReq, authToken)
			if err != nil {
				return err
			}
		}

		// Re-fetch to include updated webhook status.
		getReq := apiClientForgeInstances.NewGetForgeInstanceParams()
		getReq.ForgeInstanceID = response.Payload.ID
		fi, err := apiCli.ForgeInstances.GetForgeInstance(getReq, authToken)
		if err != nil {
			return err
		}
		formatOneForgeInstance(fi.Payload)
		return nil
	},
}

var forgeInstanceListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List forge instances",
	Long:         `List all configured forge instances that are currently managed.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listReq := apiClientForgeInstances.NewListForgeInstancesParams()
		if forgeInstanceEndpoint != "" {
			listReq.Endpoint = &forgeInstanceEndpoint
		}
		response, err := apiCli.ForgeInstances.ListForgeInstances(listReq, authToken)
		if err != nil {
			return err
		}
		formatForgeInstances(response.Payload)
		return nil
	},
}

var forgeInstanceShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for one forge instance",
	Long:         `Displays detailed information about a single forge instance.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a forge instance ID or endpoint name")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		forgeInstanceID, err := resolveForgeInstance(args[0])
		if err != nil {
			return err
		}

		showReq := apiClientForgeInstances.NewGetForgeInstanceParams()
		showReq.ForgeInstanceID = forgeInstanceID
		response, err := apiCli.ForgeInstances.GetForgeInstance(showReq, authToken)
		if err != nil {
			return err
		}
		formatOneForgeInstance(response.Payload)
		return nil
	},
}

var forgeInstanceDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Removes one forge instance",
	Long:         `Delete one forge instance from the manager.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a forge instance ID or endpoint name")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		forgeInstanceID, err := resolveForgeInstance(args[0])
		if err != nil {
			return err
		}

		deleteReq := apiClientForgeInstances.NewDeleteForgeInstanceParams()
		deleteReq.ForgeInstanceID = forgeInstanceID
		if err := apiCli.ForgeInstances.DeleteForgeInstance(deleteReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

var forgeInstanceUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update forge instance",
	Long:         `Update forge instance credentials or webhook secret.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a forge instance ID or endpoint name")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		forgeInstanceID, err := resolveForgeInstance(args[0])
		if err != nil {
			return err
		}

		updateReq := apiClientForgeInstances.NewUpdateForgeInstanceParams()
		updateReq.Body = params.UpdateEntityParams{
			WebhookSecret:    forgeInstanceWebhookSecret,
			CredentialsName:  forgeInstanceCreds,
			PoolBalancerType: params.PoolBalancerType(poolBalancerType),
		}
		if cmd.Flags().Changed("agent-mode") {
			updateReq.Body.AgentMode = &forgeInstanceAgentMode
		}
		updateReq.ForgeInstanceID = forgeInstanceID
		response, err := apiCli.ForgeInstances.UpdateForgeInstance(updateReq, authToken)
		if err != nil {
			return err
		}
		formatOneForgeInstance(response.Payload)
		return nil
	},
}

var forgeInstanceWebhookCmd = &cobra.Command{
	Use:          "webhook",
	Short:        "Manage forge instance webhooks",
	Long:         `Manage forge instance webhooks.`,
	SilenceUsage: true,
	Run:          nil,
}

var forgeInstanceWebhookInstallCmd = &cobra.Command{
	Use:          "install",
	Short:        "Install webhook",
	Long:         `Install webhook for a forge instance.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a forge instance ID or endpoint name")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		forgeInstanceID, err := resolveForgeInstance(args[0])
		if err != nil {
			return err
		}

		installWebhookReq := apiClientForgeInstances.NewInstallForgeInstanceWebhookParams()
		installWebhookReq.ForgeInstanceID = forgeInstanceID
		installWebhookReq.Body.InsecureSSL = insecureForgeInstanceWebhook
		installWebhookReq.Body.WebhookEndpointType = params.WebhookEndpointDirect

		response, err := apiCli.ForgeInstances.InstallForgeInstanceWebhook(installWebhookReq, authToken)
		if err != nil {
			return err
		}
		formatOneHookInfo(response.Payload)
		return nil
	},
}

var forgeInstanceWebhookShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show webhook info",
	Long:         `Show webhook info for a forge instance.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a forge instance ID or endpoint name")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		forgeInstanceID, err := resolveForgeInstance(args[0])
		if err != nil {
			return err
		}
		showWebhookInfoReq := apiClientForgeInstances.NewGetForgeInstanceWebhookInfoParams()
		showWebhookInfoReq.ForgeInstanceID = forgeInstanceID

		response, err := apiCli.ForgeInstances.GetForgeInstanceWebhookInfo(showWebhookInfoReq, authToken)
		if err != nil {
			return err
		}
		formatOneHookInfo(response.Payload)
		return nil
	},
}

var forgeInstanceWebhookUninstallCmd = &cobra.Command{
	Use:          "uninstall",
	Short:        "Uninstall webhook",
	Long:         `Uninstall webhook for a forge instance.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a forge instance ID or endpoint name")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		forgeInstanceID, err := resolveForgeInstance(args[0])
		if err != nil {
			return err
		}

		uninstallWebhookReq := apiClientForgeInstances.NewUninstallForgeInstanceWebhookParams()
		uninstallWebhookReq.ForgeInstanceID = forgeInstanceID

		err = apiCli.ForgeInstances.UninstallForgeInstanceWebhook(uninstallWebhookReq, authToken)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	forgeInstanceAddCmd.Flags().StringVar(&forgeInstanceEndpoint, "endpoint", "", "The endpoint name for this forge instance")
	forgeInstanceAddCmd.Flags().StringVar(&forgeInstanceWebhookSecret, "webhook-secret", "", "The webhook secret for this forge instance.")
	forgeInstanceAddCmd.Flags().BoolVar(&forgeInstanceRandomSecret, "random-webhook-secret", false, "Generate a random webhook secret for this forge instance.")
	forgeInstanceAddCmd.Flags().StringVar(&forgeInstanceCreds, "credentials", "", "Credentials name. See credentials list.")
	forgeInstanceAddCmd.Flags().StringVar(&forgeInstanceForgeType, "forge-type", string(params.GiteaEndpointType), "The forge type (e.g. gitea).")
	forgeInstanceAddCmd.Flags().StringVar(&poolBalancerType, "pool-balancer-type", string(params.PoolBalancerTypeRoundRobin), "The balancing strategy to use when creating runners in pools matching requested labels.")
	forgeInstanceAddCmd.Flags().BoolVar(&forgeInstanceAgentMode, "agent-mode", false, "Enable agent mode for runners in this forge instance.")
	forgeInstanceAddCmd.Flags().BoolVar(&installForgeInstanceWebhook, "install-webhook", false, "Install the webhook as part of the add operation.")

	forgeInstanceAddCmd.MarkFlagRequired("credentials")                                       //nolint
	forgeInstanceAddCmd.MarkFlagRequired("endpoint")                                          //nolint
	forgeInstanceAddCmd.MarkFlagsMutuallyExclusive("webhook-secret", "random-webhook-secret") //nolint
	forgeInstanceAddCmd.MarkFlagsOneRequired("webhook-secret", "random-webhook-secret")       //nolint

	forgeInstanceListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")
	forgeInstanceListCmd.Flags().StringVarP(&forgeInstanceEndpoint, "endpoint", "e", "", "Exact endpoint name to filter by.")

	forgeInstanceUpdateCmd.Flags().StringVar(&forgeInstanceWebhookSecret, "webhook-secret", "", "The webhook secret for this forge instance")
	forgeInstanceUpdateCmd.Flags().StringVar(&forgeInstanceCreds, "credentials", "", "Credentials name. See credentials list.")
	forgeInstanceUpdateCmd.Flags().StringVar(&poolBalancerType, "pool-balancer-type", "", "The balancing strategy to use when creating runners in pools matching requested labels.")
	forgeInstanceUpdateCmd.Flags().BoolVar(&forgeInstanceAgentMode, "agent-mode", false, "Enable agent mode for runners in this forge instance.")

	forgeInstanceWebhookInstallCmd.Flags().BoolVar(&insecureForgeInstanceWebhook, "insecure", false, "Ignore self signed certificate errors.")

	forgeInstanceWebhookCmd.AddCommand(
		forgeInstanceWebhookInstallCmd,
		forgeInstanceWebhookUninstallCmd,
		forgeInstanceWebhookShowCmd,
	)

	forgeInstanceCmd.AddCommand(
		forgeInstanceListCmd,
		forgeInstanceAddCmd,
		forgeInstanceShowCmd,
		forgeInstanceDeleteCmd,
		forgeInstanceUpdateCmd,
		forgeInstanceWebhookCmd,
	)

	rootCmd.AddCommand(forgeInstanceCmd)
}

func resolveForgeInstance(nameOrID string) (string, error) {
	if nameOrID == "" {
		return "", fmt.Errorf("missing forge instance endpoint name or ID")
	}
	_, err := uuid.Parse(nameOrID)
	if err == nil {
		// It's a valid UUID, use it directly.
		return nameOrID, nil
	}

	// Not a UUID — treat as endpoint name and look it up.
	listReq := apiClientForgeInstances.NewListForgeInstancesParams()
	listReq.Endpoint = &nameOrID
	response, err := apiCli.ForgeInstances.ListForgeInstances(listReq, authToken)
	if err != nil {
		return "", err
	}

	if len(response.Payload) == 0 {
		return "", fmt.Errorf("forge instance with endpoint %s was not found", nameOrID)
	}

	if len(response.Payload) > 1 {
		return "", fmt.Errorf("multiple forge instances with endpoint %s exist, please use the forge instance ID", nameOrID)
	}

	return response.Payload[0].ID, nil
}

func formatForgeInstances(forgeInstances []params.ForgeInstance) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(forgeInstances)
		return
	}
	t := table.NewWriter()
	header := table.Row{"Endpoint", "Credentials name", "Pool Balancer Type", "Pool mgr running"}
	if long {
		header = append(header, "ID", "Created At", "Updated At")
	}
	t.AppendHeader(header)
	for _, val := range forgeInstances {
		row := table.Row{val.Endpoint.Name, val.Credentials.Name, val.GetBalancerType(), val.PoolManagerStatus.IsRunning}
		if long {
			row = append(row, val.ID, val.CreatedAt, val.UpdatedAt)
		}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneForgeInstance(fi params.ForgeInstance) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(fi)
		return
	}
	t := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", fi.ID})
	t.AppendRow(table.Row{"Created At", fi.CreatedAt})
	t.AppendRow(table.Row{"Updated At", fi.UpdatedAt})
	t.AppendRow(table.Row{"Endpoint", fi.Endpoint.Name})
	t.AppendRow(table.Row{"Pool balancer type", fi.GetBalancerType()})
	t.AppendRow(table.Row{"Credentials", fi.Credentials.Name})
	t.AppendRow(table.Row{"Agent Mode", fi.AgentMode})
	t.AppendRow(table.Row{"Pool manager running", fi.PoolManagerStatus.IsRunning})
	if !fi.PoolManagerStatus.IsRunning {
		t.AppendRow(table.Row{"Failure reason", fi.PoolManagerStatus.FailureReason})
	}

	if len(fi.Pools) > 0 {
		for _, pool := range fi.Pools {
			t.AppendRow(table.Row{"Pools", pool.ID}, rowConfigAutoMerge)
		}
	}

	if len(fi.Events) > 0 {
		for _, event := range fi.Events {
			t.AppendRow(table.Row{"Events", fmt.Sprintf("%s %s: %s", event.CreatedAt.Format("2006-01-02T15:04:05"), strings.ToUpper(string(event.EventLevel)), event.Message)}, rowConfigAutoMerge)
		}
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})

	fmt.Println(t.Render())
}
