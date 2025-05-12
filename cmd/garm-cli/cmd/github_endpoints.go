package cmd

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiClientEndpoints "github.com/cloudbase/garm/client/endpoints"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var githubEndpointCmd = &cobra.Command{
	Use:          "endpoint",
	SilenceUsage: true,
	Short:        "Manage GitHub endpoints",
	Long: `Manage GitHub endpoints.

This command allows you to configure and manage GitHub endpoints`,
	Run: nil,
}

var githubEndpointListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	SilenceUsage: true,
	Short:        "List GitHub endpoints",
	Long:         `List all configured GitHub endpoints.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		newGHListReq := apiClientEndpoints.NewListGithubEndpointsParams()
		response, err := apiCli.Endpoints.ListGithubEndpoints(newGHListReq, authToken)
		if err != nil {
			return err
		}
		formatEndpoints(response.Payload)
		return nil
	},
}

var githubEndpointShowCmd = &cobra.Command{
	Use:          "show",
	Aliases:      []string{"get"},
	SilenceUsage: true,
	Short:        "Show GitHub endpoint",
	Long:         `Show details of a GitHub endpoint.`,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires an endpoint name")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		newGHShowReq := apiClientEndpoints.NewGetGithubEndpointParams()
		newGHShowReq.Name = args[0]
		response, err := apiCli.Endpoints.GetGithubEndpoint(newGHShowReq, authToken)
		if err != nil {
			return err
		}
		formatOneEndpoint(response.Payload)
		return nil
	},
}

var githubEndpointCreateCmd = &cobra.Command{
	Use:          "create",
	SilenceUsage: true,
	Short:        "Create GitHub endpoint",
	Long:         `Create a new GitHub endpoint.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		createParams, err := parseCreateParams()
		if err != nil {
			return err
		}

		newGHCreateReq := apiClientEndpoints.NewCreateGithubEndpointParams()
		newGHCreateReq.Body = createParams

		response, err := apiCli.Endpoints.CreateGithubEndpoint(newGHCreateReq, authToken)
		if err != nil {
			return err
		}
		formatOneEndpoint(response.Payload)
		return nil
	},
}

var githubEndpointDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm"},
	SilenceUsage: true,
	Short:        "Delete GitHub endpoint",
	Long:         "Delete a GitHub endpoint",
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires an endpoint name")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		newGHDeleteReq := apiClientEndpoints.NewDeleteGithubEndpointParams()
		newGHDeleteReq.Name = args[0]
		if err := apiCli.Endpoints.DeleteGithubEndpoint(newGHDeleteReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

var githubEndpointUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update GitHub endpoint",
	Long:         "Update a GitHub endpoint",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}
		if len(args) == 0 {
			return fmt.Errorf("requires an endpoint name")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		updateParams := params.UpdateGithubEndpointParams{}

		if cmd.Flags().Changed("ca-cert-path") {
			cert, err := parseReadAndParsCABundle()
			if err != nil {
				return err
			}
			updateParams.CACertBundle = cert
		}

		if cmd.Flags().Changed("description") {
			updateParams.Description = &endpointDescription
		}

		if cmd.Flags().Changed("base-url") {
			updateParams.BaseURL = &endpointBaseURL
		}

		if cmd.Flags().Changed("upload-url") {
			updateParams.UploadBaseURL = &endpointUploadURL
		}

		if cmd.Flags().Changed("api-base-url") {
			updateParams.APIBaseURL = &endpointAPIBaseURL
		}

		newGHEndpointUpdateReq := apiClientEndpoints.NewUpdateGithubEndpointParams()
		newGHEndpointUpdateReq.Name = args[0]
		newGHEndpointUpdateReq.Body = updateParams

		response, err := apiCli.Endpoints.UpdateGithubEndpoint(newGHEndpointUpdateReq, authToken)
		if err != nil {
			return err
		}
		formatOneEndpoint(response.Payload)
		return nil
	},
}

