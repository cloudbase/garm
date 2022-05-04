/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"garm/config"
	"garm/params"
	"strings"

	"github.com/spf13/cobra"
)

// orgPoolCmd represents the pool command
var orgPoolCmd = &cobra.Command{
	Use:          "pool",
	SilenceUsage: true,
	Aliases:      []string{"pools"},
	Short:        "Manage pools",
	Long: `Manage pools for a organization.

Repositories and organizations can define multiple pools with different
characteristics, which in turn will spawn github self hosted runners on
compute instances that reflect those characteristics.

For example, one pool can define a runner with tags "GPU,ML" which will
spin up instances with access to a GPU, on the desired provider.`,
	Run: nil,
}

var orgPoolAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add pool",
	Long:         `Add a new pool organization to the manager.`,
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

		tags := strings.Split(poolTags, ",")
		newPoolParams := params.CreatePoolParams{
			ProviderName:   poolProvider,
			MaxRunners:     poolMaxRunners,
			MinIdleRunners: poolMinIdleRunners,
			Image:          poolImage,
			Flavor:         poolFlavor,
			OSType:         config.OSType(poolOSType),
			OSArch:         config.OSArch(poolOSArch),
			Tags:           tags,
			Enabled:        poolEnabled,
		}
		if err := newPoolParams.Validate(); err != nil {
			return err
		}
		pool, err := cli.CreateOrgPool(args[0], newPoolParams)
		if err != nil {
			return err
		}
		formatOnePool(pool)
		return nil
	},
}

var orgPoolListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List organization pools",
	Long:         `List all configured pools for a given organization.`,
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

		pools, err := cli.ListOrgPools(args[0])
		if err != nil {
			return err
		}
		formatPools(pools)
		return nil
	},
}

var orgPoolShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show details for one pool",
	Long:  `Displays detailed information about a single pool.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}

		if len(args) < 2 || len(args) > 2 {
			return fmt.Errorf("command requires orgID and poolID")
		}

		pool, err := cli.GetOrgPool(args[0], args[1])
		if err != nil {
			return err
		}

		formatOnePool(pool)
		return nil
	},
}

var orgPoolDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Removes one pool",
	Long:         `Delete one organization pool from the manager.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}
		if len(args) < 2 || len(args) > 2 {
			return fmt.Errorf("command requires orgID and poolID")
		}

		if err := cli.DeleteOrgPool(args[0], args[1]); err != nil {
			return err
		}
		return nil
	},
}

var orgPoolUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update one pool",
	Long: `Updates pool characteristics.

This command updates the pool characteristics. Runners already created prior to updating
the pool, will not be recreated. IF they no longer suit your needs, you will need to
explicitly remove them using the runner delete command.
	`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}

		if len(args) < 2 || len(args) > 2 {
			return fmt.Errorf("command requires orgID and poolID")
		}

		poolUpdateParams := params.UpdatePoolParams{}

		if cmd.Flags().Changed("image") {
			poolUpdateParams.Image = poolImage
		}

		if cmd.Flags().Changed("flavor") {
			poolUpdateParams.Flavor = poolFlavor
		}

		if cmd.Flags().Changed("tags") {
			poolUpdateParams.Tags = strings.Split(poolTags, ",")
		}

		if cmd.Flags().Changed("os-type") {
			poolUpdateParams.OSType = config.OSType(poolOSType)
		}

		if cmd.Flags().Changed("os-arch") {
			poolUpdateParams.OSArch = config.OSArch(poolOSArch)
		}

		if cmd.Flags().Changed("max-runners") {
			poolUpdateParams.MaxRunners = &poolMaxRunners
		}

		if cmd.Flags().Changed("min-idle-runners") {
			poolUpdateParams.MinIdleRunners = &poolMinIdleRunners
		}

		if cmd.Flags().Changed("enabled") {
			poolUpdateParams.Enabled = &poolEnabled
		}

		pool, err := cli.UpdateOrgPool(args[0], args[1], poolUpdateParams)
		if err != nil {
			return err
		}

		formatOnePool(pool)
		return nil
	},
}

func init() {
	orgPoolAddCmd.Flags().StringVar(&poolProvider, "provider-name", "", "The name of the provider where runners will be created.")
	orgPoolAddCmd.Flags().StringVar(&poolImage, "image", "", "The provider-specific image name to use for runners in this pool.")
	orgPoolAddCmd.Flags().StringVar(&poolFlavor, "flavor", "", "The flavor to use for this runner.")
	orgPoolAddCmd.Flags().StringVar(&poolTags, "tags", "", "A comma separated list of tags to assign to this runner.")
	orgPoolAddCmd.Flags().StringVar(&poolOSType, "os-type", "linux", "Operating system type (windows, linux, etc).")
	orgPoolAddCmd.Flags().StringVar(&poolOSArch, "os-arch", "amd64", "Operating system architecture (amd64, arm, etc).")
	orgPoolAddCmd.Flags().UintVar(&poolMaxRunners, "max-runners", 5, "The maximum number of runner this pool will create.")
	orgPoolAddCmd.Flags().UintVar(&poolMinIdleRunners, "min-idle-runners", 1, "Attempt to maintain a minimum of idle self-hosted runners of this type.")
	orgPoolAddCmd.Flags().BoolVar(&poolEnabled, "enabled", false, "Enable this pool.")
	orgPoolAddCmd.MarkFlagRequired("provider-name")
	orgPoolAddCmd.MarkFlagRequired("image")
	orgPoolAddCmd.MarkFlagRequired("flavor")
	orgPoolAddCmd.MarkFlagRequired("tags")

	orgPoolUpdateCmd.Flags().StringVar(&poolImage, "image", "", "The provider-specific image name to use for runners in this pool.")
	orgPoolUpdateCmd.Flags().StringVar(&poolFlavor, "flavor", "", "The flavor to use for this runner.")
	orgPoolUpdateCmd.Flags().StringVar(&poolTags, "tags", "", "A comma separated list of tags to assign to this runner.")
	orgPoolUpdateCmd.Flags().StringVar(&poolOSType, "os-type", "linux", "Operating system type (windows, linux, etc).")
	orgPoolUpdateCmd.Flags().StringVar(&poolOSArch, "os-arch", "amd64", "Operating system architecture (amd64, arm, etc).")
	orgPoolUpdateCmd.Flags().UintVar(&poolMaxRunners, "max-runners", 5, "The maximum number of runner this pool will create.")
	orgPoolUpdateCmd.Flags().UintVar(&poolMinIdleRunners, "min-idle-runners", 1, "Attempt to maintain a minimum of idle self-hosted runners of this type.")
	orgPoolUpdateCmd.Flags().BoolVar(&poolEnabled, "enabled", false, "Enable this pool.")

	orgPoolCmd.AddCommand(
		orgPoolListCmd,
		orgPoolAddCmd,
		orgPoolShowCmd,
		orgPoolDeleteCmd,
		orgPoolUpdateCmd,
	)

	organizationCmd.AddCommand(orgPoolCmd)
}
