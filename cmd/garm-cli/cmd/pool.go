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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	apiClientEnterprises "github.com/cloudbase/garm/client/enterprises"
	apiClientOrgs "github.com/cloudbase/garm/client/organizations"
	apiClientPools "github.com/cloudbase/garm/client/pools"
	apiClientRepos "github.com/cloudbase/garm/client/repositories"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	poolProvider               string
	poolMaxRunners             uint
	poolMinIdleRunners         uint
	poolRunnerPrefix           string
	poolImage                  string
	poolFlavor                 string
	poolOSType                 string
	poolOSArch                 string
	poolTags                   string
	poolEnabled                bool
	poolRunnerBootstrapTimeout uint
	poolRepository             string
	poolOrganization           string
	poolEnterprise             string
	poolExtraSpecsFile         string
	poolExtraSpecs             string
	poolAll                    bool
	poolGitHubRunnerGroup      string
	priority                   uint
)

type poolsPayloadGetter interface {
	GetPayload() params.Pools
}

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

	List pools from one enterprise:
	garm-cli pool list --enterprise=a8ee4c66-e762-4cbe-a35d-175dba2c9e62

	List all pools from all repos, orgs and enterprises:
	garm-cli pool list --all

`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		var response poolsPayloadGetter
		var err error

		switch len(args) {
		case 0:
			if cmd.Flags().Changed("repo") {
				listRepoPoolsReq := apiClientRepos.NewListRepoPoolsParams()
				listRepoPoolsReq.RepoID = poolRepository
				response, err = apiCli.Repositories.ListRepoPools(listRepoPoolsReq, authToken)
			} else if cmd.Flags().Changed("org") {
				listOrgPoolsReq := apiClientOrgs.NewListOrgPoolsParams()
				listOrgPoolsReq.OrgID = poolOrganization
				response, err = apiCli.Organizations.ListOrgPools(listOrgPoolsReq, authToken)
			} else if cmd.Flags().Changed("enterprise") {
				listEnterprisePoolsReq := apiClientEnterprises.NewListEnterprisePoolsParams()
				listEnterprisePoolsReq.EnterpriseID = poolEnterprise
				response, err = apiCli.Enterprises.ListEnterprisePools(listEnterprisePoolsReq, authToken)
			} else if cmd.Flags().Changed("all") {
				listPoolsReq := apiClientPools.NewListPoolsParams()
				response, err = apiCli.Pools.ListPools(listPoolsReq, authToken)
			} else {
				cmd.Help() //nolint
				os.Exit(0)
			}
		default:
			cmd.Help() //nolint
			os.Exit(0)
		}

		if err != nil {
			return err
		}
		formatPools(response.GetPayload())
		return nil
	},
}

var poolShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for a runner",
	Long:         `Displays a detailed view of a single runner.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a pool ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		getPoolReq := apiClientPools.NewGetPoolParams()
		getPoolReq.PoolID = args[0]
		response, err := apiCli.Pools.GetPool(getPoolReq, authToken)
		if err != nil {
			return err
		}
		formatOnePool(response.Payload)
		return nil
	},
}

var poolDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Delete pool by ID",
	Long:         `Delete one pool by referencing it's ID, regardless of repo or org.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a pool ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		deletePoolReq := apiClientPools.NewDeletePoolParams()
		deletePoolReq.PoolID = args[0]
		if err := apiCli.Pools.DeletePool(deletePoolReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

type poolPayloadGetter interface {
	GetPayload() params.Pool
}

var poolAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add pool",
	Long:         `Add a new pool to a repository or organization.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		tags := strings.Split(poolTags, ",")
		newPoolParams := params.CreatePoolParams{
			RunnerPrefix: params.RunnerPrefix{
				Prefix: poolRunnerPrefix,
			},
			ProviderName:           poolProvider,
			MaxRunners:             poolMaxRunners,
			MinIdleRunners:         poolMinIdleRunners,
			Image:                  poolImage,
			Flavor:                 poolFlavor,
			OSType:                 commonParams.OSType(poolOSType),
			OSArch:                 commonParams.OSArch(poolOSArch),
			Tags:                   tags,
			Enabled:                poolEnabled,
			RunnerBootstrapTimeout: poolRunnerBootstrapTimeout,
			GitHubRunnerGroup:      poolGitHubRunnerGroup,
			Priority:               priority,
		}

		if cmd.Flags().Changed("extra-specs") {
			data, err := asRawMessage([]byte(poolExtraSpecs))
			if err != nil {
				return err
			}
			newPoolParams.ExtraSpecs = data
		}

		if poolExtraSpecsFile != "" {
			data, err := extraSpecsFromFile(poolExtraSpecsFile)
			if err != nil {
				return err
			}
			newPoolParams.ExtraSpecs = data
		}

		if err := newPoolParams.Validate(); err != nil {
			return err
		}

		var err error
		var response poolPayloadGetter
		if cmd.Flags().Changed("repo") {
			newRepoPoolReq := apiClientRepos.NewCreateRepoPoolParams()
			newRepoPoolReq.RepoID = poolRepository
			newRepoPoolReq.Body = newPoolParams
			response, err = apiCli.Repositories.CreateRepoPool(newRepoPoolReq, authToken)
		} else if cmd.Flags().Changed("org") {
			newOrgPoolReq := apiClientOrgs.NewCreateOrgPoolParams()
			newOrgPoolReq.OrgID = poolOrganization
			newOrgPoolReq.Body = newPoolParams
			response, err = apiCli.Organizations.CreateOrgPool(newOrgPoolReq, authToken)
		} else if cmd.Flags().Changed("enterprise") {
			newEnterprisePoolReq := apiClientEnterprises.NewCreateEnterprisePoolParams()
			newEnterprisePoolReq.EnterpriseID = poolEnterprise
			newEnterprisePoolReq.Body = newPoolParams
			response, err = apiCli.Enterprises.CreateEnterprisePool(newEnterprisePoolReq, authToken)
		} else {
			cmd.Help() //nolint
			os.Exit(0)
		}

		if err != nil {
			return err
		}

		formatOnePool(response.GetPayload())
		return nil
	},
}

var poolUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update one pool",
	Long: `Updates pool characteristics.

This command updates the pool characteristics. Runners already created prior to updating
the pool, will not be recreated. If they no longer suit your needs, you will need to
explicitly remove them using the runner delete command.
	`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a poolID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		updatePoolReq := apiClientPools.NewUpdatePoolParams()
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
			poolUpdateParams.OSType = commonParams.OSType(poolOSType)
		}

		if cmd.Flags().Changed("os-arch") {
			poolUpdateParams.OSArch = commonParams.OSArch(poolOSArch)
		}

		if cmd.Flags().Changed("max-runners") {
			poolUpdateParams.MaxRunners = &poolMaxRunners
		}
		if cmd.Flags().Changed("priority") {
			poolUpdateParams.Priority = &priority
		}

		if cmd.Flags().Changed("min-idle-runners") {
			poolUpdateParams.MinIdleRunners = &poolMinIdleRunners
		}

		if cmd.Flags().Changed("runner-prefix") {
			poolUpdateParams.RunnerPrefix = params.RunnerPrefix{
				Prefix: poolRunnerPrefix,
			}
		}

		if cmd.Flags().Changed("runner-group") {
			poolUpdateParams.GitHubRunnerGroup = &poolGitHubRunnerGroup
		}

		if cmd.Flags().Changed("enabled") {
			poolUpdateParams.Enabled = &poolEnabled
		}

		if cmd.Flags().Changed("runner-bootstrap-timeout") {
			poolUpdateParams.RunnerBootstrapTimeout = &poolRunnerBootstrapTimeout
		}

		if cmd.Flags().Changed("extra-specs") {
			data, err := asRawMessage([]byte(poolExtraSpecs))
			if err != nil {
				return err
			}
			poolUpdateParams.ExtraSpecs = data
		}

		if poolExtraSpecsFile != "" {
			data, err := extraSpecsFromFile(poolExtraSpecsFile)
			if err != nil {
				return err
			}
			poolUpdateParams.ExtraSpecs = data
		}

		updatePoolReq.PoolID = args[0]
		updatePoolReq.Body = poolUpdateParams
		response, err := apiCli.Pools.UpdatePool(updatePoolReq, authToken)
		if err != nil {
			return err
		}

		formatOnePool(response.Payload)
		return nil
	},
}

