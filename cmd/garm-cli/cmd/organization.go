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

	"github.com/cloudbase/garm/params"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

var (
	orgName          string
	orgWebhookSecret string
	orgCreds         string
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

var orgAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add organization",
	Long:         `Add a new organization to the manager.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		newOrgReq := params.CreateOrgParams{
			Name:            orgName,
			WebhookSecret:   orgWebhookSecret,
			CredentialsName: orgCreds,
		}
		org, err := cli.CreateOrganization(newOrgReq)
		if err != nil {
			return err
		}
		formatOneOrganization(org)
		return nil
	},
}

var orgListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List organizations",
	Long:         `List all configured organizations that are currently managed.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		orgs, err := cli.ListOrganizations()
		if err != nil {
			return err
		}
		formatOrganizations(orgs)
		return nil
	},
}

var orgShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for one organization",
	Long:         `Displays detailed information about a single organization.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a organization ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		org, err := cli.GetOrganization(args[0])
		if err != nil {
			return err
		}
		formatOneOrganization(org)
		return nil
	},
}

var orgDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Removes one organization",
	Long:         `Delete one organization from the manager.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires a organization ID")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		if err := cli.DeleteOrganization(args[0]); err != nil {
			return err
		}
		return nil
	},
}

func init() {

	orgAddCmd.Flags().StringVar(&orgName, "name", "", "The name of the organization")
	orgAddCmd.Flags().StringVar(&orgWebhookSecret, "webhook-secret", "", "The webhook secret for this organization")
	orgAddCmd.Flags().StringVar(&orgCreds, "credentials", "", "Credentials name. See credentials list.")
	orgAddCmd.MarkFlagRequired("credentials") //nolint
	orgAddCmd.MarkFlagRequired("name")        //nolint

	organizationCmd.AddCommand(
		orgListCmd,
		orgAddCmd,
		orgShowCmd,
		orgDeleteCmd,
	)

	rootCmd.AddCommand(organizationCmd)
}

func formatOrganizations(orgs []params.Organization) {
	t := table.NewWriter()
	header := table.Row{"ID", "Name", "Credentials name", "Pool mgr running"}
	t.AppendHeader(header)
	for _, val := range orgs {
		t.AppendRow(table.Row{val.ID, val.Name, val.CredentialsName, val.PoolManagerStatus.IsRunning})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneOrganization(org params.Organization) {
	t := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", org.ID})
	t.AppendRow(table.Row{"Name", org.Name})
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
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: true},
	})

	fmt.Println(t.Render())
}
