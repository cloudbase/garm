package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// orgPoolCmd represents the pool command
var orgInstancesCmd = &cobra.Command{
	Use:          "runner",
	SilenceUsage: true,
	Short:        "List runners",
	Long:         `List runners from all pools defined in this organization.`,
	Run:          nil,
}

var orgRunnerListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List organization runners",
	Long:         `List all runners for a given organization.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a organization ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		instances, err := cli.ListOrgInstances(args[0])
		if err != nil {
			return err
		}
		formatInstances(instances)
		return nil
	},
}

func init() {
	orgInstancesCmd.AddCommand(
		orgRunnerListCmd,
	)

	organizationCmd.AddCommand(orgInstancesCmd)
}
