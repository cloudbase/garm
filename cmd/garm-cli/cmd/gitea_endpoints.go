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

	"github.com/spf13/cobra"

	apiClientEndpoints "github.com/cloudbase/garm/client/endpoints"
	"github.com/cloudbase/garm/params"
)

var giteaEndpointCmd = &cobra.Command{
	Use:          "endpoint",
	SilenceUsage: true,
	Short:        "Manage Gitea endpoints",
	Long: `Manage Gitea endpoints.

This command allows you to configure and manage Gitea endpoints`,
	Run: nil,
}

var giteaEndpointListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	SilenceUsage: true,
	Short:        "List Gitea endpoints",
	Long:         `List all configured Gitea endpoints.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		newListReq := apiClientEndpoints.NewListGiteaEndpointsParams()
		response, err := apiCli.Endpoints.ListGiteaEndpoints(newListReq, authToken)
		if err != nil {
			return err
		}
		formatEndpoints(response.Payload)
		return nil
	},
}

var giteaEndpointShowCmd = &cobra.Command{
	Use:          "show",
	Aliases:      []string{"get"},
	SilenceUsage: true,
	Short:        "Show Gitea endpoint",
	Long:         `Show details of a Gitea endpoint.`,
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

		newShowReq := apiClientEndpoints.NewGetGiteaEndpointParams()
		newShowReq.Name = args[0]
		response, err := apiCli.Endpoints.GetGiteaEndpoint(newShowReq, authToken)
		if err != nil {
			return err
		}
		formatOneEndpoint(response.Payload)
		return nil
	},
}

var giteaEndpointCreateCmd = &cobra.Command{
	Use:          "create",
	SilenceUsage: true,
	Short:        "Create Gitea endpoint",
	Long:         `Create a new Gitea endpoint.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		createParams, err := parseGiteaCreateParams()
		if err != nil {
			return err
		}

		newCreateReq := apiClientEndpoints.NewCreateGiteaEndpointParams()
		newCreateReq.Body = createParams

		response, err := apiCli.Endpoints.CreateGiteaEndpoint(newCreateReq, authToken)
		if err != nil {
			return err
		}
		formatOneEndpoint(response.Payload)
		return nil
	},
}

var giteaEndpointDeleteCmd = &cobra.Command{
	Use:          "delete",
	Aliases:      []string{"remove", "rm"},
	SilenceUsage: true,
	Short:        "Delete Gitea endpoint",
	Long:         "Delete a Gitea endpoint",
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

		newDeleteReq := apiClientEndpoints.NewDeleteGiteaEndpointParams()
		newDeleteReq.Name = args[0]
		if err := apiCli.Endpoints.DeleteGiteaEndpoint(newDeleteReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

var giteaEndpointUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update Gitea endpoint",
	Long:         "Update a Gitea endpoint",
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

		updateParams := params.UpdateGiteaEndpointParams{}

		if cmd.Flags().Changed("ca-cert-path") {
			cert, err := parseAndReadCABundle()
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

		if cmd.Flags().Changed("api-base-url") {
			updateParams.APIBaseURL = &endpointAPIBaseURL
		}

		newEndpointUpdateReq := apiClientEndpoints.NewUpdateGiteaEndpointParams()
		newEndpointUpdateReq.Name = args[0]
		newEndpointUpdateReq.Body = updateParams

		response, err := apiCli.Endpoints.UpdateGiteaEndpoint(newEndpointUpdateReq, authToken)
		if err != nil {
			return err
		}
		formatOneEndpoint(response.Payload)
		return nil
	},
}

func init() {
	giteaEndpointCreateCmd.Flags().StringVar(&endpointName, "name", "", "Name of the Gitea endpoint")
	giteaEndpointCreateCmd.Flags().StringVar(&endpointDescription, "description", "", "Description for the github endpoint")
	giteaEndpointCreateCmd.Flags().StringVar(&endpointBaseURL, "base-url", "", "Base URL of the Gitea endpoint")
	giteaEndpointCreateCmd.Flags().StringVar(&endpointAPIBaseURL, "api-base-url", "", "API Base URL of the Gitea endpoint")
	giteaEndpointCreateCmd.Flags().StringVar(&endpointCACertPath, "ca-cert-path", "", "CA Cert Path of the Gitea endpoint")

	giteaEndpointListCmd.Flags().BoolVarP(&long, "long", "l", false, "Include additional info.")

	giteaEndpointCreateCmd.MarkFlagRequired("name")
	giteaEndpointCreateCmd.MarkFlagRequired("base-url")
	giteaEndpointCreateCmd.MarkFlagRequired("api-base-url")

	giteaEndpointUpdateCmd.Flags().StringVar(&endpointDescription, "description", "", "Description for the gitea endpoint")
	giteaEndpointUpdateCmd.Flags().StringVar(&endpointBaseURL, "base-url", "", "Base URL of the Gitea endpoint")
	giteaEndpointUpdateCmd.Flags().StringVar(&endpointAPIBaseURL, "api-base-url", "", "API Base URL of the Gitea endpoint")
	giteaEndpointUpdateCmd.Flags().StringVar(&endpointCACertPath, "ca-cert-path", "", "CA Cert Path of the Gitea endpoint")

	giteaEndpointCmd.AddCommand(
		giteaEndpointListCmd,
		giteaEndpointShowCmd,
		giteaEndpointCreateCmd,
		giteaEndpointDeleteCmd,
		giteaEndpointUpdateCmd,
	)

	giteaCmd.AddCommand(giteaEndpointCmd)
}

func parseGiteaCreateParams() (params.CreateGiteaEndpointParams, error) {
	certBundleBytes, err := parseAndReadCABundle()
	if err != nil {
		return params.CreateGiteaEndpointParams{}, err
	}

	ret := params.CreateGiteaEndpointParams{
		Name:         endpointName,
		BaseURL:      endpointBaseURL,
		APIBaseURL:   endpointAPIBaseURL,
		Description:  endpointDescription,
		CACertBundle: certBundleBytes,
	}
	return ret, nil
}
