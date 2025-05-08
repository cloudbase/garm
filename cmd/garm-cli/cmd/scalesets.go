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
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	apiClientEnterprises "github.com/cloudbase/garm/client/enterprises"
	apiClientOrgs "github.com/cloudbase/garm/client/organizations"
	apiClientRepos "github.com/cloudbase/garm/client/repositories"
	apiClientScaleSets "github.com/cloudbase/garm/client/scalesets"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	scalesetProvider               string
	scalesetMaxRunners             uint
	scalesetMinIdleRunners         uint
	scalesetRunnerPrefix           string
	scalesetName                   string
	scalesetImage                  string
	scalesetFlavor                 string
	scalesetOSType                 string
	scalesetOSArch                 string
	scalesetEnabled                bool
	scalesetRunnerBootstrapTimeout uint
	scalesetRepository             string
	scalesetOrganization           string
	scalesetEnterprise             string
	scalesetExtraSpecsFile         string
	scalesetExtraSpecs             string
	scalesetAll                    bool
	scalesetGitHubRunnerGroup      string
)

type scalesetPayloadGetter interface {
	GetPayload() params.ScaleSet
}

type scalesetsPayloadGetter interface {
	GetPayload() params.ScaleSets
}

// scalesetCmd represents the scale set command
var scalesetCmd = &cobra.Command{
	Use:          "scaleset",
	SilenceUsage: true,
	Short:        "List scale sets",
	Long:         `Query information or perform operations on scale sets.`,
	Run:          nil,
}

var scalesetListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List scale sets",
	Long: `List scale sets of repositories, orgs or all of the above.

This command will list scale sets from one repo, one org or all scale sets
on the system. The list flags are mutually exclusive. You must however
specify one of them.

Example:

	List scalesets from one repo:
	garm-cli scaleset list --repo=05e7eac6-4705-486d-89c9-0170bbb576af

	List scalesets from one org:
	garm-cli scaleset list --org=5493e51f-3170-4ce3-9f05-3fe690fc6ec6

	List scalesets from one enterprise:
	garm-cli scaleset list --enterprise=a8ee4c66-e762-4cbe-a35d-175dba2c9e62

	List all scalesets from all repos, orgs and enterprises:
	garm-cli scaleset list --all

`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		var response scalesetsPayloadGetter
		var err error

		switch len(args) {
		case 0:
			if cmd.Flags().Changed("repo") {
				listRepoScaleSetsReq := apiClientRepos.NewListRepoScaleSetsParams()
				listRepoScaleSetsReq.RepoID = scalesetRepository
				response, err = apiCli.Repositories.ListRepoScaleSets(listRepoScaleSetsReq, authToken)
			} else if cmd.Flags().Changed("org") {
				listOrgScaleSetsReq := apiClientOrgs.NewListOrgScaleSetsParams()
				listOrgScaleSetsReq.OrgID = scalesetOrganization
				response, err = apiCli.Organizations.ListOrgScaleSets(listOrgScaleSetsReq, authToken)
			} else if cmd.Flags().Changed("enterprise") {
				listEnterpriseScaleSetsReq := apiClientEnterprises.NewListEnterpriseScaleSetsParams()
				listEnterpriseScaleSetsReq.EnterpriseID = scalesetEnterprise
				response, err = apiCli.Enterprises.ListEnterpriseScaleSets(listEnterpriseScaleSetsReq, authToken)
			} else if cmd.Flags().Changed("all") {
				listScaleSetsReq := apiClientScaleSets.NewListScalesetsParams()
				response, err = apiCli.Scalesets.ListScalesets(listScaleSetsReq, authToken)
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
		formatScaleSets(response.GetPayload())
		return nil
	},
}

