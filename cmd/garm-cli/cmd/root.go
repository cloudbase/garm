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
	"net/url"
	"os"

	"github.com/go-openapi/runtime"
	"github.com/jedib0t/go-pretty/v6/table"


	openapiRuntimeClient "github.com/go-openapi/runtime/client"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiClient "github.com/cloudbase/garm/client"
	"github.com/cloudbase/garm/cmd/garm-cli/config"
	"github.com/cloudbase/garm/params"
)

var Version string

var (
	cfg               *config.Config
	mgr               config.Manager
	apiCli            *apiClient.GarmAPI
	authToken         runtime.ClientAuthInfoWriter
	needsInit         bool
	debug             bool
	errNeedsInitError = fmt.Errorf("please log into a garm installation first")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "garm-cli",
	Short: "Runner manager CLI app",
	Long:  `CLI for the github self hosted runners manager.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug on all API calls")
	cobra.OnInitialize(initConfig)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func initApiClient(baseUrl, token string) {
	baseUrlParsed, err := url.Parse(baseUrl)
	if err != nil {
		fmt.Printf("Failed to parse base url %s: %s", baseUrl, err)
		os.Exit(1)
	}
	apiPath, err := url.JoinPath(baseUrlParsed.Path, apiClient.DefaultBasePath)
	if err != nil {
		fmt.Printf("Failed to join base url path %s with %s: %s", baseUrlParsed.Path, apiClient.DefaultBasePath, err)
		os.Exit(1)
	}
	if debug {
		os.Setenv("SWAGGER_DEBUG", "true")
	}
	transportCfg := apiClient.DefaultTransportConfig().
		WithHost(baseUrlParsed.Host).
		WithBasePath(apiPath).
		WithSchemes([]string{baseUrlParsed.Scheme})
	apiCli = apiClient.NewHTTPClientWithConfig(nil, transportCfg)
	authToken = openapiRuntimeClient.BearerToken(token)
}

func initConfig() {
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %s", err)
		os.Exit(1)
	}
	if len(cfg.Managers) == 0 {
		// config is empty.
		needsInit = true
	} else {
		mgr, err = cfg.GetActiveConfig()
		if err != nil {
			mgr = cfg.Managers[0]
		}
	}
	initApiClient(mgr.BaseURL, mgr.Token)
}

func formatOneHookInfo(hook params.HookInfo) {
	t := table.NewWriter()
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)
	t.AppendRows([]table.Row{
		{"ID", hook.ID},
		{"URL", hook.URL},
		{"Events", hook.Events},
		{"Active", hook.Active},
		{"Insecure SSL", hook.InsecureSSL},
	})
	fmt.Println(t.Render())
}
