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
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	apiserverParams "github.com/cloudbase/garm/apiserver/params"
	apiTemplates "github.com/cloudbase/garm/client/templates"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/cmd/garm-cli/editor"
	"github.com/cloudbase/garm/params"
)

var (
	templateName         string
	templatePath         string
	templateOSType       string
	templateForgeType    string
	templateDescription  string
	templateEditExternal bool
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

		if len(args) != 1 {
			return fmt.Errorf("invalid positional parameters; requires template_id")
		}

		tplID, err := resolveTemplateAsUint(args[0])
		if err != nil {
			return fmt.Errorf("failed to determine template ID: %s", err)
		}

		if cmd.Flags().Changed("name") {
			updateReq.Body.Name = &templateName
			changes = true
		}
		if cmd.Flags().Changed("description") {
			updateReq.Body.Description = &templateDescription
			changes = true
		}

		if cmd.Flags().Changed("path") {
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

		updateReq.TemplateID = float64(tplID)

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
	RunE: func(_ *cobra.Command, args []string) error {
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

		getReq := apiTemplates.NewGetTemplateParams()
		getReq.TemplateID = tplID

		response, err := apiCli.Templates.GetTemplate(getReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to get template: %q", err)
		}

		if _, err := os.Stat(templatePath); err == nil {
			return fmt.Errorf("destination path already exists; will not overwrite")
		}

		if err := os.WriteFile(templatePath, response.Payload.Data, 0o600); err != nil {
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

var templateRestoreCmd = &cobra.Command{
	Use:          "restore",
	SilenceUsage: true,
	Short:        "Restore system templates",
	Long: `Restore built-in system templates.

By default, restores all system templates. Use --forge-type and --os-type to
restore only a specific template. When either flag is specified, both must be provided.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		restoreReq := apiTemplates.NewRestoreTemplatesParams()

		forgeChanged := cmd.Flags().Changed("forge-type")
		osChanged := cmd.Flags().Changed("os-type")

		if forgeChanged != osChanged {
			return fmt.Errorf("both --forge-type and --os-type must be specified together")
		}

		if forgeChanged {
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
				return fmt.Errorf("invalid OS type: %q (supported: %s)", osType, strings.Join([]string{string(commonParams.Linux), string(commonParams.Windows)}, ", "))
			}

			restoreReq.Body.Forge = forge
			restoreReq.Body.OSType = osType
		} else {
			restoreReq.Body.RestoreAll = true
		}

		if err := apiCli.Templates.RestoreTemplates(restoreReq, authToken); err != nil {
			return fmt.Errorf("failed to restore templates: %s", err)
		}
		fmt.Println("templates restored successfully")
		return nil
	},
}

var templateEditCmd = &cobra.Command{
	Use:          "edit [flags] template_name_or_id",
	SilenceUsage: true,
	Short:        "Edit runner install templates",
	Long: `Edit templates in the built-in editor.

The built-in editor supports optional vim keybindings (F2 or Alt+V toggles
them; the choice is remembered in the CLI config). Saving from the editor
(Ctrl+S or :w) updates the template directly. With --external the template
is opened in $EDITOR instead.`,
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

		getReq := apiTemplates.NewGetTemplateParams()
		getReq.TemplateID = tplID

		response, err := apiCli.Templates.GetTemplate(getReq, authToken)
		if err != nil {
			return fmt.Errorf("failed to get source template: %q", err)
		}

		original := string(response.Payload.Data)
		updateTemplate := func(text string) error {
			updateReq := apiTemplates.NewUpdateTemplateParams()
			updateReq.TemplateID = float64(response.Payload.ID)
			updateReq.Body.Data = []byte(text)
			if _, err := apiCli.Templates.UpdateTemplate(updateReq, authToken); err != nil {
				// Surface the API's error detail (e.g. the template parse error)
				// rather than the raw go-swagger response blob. The built-in
				// editor reports this in place, keeping the editor open to fix.
				return errors.New(templateAPIErrorMessage(err))
			}
			return nil
		}

		if templateEditExternal {
			// The external editor closes after each pass, so on a failed save we
			// show the error and offer to reopen with the user's edits intact.
			toEdit := original
			for {
				newContent, _, err := editWithExternalEditor(toEdit, string(response.Payload.OSType))
				if err != nil {
					return err
				}
				if newContent == original {
					fmt.Println("no changes made")
					return nil
				}
				if err := updateTemplate(newContent); err != nil {
					fmt.Fprintf(os.Stderr, "\ntemplate not saved: %s\nHit enter to retry, ctrl-c to cancel edit ", err)
					if _, rerr := bufio.NewReader(os.Stdin).ReadString('\n'); rerr != nil {
						// stdin closed (e.g. non-interactive) — nothing to retry with.
						return errors.New("template not saved")
					}
					toEdit = newContent
					continue
				}
				fmt.Println("changes saved successfully")
				return nil
			}
		}

		ed := editor.NewEditor()
		ed.SetSyntax(editor.SyntaxForOSType(string(response.Payload.OSType)))
		ed.SetTitle(fmt.Sprintf("Template: %s", response.Payload.Name))
		ed.SetVimMode(cfg.EditorUseVim)
		// Saving inside the editor (Ctrl+S, :w, :wq) pushes the update
		// directly, so nothing needs to happen after EditText returns.
		ed.SetSaveHandler(updateTemplate)

		_, saved, err := ed.EditText(original)
		if err != nil {
			return fmt.Errorf("failed to open editor: %s", err)
		}

		// Remember the vim preference across sessions.
		if ed.VimEnabled() != cfg.EditorUseVim {
			cfg.EditorUseVim = ed.VimEnabled()
			if err := cfg.SaveConfig(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save editor preference: %s\n", err)
			}
		}

		switch {
		case saved:
			fmt.Println("changes saved successfully")
		case ed.DiscardedChanges():
			fmt.Println("changes discarded")
		default:
			fmt.Println("no changes made")
		}
		return nil
	},
}

// editWithExternalEditor writes the template to a temporary file, opens it
// in $EDITOR (falling back to vi) and reads the result back.
func editWithExternalEditor(content, osType string) (string, bool, error) {
	ext := ".sh"
	if strings.EqualFold(osType, "windows") {
		ext = ".ps1"
	}
	tmp, err := os.CreateTemp("", "garm-template-*"+ext)
	if err != nil {
		return "", false, fmt.Errorf("failed to create temporary file: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return "", false, fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", false, fmt.Errorf("failed to write temporary file: %w", err)
	}

	editorCmd := os.Getenv("EDITOR")
	if editorCmd == "" {
		editorCmd = "vi"
	}
	// $EDITOR may carry arguments (e.g. "code --wait").
	parts := strings.Fields(editorCmd)
	cmd := exec.Command(parts[0], append(parts[1:], path)...) // #nosec G204,G702 -- $EDITOR is user-controlled by design
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", false, fmt.Errorf("editor %q failed: %w", editorCmd, err)
	}

	edited, err := os.ReadFile(path) // #nosec G304,G703 -- our own temp file
	if err != nil {
		return "", false, fmt.Errorf("failed to read edited file: %w", err)
	}
	return string(edited), string(edited) != content, nil
}

// templateAPIErrorMessage extracts the human-readable message from a template
// API error, preferring the structured APIErrorResponse detail (already
// unmarshaled by the client) over the raw go-swagger response string.
func templateAPIErrorMessage(err error) string {
	var updateErr *apiTemplates.UpdateTemplateDefault
	if errors.As(err, &updateErr) {
		if msg := apiErrorResponseMessage(updateErr.Payload); msg != "" {
			return msg
		}
	}
	var createErr *apiTemplates.CreateTemplateDefault
	if errors.As(err, &createErr) {
		if msg := apiErrorResponseMessage(createErr.Payload); msg != "" {
			return msg
		}
	}
	return err.Error()
}

func apiErrorResponseMessage(p apiserverParams.APIErrorResponse) string {
	switch {
	case p.Details != "":
		return p.Details
	case p.Error != "":
		return p.Error
	default:
		return ""
	}
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

	templateEditCmd.Flags().BoolVar(&templateEditExternal, "external", false, "Edit in $EDITOR (falls back to vi) instead of the built-in editor.")

	templateCopyCmd.Flags().StringVar(&templateDescription, "description", "", "Template description.")

	templateRestoreCmd.Flags().StringVar(&templateForgeType, "forge-type", "", "The forge type of the template. Supported values: github, gitea.")
	templateRestoreCmd.Flags().StringVar(&templateOSType, "os-type", "", "Operating system type (windows, linux).")

	templatesCmd.AddCommand(templateCreateCmd)
	templatesCmd.AddCommand(templateShowCmd)
	templatesCmd.AddCommand(templateListCmd)
	templatesCmd.AddCommand(templateUpdateCmd)
	templatesCmd.AddCommand(templateDeleteCmd)
	templatesCmd.AddCommand(templateCopyCmd)
	templatesCmd.AddCommand(templateEditCmd)
	templatesCmd.AddCommand(templateDownloadCmd)
	templatesCmd.AddCommand(templateRestoreCmd)
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