func init() {
	githubEndpointCreateCmd.Flags().StringVar(&endpointName, "name", "", "Name of the GitHub endpoint")
	githubEndpointCreateCmd.Flags().StringVar(&endpointDescription, "description", "", "Description for the github endpoint")
	githubEndpointCreateCmd.Flags().StringVar(&endpointBaseURL, "base-url", "", "Base URL of the GitHub endpoint")
	githubEndpointCreateCmd.Flags().StringVar(&endpointUploadURL, "upload-url", "", "Upload URL of the GitHub endpoint")
	githubEndpointCreateCmd.Flags().StringVar(&endpointAPIBaseURL, "api-base-url", "", "API Base URL of the GitHub endpoint")
	githubEndpointCreateCmd.Flags().StringVar(&endpointCACertPath, "ca-cert-path", "", "CA Cert Path of the GitHub endpoint")

	githubEndpointListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")

	githubEndpointCreateCmd.MarkFlagRequired("name")
	githubEndpointCreateCmd.MarkFlagRequired("base-url")
	githubEndpointCreateCmd.MarkFlagRequired("api-base-url")
	githubEndpointCreateCmd.MarkFlagRequired("upload-url")

	githubEndpointUpdateCmd.Flags().StringVar(&endpointDescription, "description", "", "Description for the github endpoint")
	githubEndpointUpdateCmd.Flags().StringVar(&endpointBaseURL, "base-url", "", "Base URL of the GitHub endpoint")
	githubEndpointUpdateCmd.Flags().StringVar(&endpointUploadURL, "upload-url", "", "Upload URL of the GitHub endpoint")
	githubEndpointUpdateCmd.Flags().StringVar(&endpointAPIBaseURL, "api-base-url", "", "API Base URL of the GitHub endpoint")
	githubEndpointUpdateCmd.Flags().StringVar(&endpointCACertPath, "ca-cert-path", "", "CA Cert Path of the GitHub endpoint")

	githubEndpointCmd.AddCommand(
		githubEndpointListCmd,
		githubEndpointShowCmd,
		githubEndpointCreateCmd,
		githubEndpointDeleteCmd,
		githubEndpointUpdateCmd,
	)

	githubCmd.AddCommand(githubEndpointCmd)
}

func parseReadAndParsCABundle() ([]byte, error) {
	if endpointCACertPath == "" {
		return nil, nil
	}

	if _, err := os.Stat(endpointCACertPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("CA cert file not found: %s", endpointCACertPath)
	}
	contents, err := os.ReadFile(endpointCACertPath)
	if err != nil {
		return nil, err
	}
	pemBlock, _ := pem.Decode(contents)
	if pemBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	if _, err := x509.ParseCertificates(pemBlock.Bytes); err != nil {
		return nil, fmt.Errorf("failed to parse CA cert bundle: %w", err)
	}
	return contents, nil
}

func parseCreateParams() (params.CreateGithubEndpointParams, error) {
	certBundleBytes, err := parseReadAndParsCABundle()
	if err != nil {
		return params.CreateGithubEndpointParams{}, err
	}

	ret := params.CreateGithubEndpointParams{
		Name:          endpointName,
		BaseURL:       endpointBaseURL,
		UploadBaseURL: endpointUploadURL,
		APIBaseURL:    endpointAPIBaseURL,
		Description:   endpointDescription,
		CACertBundle:  certBundleBytes,
	}
	return ret, nil
}

func formatEndpoints(endpoints params.ForgeEndpoints) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(endpoints)
		return
	}
	t := table.NewWriter()
	header := table.Row{"Name", "Base URL", "Description"}
	if long {
		header = append(header, "Created At", "Updated At")
	}
	t.AppendHeader(header)
	for _, val := range endpoints {
		row := table.Row{val.Name, val.BaseURL, val.Description}
		if long {
			row = append(row, val.CreatedAt, val.UpdatedAt)
		}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}

func formatOneEndpoint(endpoint params.ForgeEndpoint) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(endpoint)
		return
	}
	t := table.NewWriter()
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRow([]interface{}{"Name", endpoint.Name})
	t.AppendRow([]interface{}{"Description", endpoint.Description})
	t.AppendRow([]interface{}{"Created At", endpoint.CreatedAt})
	t.AppendRow([]interface{}{"Updated At", endpoint.UpdatedAt})
	t.AppendRow([]interface{}{"Base URL", endpoint.BaseURL})
	t.AppendRow([]interface{}{"Upload URL", endpoint.UploadBaseURL})
	t.AppendRow([]interface{}{"API Base URL", endpoint.APIBaseURL})
	if len(endpoint.CACertBundle) > 0 {
		t.AppendRow([]interface{}{"CA Cert Bundle", string(endpoint.CACertBundle)})
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}
