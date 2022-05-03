/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"runner-manager/params"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

// runnerCmd represents the runner command
var runnerCmd = &cobra.Command{
	Use:          "runner",
	SilenceUsage: true,
	Short:        "List runners in a pool",
	Long: `Given a pool ID, of either a repository or an organization,
list all instances.`,
	Run: nil,
}

var runnerListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List pool runners",
	Long:         `List all configured pools for a given repository.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a pool ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		instances, err := cli.ListPoolInstances(args[0])
		if err != nil {
			return err
		}
		formatInstances(instances)
		return nil
	},
}

var runnerShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for a runner",
	Long:         `Displays a detailed view of a single runner.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a runner name")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		instance, err := cli.GetInstanceByName(args[0])
		if err != nil {
			return err
		}
		formatSingleInstance(instance)
		return nil
	},
}

func init() {
	runnerCmd.AddCommand(
		runnerListCmd,
		runnerShowCmd,
	)

	rootCmd.AddCommand(runnerCmd)
}

func formatInstances(param []params.Instance) {
	t := table.NewWriter()
	header := table.Row{"Name", "Status", "Runner Status", "Pool ID"}
	t.AppendHeader(header)

	for _, inst := range param {
		t.AppendRow(table.Row{inst.Name, inst.Status, inst.RunnerStatus, inst.PoolID})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatSingleInstance(instance params.Instance) {
	t := table.NewWriter()

	header := table.Row{"Field", "Value"}

	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", instance.ID}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Provider ID", instance.ProviderID}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Name", instance.Name}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"OS Type", instance.OSType}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"OS Architecture", instance.OSArch}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"OS Name", instance.OSName}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"OS Version", instance.OSVersion}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Status", instance.Status}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Runner Status", instance.RunnerStatus}, table.RowConfig{AutoMerge: false})
	t.AppendRow(table.Row{"Pool ID", instance.PoolID}, table.RowConfig{AutoMerge: false})

	if len(instance.Addresses) > 0 {
		for _, addr := range instance.Addresses {
			t.AppendRow(table.Row{"Addresses", addr}, table.RowConfig{AutoMerge: true})
		}
	}

	if len(instance.StatusMessages) > 0 {
		for _, msg := range instance.StatusMessages {
			t.AppendRow(table.Row{"Status Updates", fmt.Sprintf("%s: %s", msg.CreatedAt.Format("2006-01-02T15:04:05"), msg.Message)}, table.RowConfig{AutoMerge: true})
		}
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false},
	})
	fmt.Println(t.Render())
}
