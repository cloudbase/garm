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
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/cloudbase/garm-provider-common/util"
	apiClientOrgs "github.com/cloudbase/garm/client/organizations"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	orgName                string
	orgEndpoint            string
	orgWebhookSecret       string
	orgCreds               string
	orgRandomWebhookSecret bool
	insecureOrgWebhook     bool
	keepOrgWebhook         bool
	installOrgWebhook      bool
)

// organizationCmd represents the organization command
var organizationCmd = &cobra.Command{
	Use:          "organization",
	Aliases:      []string{"org"},
	SilenceUsage: true,
	Short:        "Manage organizations",
	Long: `Add, remove or update organizations for which we manage
self hosted runners.

This command allows you to define a new organization or manage an existing
organization for which garm maintains pools of self hosted runners.`,
	Run: nil,
}

var orgWebhookCmd = &cobra.Command{
	Use:          "webhook",
	Short:        "Manage organization webhooks",
	Long:         `Manage organization webhooks.`,
	SilenceUsage: true,
	Run:          nil,
}

var orgWebhookInstallCmd = &cobra.Command{
	Use:          "install",
	Short:        "Install webhook",
	Long:         `Install webhook for an organization.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires an organization ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		installWebhookReq := apiClientOrgs.NewInstallOrgWebhookParams()
		installWebhookReq.OrgID = args[0]
		installWebhookReq.Body.InsecureSSL = insecureOrgWebhook
		installWebhookReq.Body.WebhookEndpointType = params.WebhookEndpointDirect

		response, err := apiCli.Organizations.InstallOrgWebhook(installWebhookReq, authToken)
		if err != nil {
			return err
		}
		formatOneHookInfo(response.Payload)
		return nil
	},
}

var orgHookInfoShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show webhook info",
	Long:         `Show webhook info for an organization.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires an organization ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		showWebhookInfoReq := apiClientOrgs.NewGetOrgWebhookInfoParams()
		showWebhookInfoReq.OrgID = args[0]

		response, err := apiCli.Organizations.GetOrgWebhookInfo(showWebhookInfoReq, authToken)
		if err != nil {
			return err
		}
		formatOneHookInfo(response.Payload)
		return nil
	},
}

var orgWebhookUninstallCmd = &cobra.Command{
	Use:          "uninstall",
	Short:        "Uninstall webhook",
	Long:         `Uninstall webhook for an organization.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires an organization ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		uninstallWebhookReq := apiClientOrgs.NewUninstallOrgWebhookParams()
		uninstallWebhookReq.OrgID = args[0]

		err := apiCli.Organizations.UninstallOrgWebhook(uninstallWebhookReq, authToken)
		if err != nil {
			return err
		}
		return nil
	},
}

var orgAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add organization",
	Long:         `Add a new organization to the manager.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if orgRandomWebhookSecret {
			secret, err := util.GetRandomString(32)
			if err != nil {
				return err
			}
			orgWebhookSecret = secret
		}

		newOrgReq := apiClientOrgs.NewCreateOrgParams()
		newOrgReq.Body = params.CreateOrgParams{
			Name:             orgName,
			WebhookSecret:    orgWebhookSecret,
			CredentialsName:  orgCreds,
			ForgeType:        params.EndpointType(forgeType),
			PoolBalancerType: params.PoolBalancerType(poolBalancerType),
		}
		response, err := apiCli.Organizations.CreateOrg(newOrgReq, authToken)
		if err != nil {
			return err
		}

		if installOrgWebhook {
			installWebhookReq := apiClientOrgs.NewInstallOrgWebhookParams()
			installWebhookReq.OrgID = response.Payload.ID
			installWebhookReq.Body.WebhookEndpointType = params.WebhookEndpointDirect

			_, err = apiCli.Organizations.InstallOrgWebhook(installWebhookReq, authToken)
			if err != nil {
				return err
			}
		}

		getOrgRequest := apiClientOrgs.NewGetOrgParams()
		getOrgRequest.OrgID = response.Payload.ID
		org, err := apiCli.Organizations.GetOrg(getOrgRequest, authToken)
		if err != nil {
			return err
		}
		formatOneOrganization(org.Payload)
		return nil
	},
}

var orgUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update organization",
	Long:         `Update organization credentials or webhook secret.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a organization ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		updateOrgReq := apiClientOrgs.NewUpdateOrgParams()
		updateOrgReq.Body = params.UpdateEntityParams{
			WebhookSecret:    orgWebhookSecret,
			CredentialsName:  orgCreds,
			PoolBalancerType: params.PoolBalancerType(poolBalancerType),
		}
		updateOrgReq.OrgID = args[0]
		response, err := apiCli.Organizations.UpdateOrg(updateOrgReq, authToken)
		if err != nil {
			return err
		}
		formatOneOrganization(response.Payload)
		return nil
	},
}

var orgListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List organizations",
	Long:         `List all configured organizations that are currently managed.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listOrgsReq := apiClientOrgs.NewListOrgsParams()
		listOrgsReq.Name = &orgName
		listOrgsReq.Endpoint = &orgEndpoint
		response, err := apiCli.Organizations.ListOrgs(listOrgsReq, authToken)
		if err != nil {
			return err
		}
		formatOrganizations(response.Payload)
		return nil
	},
}

var orgShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for one organization",
	Long:         `Displays detailed information about a single organization.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a organization ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		showOrgReq := apiClientOrgs.NewGetOrgParams()
		showOrgReq.OrgID = args[0]
		response, err := apiCli.Organizations.GetOrg(showOrgReq, authToken)
		if err != nil {
			return err
		}
		formatOneOrganization(response.Payload)
		return nil
	},
}

var orgDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Removes one organization",
	Long:         `Delete one organization from the manager.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a organization ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		deleteOrgReq := apiClientOrgs.NewDeleteOrgParams()
		deleteOrgReq.OrgID = args[0]
		deleteOrgReq.KeepWebhook = &keepOrgWebhook
		if err := apiCli.Organizations.DeleteOrg(deleteOrgReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	orgAddCmd.Flags().StringVar(&orgName, "name", "", "The name of the organization")
	orgAddCmd.Flags().StringVar(&poolBalancerType, "pool-balancer-type", string(params.PoolBalancerTypeRoundRobin), "The balancing strategy to use when creating runners in pools matching requested labels.")
	orgAddCmd.Flags().StringVar(&orgWebhookSecret, "webhook-secret", "", "The webhook secret for this organization")
	orgAddCmd.Flags().StringVar(&forgeType, "forge-type", "", "The forge type of the organization. Supported values: github, gitea.")
	orgAddCmd.Flags().StringVar(&orgCreds, "credentials", "", "Credentials name. See credentials list.")
	orgAddCmd.Flags().BoolVar(&orgRandomWebhookSecret, "random-webhook-secret", false, "Generate a random webhook secret for this organization.")
	orgAddCmd.Flags().BoolVar(&installOrgWebhook, "install-webhook", false, "Install the webhook as part of the add operation.")
	orgAddCmd.MarkFlagsMutuallyExclusive("webhook-secret", "random-webhook-secret")
	orgAddCmd.MarkFlagsOneRequired("webhook-secret", "random-webhook-secret")

	orgListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")
	orgListCmd.Flags().StringVarP(&orgName, "name", "n", "", "Exact org name to filter by.")
	orgListCmd.Flags().StringVarP(&orgEndpoint, "endpoint", "e", "", "Exact endpoint name to filter by.")

	orgAddCmd.MarkFlagRequired("credentials") //nolint
	orgAddCmd.MarkFlagRequired("name")        //nolint

	orgDeleteCmd.Flags().BoolVar(&keepOrgWebhook, "keep-webhook", false, "Do not delete any existing webhook when removing the organization from GARM.")

	orgUpdateCmd.Flags().StringVar(&orgWebhookSecret, "webhook-secret", "", "The webhook secret for this organization")
	orgUpdateCmd.Flags().StringVar(&orgCreds, "credentials", "", "Credentials name. See credentials list.")
	orgUpdateCmd.Flags().StringVar(&poolBalancerType, "pool-balancer-type", "", "The balancing strategy to use when creating runners in pools matching requested labels.")

	orgWebhookInstallCmd.Flags().BoolVar(&insecureOrgWebhook, "insecure", false, "Ignore self signed certificate errors.")
	orgWebhookCmd.AddCommand(
		orgWebhookInstallCmd,
		orgWebhookUninstallCmd,
		orgHookInfoShowCmd,
	)

	organizationCmd.AddCommand(
		orgListCmd,
		orgAddCmd,
		orgShowCmd,
		orgDeleteCmd,
		orgUpdateCmd,
		orgWebhookCmd,
	)

	rootCmd.AddCommand(organizationCmd)
}

func formatOrganizations(orgs []params.Organization) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(orgs)
		return
	}
	t := table.NewWriter()
	header := table.Row{"ID", "Name", "Endpoint", "Credentials name", "Pool Balancer Type", "Forge type", "Pool mgr running"}
	if long {
		header = append(header, "Created At", "Updated At")
	}
	t.AppendHeader(header)
	for _, val := range orgs {
		forgeType := val.Endpoint.EndpointType
		if forgeType == "" {
			forgeType = params.GithubEndpointType
		}
		row := table.Row{val.ID, val.Name, val.Endpoint.Name, val.CredentialsName, val.GetBalancerType(), forgeType, val.PoolManagerStatus.IsRunning}
		if long {
			row = append(row, val.CreatedAt, val.UpdatedAt)
		}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneOrganization(org params.Organization) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(org)
		return
	}
	t := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", org.ID})
	t.AppendRow(table.Row{"Created At", org.CreatedAt})
	t.AppendRow(table.Row{"Updated At", org.UpdatedAt})
	t.AppendRow(table.Row{"Name", org.Name})
	t.AppendRow(table.Row{"Endpoint", org.Endpoint.Name})
	t.AppendRow(table.Row{"Pool balancer type", org.GetBalancerType()})
	t.AppendRow(table.Row{"Credentials", org.CredentialsName})
	t.AppendRow(table.Row{"Pool manager running", org.PoolManagerStatus.IsRunning})
	if !org.PoolManagerStatus.IsRunning {
		t.AppendRow(table.Row{"Failure reason", org.PoolManagerStatus.FailureReason})
	}
	if len(org.Pools) > 0 {
		for _, pool := range org.Pools {
			t.AppendRow(table.Row{"Pools", pool.ID}, rowConfigAutoMerge)
		}
	}
	if len(org.Events) > 0 {
		for _, event := range org.Events {
			t.AppendRow(table.Row{"Events", fmt.Sprintf("%s %s: %s", event.CreatedAt.Format("2006-01-02T15:04:05"), strings.ToUpper(string(event.EventLevel)), event.Message)}, rowConfigAutoMerge)
		}
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})

	fmt.Println(t.Render())
}