func init() {
	poolListCmd.Flags().StringVarP(&poolRepository, "repo", "r", "", "List all pools within this repository.")
	poolListCmd.Flags().StringVarP(&poolOrganization, "org", "o", "", "List all pools within this organization.")
	poolListCmd.Flags().StringVarP(&poolEnterprise, "enterprise", "e", "", "List all pools within this enterprise.")
	poolListCmd.Flags().BoolVarP(&poolAll, "all", "a", false, "List all pools, regardless of org or repo.")
	poolListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")
	poolListCmd.MarkFlagsMutuallyExclusive("repo", "org", "all", "enterprise")

	poolUpdateCmd.Flags().StringVar(&poolImage, "image", "", "The provider-specific image name to use for runners in this pool.")
	poolUpdateCmd.Flags().UintVar(&priority, "priority", 0, "When multiple pools match the same labels, priority dictates the order by which they are returned, in descending order.")
	poolUpdateCmd.Flags().StringVar(&poolFlavor, "flavor", "", "The flavor to use for this runner.")
	poolUpdateCmd.Flags().StringVar(&poolTags, "tags", "", "A comma separated list of tags to assign to this runner.")
	poolUpdateCmd.Flags().StringVar(&poolOSType, "os-type", "linux", "Operating system type (windows, linux, etc).")
	poolUpdateCmd.Flags().StringVar(&poolOSArch, "os-arch", "amd64", "Operating system architecture (amd64, arm, etc).")
	poolUpdateCmd.Flags().StringVar(&poolRunnerPrefix, "runner-prefix", "", "The name prefix to use for runners in this pool.")
	poolUpdateCmd.Flags().UintVar(&poolMaxRunners, "max-runners", 5, "The maximum number of runner this pool will create.")
	poolUpdateCmd.Flags().UintVar(&poolMinIdleRunners, "min-idle-runners", 1, "Attempt to maintain a minimum of idle self-hosted runners of this type.")
	poolUpdateCmd.Flags().StringVar(&poolGitHubRunnerGroup, "runner-group", "", "The GitHub runner group in which all runners of this pool will be added.")
	poolUpdateCmd.Flags().BoolVar(&poolEnabled, "enabled", false, "Enable this pool.")
	poolUpdateCmd.Flags().UintVar(&poolRunnerBootstrapTimeout, "runner-bootstrap-timeout", 20, "Duration in minutes after which a runner is considered failed if it does not join Github.")
	poolUpdateCmd.Flags().StringVar(&poolExtraSpecsFile, "extra-specs-file", "", "A file containing a valid json which will be passed to the IaaS provider managing the pool.")
	poolUpdateCmd.Flags().StringVar(&poolExtraSpecs, "extra-specs", "", "A valid json which will be passed to the IaaS provider managing the pool.")
	poolUpdateCmd.MarkFlagsMutuallyExclusive("extra-specs-file", "extra-specs")

	poolAddCmd.Flags().StringVar(&poolProvider, "provider-name", "", "The name of the provider where runners will be created.")
	poolAddCmd.Flags().UintVar(&priority, "priority", 0, "When multiple pools match the same labels, priority dictates the order by which they are returned, in descending order.")
	poolAddCmd.Flags().StringVar(&poolImage, "image", "", "The provider-specific image name to use for runners in this pool.")
	poolAddCmd.Flags().StringVar(&poolFlavor, "flavor", "", "The flavor to use for this runner.")
	poolAddCmd.Flags().StringVar(&poolRunnerPrefix, "runner-prefix", "", "The name prefix to use for runners in this pool.")
	poolAddCmd.Flags().StringVar(&poolTags, "tags", "", "A comma separated list of tags to assign to this runner.")
	poolAddCmd.Flags().StringVar(&poolOSType, "os-type", "linux", "Operating system type (windows, linux, etc).")
	poolAddCmd.Flags().StringVar(&poolOSArch, "os-arch", "amd64", "Operating system architecture (amd64, arm, etc).")
	poolAddCmd.Flags().StringVar(&poolExtraSpecsFile, "extra-specs-file", "", "A file containing a valid json which will be passed to the IaaS provider managing the pool.")
	poolAddCmd.Flags().StringVar(&poolExtraSpecs, "extra-specs", "", "A valid json which will be passed to the IaaS provider managing the pool.")
	poolAddCmd.Flags().StringVar(&poolGitHubRunnerGroup, "runner-group", "", "The GitHub runner group in which all runners of this pool will be added.")
	poolAddCmd.Flags().UintVar(&poolMaxRunners, "max-runners", 5, "The maximum number of runner this pool will create.")
	poolAddCmd.Flags().UintVar(&poolRunnerBootstrapTimeout, "runner-bootstrap-timeout", 20, "Duration in minutes after which a runner is considered failed if it does not join Github.")
	poolAddCmd.Flags().UintVar(&poolMinIdleRunners, "min-idle-runners", 1, "Attempt to maintain a minimum of idle self-hosted runners of this type.")
	poolAddCmd.Flags().BoolVar(&poolEnabled, "enabled", false, "Enable this pool.")
	poolAddCmd.MarkFlagRequired("provider-name") //nolint
	poolAddCmd.MarkFlagRequired("image")         //nolint
	poolAddCmd.MarkFlagRequired("flavor")        //nolint
	poolAddCmd.MarkFlagRequired("tags")          //nolint

	poolAddCmd.Flags().StringVarP(&poolRepository, "repo", "r", "", "Add the new pool within this repository.")
	poolAddCmd.Flags().StringVarP(&poolOrganization, "org", "o", "", "Add the new pool within this organization.")
	poolAddCmd.Flags().StringVarP(&poolEnterprise, "enterprise", "e", "", "Add the new pool within this enterprise.")
	poolAddCmd.MarkFlagsMutuallyExclusive("repo", "org", "enterprise")
	poolAddCmd.MarkFlagsMutuallyExclusive("extra-specs-file", "extra-specs")

	poolCmd.AddCommand(
		poolListCmd,
		poolShowCmd,
		poolDeleteCmd,
		poolUpdateCmd,
		poolAddCmd,
	)

	rootCmd.AddCommand(poolCmd)
}