var scaleSetShowCmd = &cobra.Command{
	Use:          "show",
	Short:        "Show details for a scale set",
	Long:         `Displays a detailed view of a single scale set.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a scale set ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		getScaleSetReq := apiClientScaleSets.NewGetScaleSetParams()
		getScaleSetReq.ScalesetID = args[0]
		response, err := apiCli.Scalesets.GetScaleSet(getScaleSetReq, authToken)
		if err != nil {
			return err
		}
		formatOneScaleSet(response.Payload)
		return nil
	},
}

var scaleSetDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm", "del"},
	Short:        "Delete scale set by ID",
	Long:         `Delete one scale set by referencing it's ID, regardless of repo or org.`,
	SilenceUsage: true,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("requires a scale set ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		deleteScaleSetReq := apiClientScaleSets.NewDeleteScaleSetParams()
		deleteScaleSetReq.ScalesetID = args[0]
		if err := apiCli.Scalesets.DeleteScaleSet(deleteScaleSetReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

var scaleSetAddCmd = &cobra.Command{
	Use:          "add",
	Aliases:      []string{"create"},
	Short:        "Add scale set",
	Long:         `Add a new scale set.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		newScaleSetParams := params.CreateScaleSetParams{
			RunnerPrefix: params.RunnerPrefix{
				Prefix: scalesetRunnerPrefix,
			},
			ProviderName:           scalesetProvider,
			Name:                   scalesetName,
			MaxRunners:             scalesetMaxRunners,
			MinIdleRunners:         scalesetMinIdleRunners,
			Image:                  scalesetImage,
			Flavor:                 scalesetFlavor,
			OSType:                 commonParams.OSType(scalesetOSType),
			OSArch:                 commonParams.OSArch(scalesetOSArch),
			Enabled:                scalesetEnabled,
			RunnerBootstrapTimeout: scalesetRunnerBootstrapTimeout,
			GitHubRunnerGroup:      scalesetGitHubRunnerGroup,
		}

		if cmd.Flags().Changed("extra-specs") {
			data, err := asRawMessage([]byte(scalesetExtraSpecs))
			if err != nil {
				return err
			}
			newScaleSetParams.ExtraSpecs = data
		}

		if scalesetExtraSpecsFile != "" {
			data, err := extraSpecsFromFile(scalesetExtraSpecsFile)
			if err != nil {
				return err
			}
			newScaleSetParams.ExtraSpecs = data
		}

		if err := newScaleSetParams.Validate(); err != nil {
			return err
		}

		var err error
		var response scalesetPayloadGetter
		if cmd.Flags().Changed("repo") {
			newRepoScaleSetReq := apiClientRepos.NewCreateRepoScaleSetParams()
			newRepoScaleSetReq.RepoID = scalesetRepository
			newRepoScaleSetReq.Body = newScaleSetParams
			response, err = apiCli.Repositories.CreateRepoScaleSet(newRepoScaleSetReq, authToken)
		} else if cmd.Flags().Changed("org") {
			newOrgScaleSetReq := apiClientOrgs.NewCreateOrgScaleSetParams()
			newOrgScaleSetReq.OrgID = scalesetOrganization
			newOrgScaleSetReq.Body = newScaleSetParams
			response, err = apiCli.Organizations.CreateOrgScaleSet(newOrgScaleSetReq, authToken)
		} else if cmd.Flags().Changed("enterprise") {
			newEnterpriseScaleSetReq := apiClientEnterprises.NewCreateEnterpriseScaleSetParams()
			newEnterpriseScaleSetReq.EnterpriseID = scalesetEnterprise
			newEnterpriseScaleSetReq.Body = newScaleSetParams
			response, err = apiCli.Enterprises.CreateEnterpriseScaleSet(newEnterpriseScaleSetReq, authToken)
		} else {
			cmd.Help() //nolint
			os.Exit(0)
		}

		if err != nil {
			return err
		}

		formatOneScaleSet(response.GetPayload())
		return nil
	},
}

var scaleSetUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update one scale set",
	Long: `Updates scale set characteristics.

This command updates the scale set characteristics. Runners already created prior to updating
the scale set, will not be recreated. If they no longer suit your needs, you will need to
explicitly remove them using the runner delete command.
	`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a scale set ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		updateScaleSetReq := apiClientScaleSets.NewUpdateScaleSetParams()
		scaleSetUpdateParams := params.UpdateScaleSetParams{}

		if cmd.Flags().Changed("image") {
			scaleSetUpdateParams.Image = scalesetImage
		}

		if cmd.Flags().Changed("name") {
			scaleSetUpdateParams.Name = scalesetName
		}

		if cmd.Flags().Changed("flavor") {
			scaleSetUpdateParams.Flavor = scalesetFlavor
		}

		if cmd.Flags().Changed("os-type") {
			scaleSetUpdateParams.OSType = commonParams.OSType(scalesetOSType)
		}

		if cmd.Flags().Changed("os-arch") {
			scaleSetUpdateParams.OSArch = commonParams.OSArch(scalesetOSArch)
		}

		if cmd.Flags().Changed("max-runners") {
			scaleSetUpdateParams.MaxRunners = &scalesetMaxRunners
		}

		if cmd.Flags().Changed("min-idle-runners") {
			scaleSetUpdateParams.MinIdleRunners = &scalesetMinIdleRunners
		}

		if cmd.Flags().Changed("runner-prefix") {
			scaleSetUpdateParams.RunnerPrefix = params.RunnerPrefix{
				Prefix: scalesetRunnerPrefix,
			}
		}

		if cmd.Flags().Changed("runner-group") {
			scaleSetUpdateParams.GitHubRunnerGroup = &scalesetGitHubRunnerGroup
		}

		if cmd.Flags().Changed("enabled") {
			scaleSetUpdateParams.Enabled = &scalesetEnabled
		}

		if cmd.Flags().Changed("runner-bootstrap-timeout") {
			scaleSetUpdateParams.RunnerBootstrapTimeout = &scalesetRunnerBootstrapTimeout
		}

		if cmd.Flags().Changed("extra-specs") {
			data, err := asRawMessage([]byte(scalesetExtraSpecs))
			if err != nil {
				return err
			}
			scaleSetUpdateParams.ExtraSpecs = data
		}

		if scalesetExtraSpecsFile != "" {
			data, err := extraSpecsFromFile(scalesetExtraSpecsFile)
			if err != nil {
				return err
			}
			scaleSetUpdateParams.ExtraSpecs = data
		}

		updateScaleSetReq.ScalesetID = args[0]
		updateScaleSetReq.Body = scaleSetUpdateParams
		response, err := apiCli.Scalesets.UpdateScaleSet(updateScaleSetReq, authToken)
		if err != nil {
			return err
		}

		formatOneScaleSet(response.Payload)
		return nil
	},
}

func init() {
	scalesetListCmd.Flags().StringVarP(&scalesetRepository, "repo", "r", "", "List all scale sets within this repository.")
	scalesetListCmd.Flags().StringVarP(&scalesetOrganization, "org", "o", "", "List all scale sets within this organization.")
	scalesetListCmd.Flags().StringVarP(&scalesetEnterprise, "enterprise", "e", "", "List all scale sets within this enterprise.")
	scalesetListCmd.Flags().BoolVarP(&scalesetAll, "all", "a", false, "List all scale sets, regardless of org or repo.")
	scalesetListCmd.MarkFlagsMutuallyExclusive("repo", "org", "all", "enterprise")

	scaleSetUpdateCmd.Flags().StringVar(&scalesetImage, "image", "", "The provider-specific image name to use for runners in this scale set.")
	scaleSetUpdateCmd.Flags().StringVar(&scalesetFlavor, "flavor", "", "The flavor to use for the runners in this scale set.")
	scaleSetUpdateCmd.Flags().StringVar(&scalesetName, "name", "", "The name of the scale set. This option is mandatory.")
	scaleSetUpdateCmd.Flags().StringVar(&scalesetOSType, "os-type", "linux", "Operating system type (windows, linux, etc).")
	scaleSetUpdateCmd.Flags().StringVar(&scalesetOSArch, "os-arch", "amd64", "Operating system architecture (amd64, arm, etc).")
	scaleSetUpdateCmd.Flags().StringVar(&scalesetRunnerPrefix, "runner-prefix", "", "The name prefix to use for runners in this scale set.")
	scaleSetUpdateCmd.Flags().UintVar(&scalesetMaxRunners, "max-runners", 5, "The maximum number of runner this scale set will create.")
	scaleSetUpdateCmd.Flags().UintVar(&scalesetMinIdleRunners, "min-idle-runners", 1, "Attempt to maintain a minimum of idle self-hosted runners of this type.")
	scaleSetUpdateCmd.Flags().StringVar(&scalesetGitHubRunnerGroup, "runner-group", "", "The GitHub runner group in which all runners of this scale set will be added.")
	scaleSetUpdateCmd.Flags().BoolVar(&scalesetEnabled, "enabled", false, "Enable this scale set.")
	scaleSetUpdateCmd.Flags().UintVar(&scalesetRunnerBootstrapTimeout, "runner-bootstrap-timeout", 20, "Duration in minutes after which a runner is considered failed if it does not join Github.")
	scaleSetUpdateCmd.Flags().StringVar(&scalesetExtraSpecsFile, "extra-specs-file", "", "A file containing a valid json which will be passed to the IaaS provider managing the scale set.")
	scaleSetUpdateCmd.Flags().StringVar(&scalesetExtraSpecs, "extra-specs", "", "A valid json which will be passed to the IaaS provider managing the scale set.")
	scaleSetUpdateCmd.MarkFlagsMutuallyExclusive("extra-specs-file", "extra-specs")

	scaleSetAddCmd.Flags().StringVar(&scalesetProvider, "provider-name", "", "The name of the provider where runners will be created.")
	scaleSetAddCmd.Flags().StringVar(&scalesetImage, "image", "", "The provider-specific image name to use for runners in this scale set.")
	scaleSetAddCmd.Flags().StringVar(&scalesetName, "name", "", "The name of the scale set. This option is mandatory.")
	scaleSetAddCmd.Flags().StringVar(&scalesetFlavor, "flavor", "", "The flavor to use for this runner.")
	scaleSetAddCmd.Flags().StringVar(&scalesetRunnerPrefix, "runner-prefix", "", "The name prefix to use for runners in this scale set.")
	scaleSetAddCmd.Flags().StringVar(&scalesetOSType, "os-type", "linux", "Operating system type (windows, linux, etc).")
	scaleSetAddCmd.Flags().StringVar(&scalesetOSArch, "os-arch", "amd64", "Operating system architecture (amd64, arm, etc).")
	scaleSetAddCmd.Flags().StringVar(&scalesetExtraSpecsFile, "extra-specs-file", "", "A file containing a valid json which will be passed to the IaaS provider managing the scale set.")
	scaleSetAddCmd.Flags().StringVar(&scalesetExtraSpecs, "extra-specs", "", "A valid json which will be passed to the IaaS provider managing the scale set.")
	scaleSetAddCmd.Flags().StringVar(&scalesetGitHubRunnerGroup, "runner-group", "", "The GitHub runner group in which all runners of this scale set will be added.")
	scaleSetAddCmd.Flags().UintVar(&scalesetMaxRunners, "max-runners", 5, "The maximum number of runner this scale set will create.")
	scaleSetAddCmd.Flags().UintVar(&scalesetRunnerBootstrapTimeout, "runner-bootstrap-timeout", 20, "Duration in minutes after which a runner is considered failed if it does not join Github.")
	scaleSetAddCmd.Flags().UintVar(&scalesetMinIdleRunners, "min-idle-runners", 1, "Attempt to maintain a minimum of idle self-hosted runners of this type.")
	scaleSetAddCmd.Flags().BoolVar(&scalesetEnabled, "enabled", false, "Enable this scale set.")
	scaleSetAddCmd.MarkFlagRequired("provider-name") //nolint
	scaleSetAddCmd.MarkFlagRequired("name")          //nolint
	scaleSetAddCmd.MarkFlagRequired("image")         //nolint
	scaleSetAddCmd.MarkFlagRequired("flavor")        //nolint

	scaleSetAddCmd.Flags().StringVarP(&scalesetRepository, "repo", "r", "", "Add the new scale set within this repository.")
	scaleSetAddCmd.Flags().StringVarP(&scalesetOrganization, "org", "o", "", "Add the new scale set within this organization.")
	scaleSetAddCmd.Flags().StringVarP(&scalesetEnterprise, "enterprise", "e", "", "Add the new scale set within this enterprise.")
	scaleSetAddCmd.MarkFlagsMutuallyExclusive("repo", "org", "enterprise")
	scaleSetAddCmd.MarkFlagsMutuallyExclusive("extra-specs-file", "extra-specs")

	scalesetCmd.AddCommand(
		scalesetListCmd,
		scaleSetShowCmd,
		scaleSetDeleteCmd,
		scaleSetUpdateCmd,
		scaleSetAddCmd,
	)

	rootCmd.AddCommand(scalesetCmd)
}

func formatScaleSets(scaleSets []params.ScaleSet) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(scaleSets)
		return
	}
	t := table.NewWriter()
	header := table.Row{"ID", "Scale Set Name", "Image", "Flavor", "Belongs to", "Level", "Runner Group", "Enabled", "Runner Prefix", "Provider"}
	t.AppendHeader(header)

	for _, scaleSet := range scaleSets {
		var belongsTo string
		var level string

		switch {
		case scaleSet.RepoID != "" && scaleSet.RepoName != "":
			belongsTo = scaleSet.RepoName
			level = entityTypeRepo
		case scaleSet.OrgID != "" && scaleSet.OrgName != "":
			belongsTo = scaleSet.OrgName
			level = entityTypeOrg
		case scaleSet.EnterpriseID != "" && scaleSet.EnterpriseName != "":
			belongsTo = scaleSet.EnterpriseName
			level = entityTypeEnterprise
		}
		t.AppendRow(table.Row{scaleSet.ID, scaleSet.Name, scaleSet.Image, scaleSet.Flavor, belongsTo, level, scaleSet.GitHubRunnerGroup, scaleSet.Enabled, scaleSet.GetRunnerPrefix(), scaleSet.ProviderName})
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneScaleSet(scaleSet params.ScaleSet) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(scaleSet)
		return
	}
	t := table.NewWriter()
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}

	header := table.Row{"Field", "Value"}

	var belongsTo string
	var level string

	switch {
	case scaleSet.RepoID != "" && scaleSet.RepoName != "":
		belongsTo = scaleSet.RepoName
		level = entityTypeRepo
	case scaleSet.OrgID != "" && scaleSet.OrgName != "":
		belongsTo = scaleSet.OrgName
		level = entityTypeOrg
	case scaleSet.EnterpriseID != "" && scaleSet.EnterpriseName != "":
		belongsTo = scaleSet.EnterpriseName
		level = entityTypeEnterprise
	}

	t.AppendHeader(header)
	t.AppendRow(table.Row{"ID", scaleSet.ID})
	t.AppendRow(table.Row{"Scale Set ID", scaleSet.ScaleSetID})
	t.AppendRow(table.Row{"Scale Name", scaleSet.Name})
	t.AppendRow(table.Row{"Provider Name", scaleSet.ProviderName})
	t.AppendRow(table.Row{"Image", scaleSet.Image})
	t.AppendRow(table.Row{"Flavor", scaleSet.Flavor})
	t.AppendRow(table.Row{"OS Type", scaleSet.OSType})
	t.AppendRow(table.Row{"OS Architecture", scaleSet.OSArch})
	t.AppendRow(table.Row{"Max Runners", scaleSet.MaxRunners})
	t.AppendRow(table.Row{"Min Idle Runners", scaleSet.MinIdleRunners})
	t.AppendRow(table.Row{"Runner Bootstrap Timeout", scaleSet.RunnerBootstrapTimeout})
	t.AppendRow(table.Row{"Belongs to", belongsTo})
	t.AppendRow(table.Row{"Level", level})
	t.AppendRow(table.Row{"Enabled", scaleSet.Enabled})
	t.AppendRow(table.Row{"Runner Prefix", scaleSet.GetRunnerPrefix()})
	t.AppendRow(table.Row{"Extra specs", string(scaleSet.ExtraSpecs)})
	t.AppendRow(table.Row{"GitHub Runner Group", scaleSet.GitHubRunnerGroup})

	if len(scaleSet.Instances) > 0 {
		for _, instance := range scaleSet.Instances {
			t.AppendRow(table.Row{"Instances", fmt.Sprintf("%s (%s)", instance.Name, instance.ID)}, rowConfigAutoMerge)
		}
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}
