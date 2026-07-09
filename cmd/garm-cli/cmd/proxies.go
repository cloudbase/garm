// Copyright 2026 Cloudbase Solutions SRL
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

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	apiClientProxies "github.com/cloudbase/garm/client/proxies"
	"github.com/cloudbase/garm/cmd/garm-cli/common"
	"github.com/cloudbase/garm/params"
)

var (
	proxyName        string
	proxyDescription string
	proxyHTTPProxy   string
	proxyHTTPSProxy  string
	proxyNoProxy     string
	proxyUsername    string
	proxyPassword    string
)

var proxiesCmd = &cobra.Command{
	Use:          "proxy",
	SilenceUsage: true,
	Short:        "Manage proxies",
	Long: `Manage proxy definitions.

Proxy definitions hold the proxy settings that runners will use to reach
back to GARM, the forge and any other resources they need during setup.
They are useful when runners are spun up in environments without direct
outbound connectivity, such as air-gapped clouds.

Pools and scale sets may reference a proxy definition, in which case the
proxy settings will be injected into the runners they create.
`,
	Run: nil,
}

var proxyCreateCmd = &cobra.Command{
	Use:          "create",
	Aliases:      []string{"add"},
	SilenceUsage: true,
	Short:        "Create a new proxy",
	Long:         `Create a new proxy definition.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		createProxyReq := apiClientProxies.NewCreateProxyParams()
		createProxyReq.Body = params.CreateProxyParams{
			Name:        proxyName,
			Description: proxyDescription,
			HTTPProxy:   proxyHTTPProxy,
			HTTPSProxy:  proxyHTTPSProxy,
			NoProxy:     proxyNoProxy,
			Username:    proxyUsername,
			Password:    proxyPassword,
		}

		response, err := apiCli.Proxies.CreateProxy(createProxyReq, authToken)
		if err != nil {
			return err
		}
		formatOneProxy(response.Payload)
		return nil
	},
}

var proxyListCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	SilenceUsage: true,
	Short:        "List proxies",
	Long:         `List all proxy definitions.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if needsInit {
			return errNeedsInitError
		}

		listProxiesReq := apiClientProxies.NewListProxiesParams()
		response, err := apiCli.Proxies.ListProxies(listProxiesReq, authToken)
		if err != nil {
			return err
		}
		formatProxyList(response.Payload)
		return nil
	},
}

var proxyShowCmd = &cobra.Command{
	Use:          "show [flags] proxy_name_or_id",
	SilenceUsage: true,
	Short:        "Show proxy",
	Long:         `Show details of one proxy definition.`,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a proxy name or ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		proxyID, err := resolveProxyAsUint(args[0])
		if err != nil {
			return err
		}

		getProxyReq := apiClientProxies.NewGetProxyParams()
		getProxyReq.ProxyID = float64(proxyID)
		response, err := apiCli.Proxies.GetProxy(getProxyReq, authToken)
		if err != nil {
			return err
		}
		formatOneProxy(response.Payload)
		return nil
	},
}

