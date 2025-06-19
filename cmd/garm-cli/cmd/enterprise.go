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

	apiClientEnterprises "github.com/cloudbase/garm/client/enterprises"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	enterpriseName          string
	enterpriseEndpoint      string
	enterpriseWebhookSecret string
	enterpriseCreds         string
)

// enterpriseCmd represents the enterprise command
var enterpriseCmd = &cobra.Command{
	Use:          "enterprise",
	Aliases:      []string{"ent"},
	SilenceUsage: true,
	Short:        "Manage enterprise",
	Long: `Add, remove or update enterprise for which we manage
self hosted runners.

This command allows you to define a new enterprise or manage an existing
enterprise for which garm maintains pools of self hosted runners.`,
	Run: nil,
}

var enterpriseAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add enterprise",
	Long:         `Add a new enterprise to the manager.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		newEnterpriseReq := apiClientEnterprises.NewCreateEnterpriseParams()
		newEnterpriseReq.Body = params.CreateEnterpriseParams{
			Name:             enterpriseName,
			WebhookSecret:    enterpriseWebhookSecret,
			CredentialsName:  enterpriseCreds,
			PoolBalancerType: params.PoolBalancerType(poolBalancerType),
		}
		response, err := apiCli.Enterprises.CreateEnterprise(newEnterpriseReq, authToken)
		if err != nil {
			return err
		}
		formatOneEnterprise(response.Payload)
		return nil
	},
}

var enterpriseListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List enterprises",
	Long:         `List all configured enterprises that are currently managed.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listEnterprisesReq := apiClientEnterprises.NewListEnterprisesParams()
		listEnterprisesReq.Name = &enterpriseName
		listEnterprisesReq.Endpoint = &enterpriseEndpoint
		response, err := apiCli.Enterprises.ListEnterprises(listEnterprisesReq, authToken)
		if err != nil {
			return err
		}
		formatEnterprises(response.Payload)
		return nil
	},
}

var enterpriseShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for one enterprise",
	Long:         `Displays detailed information about a single enterprise.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a enterprise ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		enterpriseID, err := resolveEnterprise(args[0])
		if err != nil {
			return err
		}

		showEnterpriseReq := apiClientEnterprises.NewGetEnterpriseParams()
		showEnterpriseReq.EnterpriseID = enterpriseID
		response, err := apiCli.Enterprises.GetEnterprise(showEnterpriseReq, authToken)
		if err != nil {
			return err
		}
		formatOneEnterprise(response.Payload)
		return nil
	},
}

var enterpriseDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Removes one enterprise",
	Long:         `Delete one enterprise from the manager.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a enterprise ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		enterpriseID, err := resolveEnterprise(args[0])
		if err != nil {
			return err
		}

		deleteEnterpriseReq := apiClientEnterprises.NewDeleteEnterpriseParams()
		deleteEnterpriseReq.EnterpriseID = enterpriseID
		if err := apiCli.Enterprises.DeleteEnterprise(deleteEnterpriseReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

var enterpriseUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update enterprise",
	Long:         `Update enterprise credentials or webhook secret.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a enterprise ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		enterpriseID, err := resolveEnterprise(args[0])
		if err != nil {
			return err
		}

		updateEnterpriseReq := apiClientEnterprises.NewUpdateEnterpriseParams()
		updateEnterpriseReq.Body = params.UpdateEntityParams{
			WebhookSecret:    repoWebhookSecret,
			CredentialsName:  repoCreds,
			PoolBalancerType: params.PoolBalancerType(poolBalancerType),
		}
		updateEnterpriseReq.EnterpriseID = enterpriseID
		response, err := apiCli.Enterprises.UpdateEnterprise(updateEnterpriseReq, authToken)
		if err != nil {
			return err
		}
		formatOneEnterprise(response.Payload)
		return nil
	},
}

func init() {
	enterpriseAddCmd.Flags().StringVar(&enterpriseName, "name", "", "The name of the enterprise")
	enterpriseAddCmd.Flags().StringVar(&enterpriseWebhookSecret, "webhook-secret", "", "The webhook secret for this enterprise")
	enterpriseAddCmd.Flags().StringVar(&enterpriseCreds, "credentials", "", "Credentials name. See credentials list.")
	enterpriseAddCmd.Flags().StringVar(&poolBalancerType, "pool-balancer-type", string(params.PoolBalancerTypeRoundRobin), "The balancing strategy to use when creating runners in pools matching requested labels.")

	enterpriseListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")
	enterpriseListCmd.Flags().StringVarP(&enterpriseName, "name", "n", "", "Exact enterprise name to filter by.")
	enterpriseListCmd.Flags().StringVarP(&enterpriseEndpoint, "endpoint", "e", "", "Exact endpoint name to filter by.")

	enterpriseAddCmd.MarkFlagRequired("credentials") //nolint
	enterpriseAddCmd.MarkFlagRequired("name")        //nolint
	enterpriseUpdateCmd.Flags().StringVar(&enterpriseWebhookSecret, "webhook-secret", "", "The webhook secret for this enterprise")
	enterpriseUpdateCmd.Flags().StringVar(&enterpriseCreds, "credentials", "", "Credentials name. See credentials list.")
	enterpriseUpdateCmd.Flags().StringVar(&poolBalancerType, "pool-balancer-type", "", "The balancing strategy to use when creating runners in pools matching requested labels.")

	enterpriseCmd.AddCommand(
		enterpriseListCmd,
		enterpriseAddCmd,
		enterpriseShowCmd,
		enterpriseDeleteCmd,
		enterpriseUpdateCmd,
	)

	rootCmd.AddCommand(enterpriseCmd)
}

func formatEnterprises(enterprises []params.Enterprise) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(enterprises)
		return
	}
	t := table.NewWriter()
	header := table.Row{"ID", "Name", "Endpoint", "Credentials name", "Pool Balancer Type", "Pool mgr running"}
	if long {
		header = append(header, "Created At", "Updated At")
	}
	t.AppendHeader(header)
	for _, val := range enterprises {
		row := table.Row{val.ID, val.Name, val.Endpoint.Name, val.Credentials.Name, val.GetBalancerType(), val.PoolManagerStatus.IsRunning}
		if long {
			row = append(row, val.CreatedAt, val.UpdatedAt)
		}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneEnterprise(enterprise params.Enterprise) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(enterprise)
		return
	}
	t := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", enterprise.ID})
	t.AppendRow(table.Row{"Created At", enterprise.CreatedAt})
	t.AppendRow(table.Row{"Updated At", enterprise.UpdatedAt})
	t.AppendRow(table.Row{"Name", enterprise.Name})
	t.AppendRow(table.Row{"Endpoint", enterprise.Endpoint.Name})
	t.AppendRow(table.Row{"Pool balancer type", enterprise.GetBalancerType()})
	t.AppendRow(table.Row{"Credentials", enterprise.Credentials.Name})
	t.AppendRow(table.Row{"Pool manager running", enterprise.PoolManagerStatus.IsRunning})
	if !enterprise.PoolManagerStatus.IsRunning {
		t.AppendRow(table.Row{"Failure reason", enterprise.PoolManagerStatus.FailureReason})
	}

	if len(enterprise.Pools) > 0 {
		for _, pool := range enterprise.Pools {
			t.AppendRow(table.Row{"Pools", pool.ID}, rowConfigAutoMerge)
		}
	}

	if len(enterprise.Events) > 0 {
		for _, event := range enterprise.Events {
			t.AppendRow(table.Row{"Events", fmt.Sprintf("%s %s: %s", event.CreatedAt.Format("2006-01-02T15:04:05"), strings.ToUpper(string(event.EventLevel)), event.Message)}, rowConfigAutoMerge)
		}
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})

	fmt.Println(t.Render())
}