func extraSpecsFromFile(specsFile string) (json.RawMessage, error) {
	data, err := os.ReadFile(specsFile)
	if err != nil {
		return nil, errors.Wrap(err, "opening specs file")
	}
	return asRawMessage(data)
}

func asRawMessage(data []byte) (json.RawMessage, error) {
	// unmarshaling and marshaling again will remove new lines and verify we
	// have a valid json.
	var unmarshaled interface{}
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		return nil, errors.Wrap(err, "decoding extra specs")
	}

	var asRawJSON json.RawMessage
	var err error
	asRawJSON, err = json.Marshal(unmarshaled)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling json")
	}
	return asRawJSON, nil
}

func formatPools(pools []params.Pool) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(pools)
		return
	}
	t := table.NewWriter()
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 2, WidthMax: 40},
	})
	header := table.Row{"ID", "Image", "Flavor", "Tags", "Belongs to", "Endpoint", "Forge Type", "Enabled"}
	if long {
		header = append(header, "Level", "Created At", "Updated at", "Runner Prefix", "Priority")
	}
	t.AppendHeader(header)

	for _, pool := range pools {
		tags := []string{}
		for _, tag := range pool.Tags {
			tags = append(tags, tag.Name)
		}
		var belongsTo string
		var level string

		switch {
		case pool.RepoID != "" && pool.RepoName != "":
			belongsTo = pool.RepoName
			level = entityTypeRepo
		case pool.OrgID != "" && pool.OrgName != "":
			belongsTo = pool.OrgName
			level = entityTypeOrg
		case pool.EnterpriseID != "" && pool.EnterpriseName != "":
			belongsTo = pool.EnterpriseName
			level = entityTypeEnterprise
		}
		row := table.Row{pool.ID, pool.Image, pool.Flavor, strings.Join(tags, " "), belongsTo, pool.Endpoint.Name, pool.Endpoint.EndpointType, pool.Enabled}
		if long {
			row = append(row, level, pool.CreatedAt, pool.UpdatedAt, pool.GetRunnerPrefix(), pool.Priority)
		}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOnePool(pool params.Pool) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(pool)
		return
	}
	t := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}

	header := table.Row{"Field", "Value"}

	tags := []string{}
	for _, tag := range pool.Tags {
		tags = append(tags, tag.Name)
	}

	var belongsTo string
	var level string

	switch {
	case pool.RepoID != "" && pool.RepoName != "":
		belongsTo = pool.RepoName
		level = entityTypeRepo
	case pool.OrgID != "" && pool.OrgName != "":
		belongsTo = pool.OrgName
		level = entityTypeOrg
	case pool.EnterpriseID != "" && pool.EnterpriseName != "":
		belongsTo = pool.EnterpriseName
		level = entityTypeEnterprise
	}

	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", pool.ID})
	t.AppendRow(table.Row{"Created At", pool.CreatedAt})
	t.AppendRow(table.Row{"Updated At", pool.UpdatedAt})
	t.AppendRow(table.Row{"Provider Name", pool.ProviderName})
	t.AppendRow(table.Row{"Priority", pool.Priority})
	t.AppendRow(table.Row{"Image", pool.Image})
	t.AppendRow(table.Row{"Flavor", pool.Flavor})
	t.AppendRow(table.Row{"OS Type", pool.OSType})
	t.AppendRow(table.Row{"OS Architecture", pool.OSArch})
	t.AppendRow(table.Row{"Max Runners", pool.MaxRunners})
	t.AppendRow(table.Row{"Min Idle Runners", pool.MinIdleRunners})
	t.AppendRow(table.Row{"Runner Bootstrap Timeout", pool.RunnerBootstrapTimeout})
	t.AppendRow(table.Row{"Tags", strings.Join(tags, ", ")})
	t.AppendRow(table.Row{"Belongs to", belongsTo})
	t.AppendRow(table.Row{"Level", level})
	t.AppendRow(table.Row{"Enabled", pool.Enabled})
	t.AppendRow(table.Row{"Runner Prefix", pool.GetRunnerPrefix()})
	t.AppendRow(table.Row{"Extra specs", string(pool.ExtraSpecs)})
	t.AppendRow(table.Row{"GitHub Runner Group", pool.GitHubRunnerGroup})
	t.AppendRow(table.Row{"Forge Type", pool.Endpoint.EndpointType})
	t.AppendRow(table.Row{"Endpoint Name", pool.Endpoint.Name})

	if len(pool.Instances) > 0 {
		for _, instance := range pool.Instances {
			t.AppendRow(table.Row{"Instances", fmt.Sprintf("%s (%s)", instance.Name, instance.ID)}, rowConfigAutoMerge)
		}
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}
