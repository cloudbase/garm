// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package cmd

import (
	"fmt"
	"os"
	"strings"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiTemplates "github.com/cloudbase/garm/client/templates"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/cmd/garm-cli/editor"
	"github.com/cloudbase/garm/params"
)

var (
	templateName        string
	templatePath        string
	templateOSType      string
	templateForgeType   string
	templateDescription string
)

// templatesCmd represents the the templates command.
var templatesCmd = &cobra.Command{
	Use:          "template",
	SilenceUsage: true,
	Short:        "Manage templates",
	Long: `Manage runner install templates.

The commands in this group enable you to manage github and gitea runner install templates for both Linux and Windows.
Templates are a convenience feature that allows providers to point the userdata of the new runner to an URL in GARM
which will serve an OS specific script (catered to the runner type) that will set up the runner software on a new
generic machine.

Templates give users the flexibility to easily change and manage runner install scripts without setting the entire
template body in extra_specs. Think of it as an easier way to manage runner install scripts that allows you to keep
the templates in GARM itself instead of keeping track of multiple files written for various pools or scale sets.
`,
	Run: nil,
}

var templateCreateCmd = &cobra.Command{
	Use:          "create",
	Aliases:      []string{"add"},
	SilenceUsage: true,
	Short:        "Create a new template",
	Long:         `Create a new runner install template.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		forge := params.EndpointType(templateForgeType)
		switch forge {
		case params.GithubEndpointType, params.GiteaEndpointType:
		default:
			return fmt.Errorf("invalid forge type: %q (supported: %s)", forge, strings.Join([]string{string(params.GithubEndpointType), string(params.GiteaEndpointType)}, ", "))
		}

		osType := commonParams.OSType(templateOSType)
		switch osType {
		case commonParams.Linux, commonParams.Windows:
		default:
			return fmt.Errorf("invalid OS type: %q (supported: %s)", osType, strings.Join([]string{string(params.GithubEndpointType), string(params.GiteaEndpointType)}, ", "))
		}

		if templatePath == "" {
			return fmt.Errorf("missing template path")
		}

		mode, err := os.Stat(templatePath)
		if err != nil {
			return fmt.Errorf("failed to access %s: %q", templatePath, err)
		}
		if mode.Size() > 1<<20 {
			return fmt.Errorf("script is larger than 1 MB")
		}
		data, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template file: %q", err)
		}

		createTemplateReq := apiTemplates.NewCreateTemplateParams()
		createTemplateReq.Body.Name = templateName
		createTemplateReq.Body.ForgeType = forge
		createTemplateReq.Body.OSType = osType
		createTemplateReq.Body.Description = templateDescription
		createTemplateReq.Body.Data = data

		response, err := apiCli.Templates.CreateTemplate(createTemplateReq, authToken)
		if err != nil {
			return err
		}
		formatOneTemplate(response.Payload)
		return nil
	},
}

var templateUpdateCmd = &cobra.Command{
	Use:          "update [flags] template_id",
	SilenceUsage: true,
	Short:        "Update template",
	Long:         `Update a runner install template.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		updateReq := apiTemplates.NewUpdateTemplateParams()

		var changes bool

		if cmd.Flags().Changed("name") {
			updateReq.Body.Name = &templateName
			changes = true
		}
		if cmd.Flags().Changed("description") {
			updateReq.Body.Description = &templateDescription
			changes = true
		}

		if cmd.Flags().Changed("description") {
			mode, err := os.Stat(templatePath)
			if err != nil {
				return fmt.Errorf("failed to access %s: %q", templatePath, err)
			}
			if mode.Size() > 1<<20 {
				return fmt.Errorf("script is larger than 1 MB")
			}
			data, err := os.ReadFile(templatePath)
			if err != nil {
				return fmt.Errorf("failed to read template file: %q", err)
			}
			updateReq.Body.Data = data
			changes = true
		}
		if !changes {
			return fmt.Errorf("at least one of name, description or path must be specified")
		}
		if len(args) != 1 {
			return fmt.Errorf("invalid positional parameters; requires template_id")
		}

		updateReq.TemplateID = args[0]

		response, err := apiCli.Templates.UpdateTemplate(updateReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to update template: %q", err)
		}

		formatOneTemplate(response.Payload)
		return nil
	},
}

var templateListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	SilenceUsage: true,
	Short:        "List templates",
	Long:         `List available runner install templates.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listReq := apiTemplates.NewListTemplatesParams()

		if cmd.Flags().Changed("name") {
			listReq.PartialName = &templateName
		}

		if cmd.Flags().Changed("os-type") {
			listReq.OsType = &templateOSType
		}

		if cmd.Flags().Changed("forge-type") {
			listReq.ForgeType = &templateForgeType
		}

		response, err := apiCli.Templates.ListTemplates(listReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to list templates: %q", err)
		}
		formatTemplateList(response.Payload)
		return nil
	},
}

var templateShowCmd = &cobra.Command{
	Use:          "show [flags] template_name_or_id",
	SilenceUsage: true,
	Short:        "Show template",
	Long:         `Show template details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) != 1 {
			return fmt.Errorf("invalid number of parameters; requires template_name_or_id")
		}

		tplID, err := resolveTemplate(args[0])
		if err != nil {
			return fmt.Errorf("failed to determine template ID: %s", err)
		}

		getReq := apiTemplates.NewGetTemplateParams()
		getReq.TemplateID = tplID

		response, err := apiCli.Templates.GetTemplate(getReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to get template: %q", err)
		}
		formatOneTemplate(response.Payload)
		return nil
	},
}

var templateDownloadCmd = &cobra.Command{
	Use:          "download [flags] template_name_or_id",
	SilenceUsage: true,
	Short:        "Download template",
	Long:         `Download a specific template to a file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) != 1 {
			return fmt.Errorf("invalid number of parameters; requires template_name_or_id")
		}

		tplID, err := resolveTemplate(args[0])
		if err != nil {
			return fmt.Errorf("failed to determine template ID: %q", err)
		}

		getReq := apiTemplates.NewGetTemplateParams()
		getReq.TemplateID = tplID

		response, err := apiCli.Templates.GetTemplate(getReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to get template: %q", err)
		}

		if _, err := os.Stat(templatePath); err == nil {
			return fmt.Errorf("destination path already exists; will not overwrite")
		}

		if err := os.WriteFile(templatePath, response.Payload.Data, 0o640); err != nil {
			return fmt.Errorf("failed to save file %s: %s", templatePath, err)
		}
		return nil
	},
}

var templateDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm"},
	SilenceUsage: true,
	Short:        "Delete template",
	Long:         `Delete a specific template.`,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) != 1 {
			return fmt.Errorf("invalid number of parameters; requires template_name_or_id")
		}

		tplID, err := resolveTemplate(args[0])
		if err != nil {
			return fmt.Errorf("failed to determine template ID: %q", err)
		}

		deleteReq := apiTemplates.NewDeleteTemplateParams()
		deleteReq.TemplateID = tplID

		if err := apiCli.Templates.DeleteTemplate(deleteReq, authToken); err != nil {
			return fmt.Errorf("failed to delete template: %s", err)
		}
		return nil
	},
}

var templateCopyCmd = &cobra.Command{
	Use:          "copy [flags] source_template new_name",
	Aliases:      []string{"clone", "cp"},
	SilenceUsage: true,
	Short:        "Clone a template",
	Long:         `Create a new template using an existing template as a source.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) != 2 {
			return fmt.Errorf("invalid number of parameters; requires source_template and new_name")
		}

		tplID, err := resolveTemplate(args[0])
		if err != nil {
			return fmt.Errorf("failed to determine template ID: %q", err)
		}

		getReq := apiTemplates.NewGetTemplateParams()
		getReq.TemplateID = tplID

		response, err := apiCli.Templates.GetTemplate(getReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to get source template: %q", err)
		}

		createTemplateReq := apiTemplates.NewCreateTemplateParams()
		createTemplateReq.Body.Data = response.Payload.Data
		createTemplateReq.Body.ForgeType = response.Payload.ForgeType
		createTemplateReq.Body.OSType = response.Payload.OSType

		createTemplateReq.Body.Name = args[1]

		if cmd.Flags().Changed("description") {
			createTemplateReq.Body.Description = templateDescription
		} else {
			createTemplateReq.Body.Description = response.Payload.Description
		}

		newResponse, err := apiCli.Templates.CreateTemplate(createTemplateReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to create template: %s", err)
		}
		formatOneTemplate(newResponse.Payload)
		return nil
	},
}