var proxyUpdateCmd = &cobra.Command{
	Use:          "update [flags] proxy_name_or_id",
	SilenceUsage: true,
	Short:        "Update proxy",
	Long: `Update a proxy definition.

Runners created after the update will use the new proxy settings. Existing
runners are not affected.

Setting --username to an empty string clears the proxy credentials.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a proxy name or ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		proxyID, err := resolveProxyAsUint(args[0])
		if err != nil {
			return err
		}

		updateParams := params.UpdateProxyParams{}
		if cmd.Flags().Changed("name") {
			updateParams.Name = &proxyName
		}
		if cmd.Flags().Changed("description") {
			updateParams.Description = &proxyDescription
		}
		if cmd.Flags().Changed("http-proxy") {
			updateParams.HTTPProxy = &proxyHTTPProxy
		}
		if cmd.Flags().Changed("https-proxy") {
			updateParams.HTTPSProxy = &proxyHTTPSProxy
		}
		if cmd.Flags().Changed("no-proxy") {
			updateParams.NoProxy = &proxyNoProxy
		}
		if cmd.Flags().Changed("username") {
			updateParams.Username = &proxyUsername
		}
		if cmd.Flags().Changed("password") {
			updateParams.Password = &proxyPassword
		}

		updateProxyReq := apiClientProxies.NewUpdateProxyParams()
		updateProxyReq.ProxyID = float64(proxyID)
		updateProxyReq.Body = updateParams

		response, err := apiCli.Proxies.UpdateProxy(updateProxyReq, authToken)
		if err != nil {
			return err
		}
		formatOneProxy(response.Payload)
		return nil
	},
}

var proxyDeleteCmd = &cobra.Command{
	Use:          "delete [flags] proxy_name_or_id",
	Aliases:      []string{"remove", "rm"},
	SilenceUsage: true,
	Short:        "Delete proxy",
	Long: `Delete a proxy definition.

Proxies that are still referenced by pools or scale sets cannot be deleted.
`,
	RunE: func(_ *cobra.Command, args []string) error {
		if needsInit {
			return errNeedsInitError
		}

		if len(args) == 0 {
			return fmt.Errorf("command requires a proxy name or ID")
		}

		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}

		proxyID, err := resolveProxyAsUint(args[0])
		if err != nil {
			return err
		}

		deleteProxyReq := apiClientProxies.NewDeleteProxyParams()
		deleteProxyReq.ProxyID = float64(proxyID)
		if err := apiCli.Proxies.DeleteProxy(deleteProxyReq, authToken); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	proxyCreateCmd.Flags().StringVar(&proxyName, "name", "", "Name of the proxy.")
	proxyCreateCmd.Flags().StringVar(&proxyDescription, "description", "", "Proxy description.")
	proxyCreateCmd.Flags().StringVar(&proxyHTTPProxy, "http-proxy", "", "Proxy URL for plain HTTP requests.")
	proxyCreateCmd.Flags().StringVar(&proxyHTTPSProxy, "https-proxy", "", "Proxy URL for HTTPS requests.")
	proxyCreateCmd.Flags().StringVar(&proxyNoProxy, "no-proxy", "", "Comma separated list of hosts, domains or CIDRs that bypass the proxy.")
	proxyCreateCmd.Flags().StringVar(&proxyUsername, "username", "", "Username used to authenticate to the proxy.")
	proxyCreateCmd.Flags().StringVar(&proxyPassword, "password", "", "Password used to authenticate to the proxy.")
	proxyCreateCmd.MarkFlagRequired("name") //nolint

	proxyUpdateCmd.Flags().StringVar(&proxyName, "name", "", "Name of the proxy.")
	proxyUpdateCmd.Flags().StringVar(&proxyDescription, "description", "", "Proxy description.")
	proxyUpdateCmd.Flags().StringVar(&proxyHTTPProxy, "http-proxy", "", "Proxy URL for plain HTTP requests.")
	proxyUpdateCmd.Flags().StringVar(&proxyHTTPSProxy, "https-proxy", "", "Proxy URL for HTTPS requests.")
	proxyUpdateCmd.Flags().StringVar(&proxyNoProxy, "no-proxy", "", "Comma separated list of hosts, domains or CIDRs that bypass the proxy.")
	proxyUpdateCmd.Flags().StringVar(&proxyUsername, "username", "", "Username used to authenticate to the proxy. Set to an empty string to clear the credentials.")
	proxyUpdateCmd.Flags().StringVar(&proxyPassword, "password", "", "Password used to authenticate to the proxy.")

	proxiesCmd.AddCommand(proxyCreateCmd)
	proxiesCmd.AddCommand(proxyListCmd)
	proxiesCmd.AddCommand(proxyShowCmd)
	proxiesCmd.AddCommand(proxyUpdateCmd)
	proxiesCmd.AddCommand(proxyDeleteCmd)
	rootCmd.AddCommand(proxiesCmd)
}

func formatOneProxy(proxy params.Proxy) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(proxy)
		return
	}
	t := table.NewWriter()
	header := table.Row{"Field", "Value"}
	t.AppendHeader(header)

	t.AppendRow(table.Row{"ID", proxy.ID})
	t.AppendRow(table.Row{"Created At", proxy.CreatedAt})
	t.AppendRow(table.Row{"Updated At", proxy.UpdatedAt})
	t.AppendRow(table.Row{"Name", proxy.Name})
	if proxy.Description != "" {
		t.AppendRow(table.Row{"Description", proxy.Description})
	}
	if proxy.HTTPProxy != "" {
		t.AppendRow(table.Row{"HTTP Proxy", proxy.HTTPProxy})
	}
	if proxy.HTTPSProxy != "" {
		t.AppendRow(table.Row{"HTTPS Proxy", proxy.HTTPSProxy})
	}
	if proxy.NoProxy != "" {
		t.AppendRow(table.Row{"No Proxy", proxy.NoProxy})
	}
	t.AppendRow(table.Row{"Auth Enabled", proxy.Username != ""})
	if proxy.Username != "" {
		t.AppendRow(table.Row{"Username", proxy.Username})
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
		{Number: 2, AutoMerge: false, WidthMax: 100},
	})
	fmt.Println(t.Render())
}

func formatProxyList(proxies params.Proxies) {
	if outputFormat == common.OutputFormatJSON {
		printAsJSON(proxies)
		return
	}
	t := table.NewWriter()
	header := table.Row{"ID", "Name", "Description", "HTTP Proxy", "HTTPS Proxy", "Auth Enabled"}
	t.AppendHeader(header)
	for _, val := range proxies {
		row := table.Row{val.ID, val.Name, val.Description, val.HTTPProxy, val.HTTPSProxy, val.Username != ""}
		t.AppendRow(row)
		t.AppendSeparator()
	}
	fmt.Println(t.Render())
}
