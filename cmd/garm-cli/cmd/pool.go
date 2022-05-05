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
	"garm/config"
	"garm/params"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	poolRepository   string
	poolOrganization string
	poolAll          bool
)

// runnerCmd represents the runner command
var poolCmd = &cobra.Command{
	Use:          "pool",
	SilenceUsage: true,
	Short:        "List pools",
	Long:         `Query information or perform operations on pools.`,
	Run:          nil,
}

var poolListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List pools",
	Long: `List pools of repositories, orgs or all of the above.
	
This command will list pools from one repo, one org or all pools
on the system. The list flags are mutually exclusive. You must however
specify one of them.

Example:

	List pools from one repo:
	garm-cli pool list --repo=05e7eac6-4705-486d-89c9-0170bbb576af

	List pools from one org:
	garm-cli pool list --org=5493e51f-3170-4ce3-9f05-3fe690fc6ec6

	List all pools from all repos and orgs:
	garm-cli pool list --all

`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
		}

		var pools []params.Pool
		var err error

		switch len(args) {
		case 0:
			if cmd.Flags().Changed("repo") {
				pools, err = cli.ListRepoPools(poolRepository)
			} else if cmd.Flags().Changed("org") {
				pools, err = cli.ListOrgPools(poolOrganization)
			} else if cmd.Flags().Changed("all") {
				pools, err = cli.ListAllPools()
			} else {
				cmd.Help()
				os.Exit(0)
			}
		default:
			cmd.Help()
			os.Exit(0)
		}

		if err != nil {
			return err
		}
		formatPools(pools)
		return nil
	},
}

var poolShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for a runner",
	Long:         `Displays a detailed view of a single runner.`,
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

		pool, err := cli.GetPoolByID(args[0])
		if err != nil {
			return err
		}
		formatOnePool(pool)
		return nil
	},
}

var poolDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Delete pool by ID",
	Long:         `Delete one pool by referencing it's ID, regardless of repo or org.`,
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

		if err := cli.DeletePoolByID(args[0]); err != nil {
			return err
		}
		return nil
	},
}

var poolAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add pool",
	Long:         `Add a new pool to a repository or organization.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return needsInitError
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

		var pool params.Pool
		var err error

		if cmd.Flags().Changed("repo") {
			pool, err = cli.CreateRepoPool(poolRepository, newPoolParams)
		} else if cmd.Flags().Changed("org") {
			pool, err = cli.CreateOrgPool(poolOrganization, newPoolParams)
		} else {
			cmd.Help()
			os.Exit(0)
		}

		if err != nil {
			return err
		}
		formatOnePool(pool)
		return nil
	},
}

var poolUpdateCmd = &cobra.Command{
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

		if len(args) == 0 {
			return fmt.Errorf("command requires a poolID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
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

		pool, err := cli.UpdatePoolByID(args[0], poolUpdateParams)
		if err != nil {
			return err
		}

		formatOnePool(pool)
		return nil
	},
}

func init() {
	poolListCmd.Flags().StringVarP(&poolRepository, "repo", "r", "", "List all pools within this repository.")
	poolListCmd.Flags().StringVarP(&poolOrganization, "org", "o", "", "List all pools withing this organization.")
	poolListCmd.Flags().BoolVarP(&poolAll, "all", "a", false, "List all pools, regardless of org or repo.")
	poolListCmd.MarkFlagsMutuallyExclusive("repo", "org", "all")

	poolUpdateCmd.Flags().StringVar(&poolImage, "image", "", "The provider-specific image name to use for runners in this pool.")
	poolUpdateCmd.Flags().StringVar(&poolFlavor, "flavor", "", "The flavor to use for this runner.")
	poolUpdateCmd.Flags().StringVar(&poolTags, "tags", "", "A comma separated list of tags to assign to this runner.")
	poolUpdateCmd.Flags().StringVar(&poolOSType, "os-type", "linux", "Operating system type (windows, linux, etc).")
	poolUpdateCmd.Flags().StringVar(&poolOSArch, "os-arch", "amd64", "Operating system architecture (amd64, arm, etc).")
	poolUpdateCmd.Flags().UintVar(&poolMaxRunners, "max-runners", 5, "The maximum number of runner this pool will create.")
	poolUpdateCmd.Flags().UintVar(&poolMinIdleRunners, "min-idle-runners", 1, "Attempt to maintain a minimum of idle self-hosted runners of this type.")
	poolUpdateCmd.Flags().BoolVar(&poolEnabled, "enabled", false, "Enable this pool.")

	poolAddCmd.Flags().StringVar(&poolProvider, "provider-name", "", "The name of the provider where runners will be created.")
	poolAddCmd.Flags().StringVar(&poolImage, "image", "", "The provider-specific image name to use for runners in this pool.")
	poolAddCmd.Flags().StringVar(&poolFlavor, "flavor", "", "The flavor to use for this runner.")
	poolAddCmd.Flags().StringVar(&poolTags, "tags", "", "A comma separated list of tags to assign to this runner.")
	poolAddCmd.Flags().StringVar(&poolOSType, "os-type", "linux", "Operating system type (windows, linux, etc).")
	poolAddCmd.Flags().StringVar(&poolOSArch, "os-arch", "amd64", "Operating system architecture (amd64, arm, etc).")
	poolAddCmd.Flags().UintVar(&poolMaxRunners, "max-runners", 5, "The maximum number of runner this pool will create.")
	poolAddCmd.Flags().UintVar(&poolMinIdleRunners, "min-idle-runners", 1, "Attempt to maintain a minimum of idle self-hosted runners of this type.")
	poolAddCmd.Flags().BoolVar(&poolEnabled, "enabled", false, "Enable this pool.")
	poolAddCmd.MarkFlagRequired("provider-name")
	poolAddCmd.MarkFlagRequired("image")
	poolAddCmd.MarkFlagRequired("flavor")
	poolAddCmd.MarkFlagRequired("tags")

	poolAddCmd.Flags().StringVarP(&poolRepository, "repo", "r", "", "Add the new pool within this repository.")
	poolAddCmd.Flags().StringVarP(&poolOrganization, "org", "o", "", "Add the new pool withing this organization.")
	poolAddCmd.MarkFlagsMutuallyExclusive("repo", "org")

	poolCmd.AddCommand(
		poolListCmd,
		poolShowCmd,
		poolDeleteCmd,
		poolUpdateCmd,
		poolAddCmd,
	)

	rootCmd.AddCommand(poolCmd)
}