var templateEditCmd = &cobra.Command{
	Use:          "edit [flags] template_name_or_id",
	SilenceUsage: true,
	Short:        "Edit runner install templates",
	Long:         `Edit templates with optional basic vim keybindings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) != 1 {
			return fmt.Errorf("invalid number of parameters; requires template_name_or_id")
		}

		tplID, err := resolveTemplate(args[0])
		if err != nil {
			return fmt.Errorf("failed to determine template ID: %q", err)
		}

		getReq := apiTemplates.NewGetTemplateParams()
		getReq.TemplateID = tplID

		response, err := apiCli.Templates.GetTemplate(getReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to get source template: %q", err)
		}

		ed := editor.NewEditor()

		newContent, saved, err := ed.EditText(string(response.Payload.Data))
		if err != nil {
			return fmt.Errorf("failed to open editor: %s", err)
		}

		if saved && newContent != string(response.Payload.Data) {
			updateReq := apiTemplates.NewUpdateTemplateParams()
			updateReq.TemplateID = fmt.Sprintf("%d", response.Payload.ID)
			updateReq.Body.Data = []byte(newContent)

			_, err = apiCli.Templates.UpdateTemplate(updateReq, authToken)
			if err != nil {
				return fmt.Errorf("failed to update template: %s", err)
			}
			fmt.Println("changes saved successfully")
		} else {
			fmt.Println("changes discarded")
		}
		return nil
	},
}

func init() {
	templateCreateCmd.Flags().StringVar(&templateName, "name", "", "Name of the template.")
	templateCreateCmd.Flags().StringVar(&templateDescription, "description", "", "Template description.")
	templateCreateCmd.Flags().StringVar(&templatePath, "path", "", "Path on disk to the template.")
	templateCreateCmd.Flags().StringVar(&templateForgeType, "forge-type", "", "The forge type of the template. Supported values: github, gitea.")
	templateCreateCmd.Flags().StringVar(&templateOSType, "os-type", "", "Operating system type (windows, linux, etc).")

	templateCreateCmd.MarkFlagRequired("name")       //nolint
	templateCreateCmd.MarkFlagRequired("path")       //nolint
	templateCreateCmd.MarkFlagRequired("forge-type") //nolint
	templateCreateCmd.MarkFlagRequired("os-type")    //nolint

	templateUpdateCmd.Flags().StringVar(&templateName, "name", "", "Name of the template.")
	templateUpdateCmd.Flags().StringVar(&templateDescription, "description", "", "Template description.")
	templateUpdateCmd.Flags().StringVar(&templatePath, "path", "", "Path on disk to the template.")

	templateListCmd.Flags().StringVar(&templateName, "name", "", "Full or partial name to search by.")
	templateListCmd.Flags().StringVar(&templateForgeType, "forge-type", "", "The forge type of the template. Supported values: github, gitea.")
	templateListCmd.Flags().StringVar(&templateOSType, "os-type", "", "Operating system type (windows, linux, etc).")

	templateDownloadCmd.Flags().StringVar(&templatePath, "path", "", "Destination path for the download.")
	templateDownloadCmd.MarkFlagRequired("path") //nolint

	templateCopyCmd.Flags().StringVar(&templateDescription, "description", "", "Template description.")

	templatesCmd.AddCommand(templateCreateCmd)
	templatesCmd.AddCommand(templateShowCmd)
	templatesCmd.AddCommand(templateListCmd)
	templatesCmd.AddCommand(templateUpdateCmd)
	templatesCmd.AddCommand(templateDeleteCmd)
	templatesCmd.AddCommand(templateCopyCmd)
	templatesCmd.AddCommand(templateEditCmd)
	templatesCmd.AddCommand(templateDownloadCmd)
	rootCmd.AddCommand(templatesCmd)
}

func formatOneTemplate(template params.Template) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(template)
		return
	}
	t := table.NewWriter()
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)

	t.AppendRow(table.Row{"ID", template.ID})
	t.AppendRow(table.Row{"Created At", template.CreatedAt})
	t.AppendRow(table.Row{"Updated At", template.UpdatedAt})
	t.AppendRow(table.Row{"Name", template.Name})
	t.AppendRow(table.Row{"Description", template.Description})
	t.AppendRow(table.Row{"Owner", template.Owner})
	t.AppendRow(table.Row{"Forge Type", template.ForgeType})
	t.AppendRow(table.Row{"OS Type", template.OSType})

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}

func formatTemplateList(templates params.Templates) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(templates)
		return
	}
	t := table.NewWriter()
	header := table.Row{"ID", "Name", "Description", "Forge Type", "OS Type", "Owner"}
	t.AppendHeader(header)
	for _, val := range templates {
		row := table.Row{val.ID, val.Name, val.Description, val.ForgeType, val.OSType, val.Owner}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}
