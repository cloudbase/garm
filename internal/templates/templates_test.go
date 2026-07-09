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

package templates

import (
	"context"
	"fmt"
	"strings"
	"testing"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

// TestInstallTemplatesRenderForceInsecure renders every default install
// template and checks that the agent config only carries force_insecure when
// the controller allows insecure agent connections. Agents older than v0.1.1
// do not know the setting, so it must be absent unless explicitly enabled.
// TestWrapperRendersProxySettings renders the install wrappers with and
// without a proxy config and checks that proxy environment variables only
// show up when a proxy is set.
func TestWrapperRendersProxySettings(t *testing.T) {
	ctx := context.Background()
	proxyCfg := commonParams.ProxyConfig{
		HTTPProxy:  "http://user:pass@proxy.example.com:3128",
		HTTPSProxy: "http://user:pass@proxy.example.com:3128",
		NoProxy:    "localhost,.internal.example.com,10.0.0.0/8",
	}

	for _, osType := range []commonParams.OSType{commonParams.Linux, commonParams.Windows} {
		t.Run(string(osType), func(t *testing.T) {
			rendered, err := RenderRunnerInstallWrapper(ctx, osType, "https://garm.example.com/api/v1/metadata", "https://garm.example.com/api/v1/callbacks", "token", commonParams.ProxyConfig{})
			if err != nil {
				t.Fatalf("failed to render wrapper: %v", err)
			}
			for _, needle := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "DefaultWebProxy"} {
				if strings.Contains(string(rendered), needle) {
					t.Errorf("expected %q to be absent when no proxy is set", needle)
				}
			}

			rendered, err = RenderRunnerInstallWrapper(ctx, osType, "https://garm.example.com/api/v1/metadata", "https://garm.example.com/api/v1/callbacks", "token", proxyCfg)
			if err != nil {
				t.Fatalf("failed to render wrapper: %v", err)
			}
			for _, needle := range []string{proxyCfg.HTTPProxy, proxyCfg.NoProxy} {
				if !strings.Contains(string(rendered), needle) {
					t.Errorf("expected %q in the rendered wrapper", needle)
				}
			}
			if osType == commonParams.Windows {
				for _, needle := range []string{"DefaultWebProxy", `[Environment]::SetEnvironmentVariable("HTTPS_PROXY", $env:HTTPS_PROXY, "Machine")`} {
					if !strings.Contains(string(rendered), needle) {
						t.Errorf("expected %q in the rendered windows wrapper", needle)
					}
				}
			} else if !strings.Contains(string(rendered), `export https_proxy='`+proxyCfg.HTTPSProxy+`'`) {
				t.Error("expected lowercase https_proxy export in the linux wrapper")
			}
		})
	}
}

// TestWindowsInstallTemplatesHaveProxyPreamble checks that the windows
// install scripts configure the .NET default web proxy from the proxy
// environment variables inherited from the wrapper. PowerShell 5.1 does not
// honor proxy environment variables on its own.
func TestWindowsInstallTemplatesHaveProxyPreamble(t *testing.T) {
	for _, forge := range []params.EndpointType{params.GithubEndpointType, params.GiteaEndpointType} {
		content, err := GetTemplateContent(commonParams.Windows, forge)
		if err != nil {
			t.Fatalf("failed to get template content: %v", err)
		}
		if !strings.Contains(string(content), "[System.Net.WebRequest]::DefaultWebProxy = $defaultProxy") {
			t.Errorf("expected windows %s install template to set the default web proxy", forge)
		}
	}
}

// TestInstallTemplatesRenderAgentProxy renders every default install
// template and checks that the garm-agent proxy section is only emitted when
// a proxy is set. Agents older than v0.1.1 do not know the proxy section, so
// it must be absent otherwise.
func TestInstallTemplatesRenderAgentProxy(t *testing.T) {
	for _, forge := range []params.EndpointType{params.GithubEndpointType, params.GiteaEndpointType} {
		for _, osType := range []commonParams.OSType{commonParams.Linux, commonParams.Windows} {
			t.Run(fmt.Sprintf("%s_%s", forge, osType), func(t *testing.T) {
				content, err := GetTemplateContent(osType, forge)
				if err != nil {
					t.Fatalf("failed to get template content: %v", err)
				}

				tplCtx := InstallContext{
					AgentMode:  true,
					AgentURL:   "wss://garm.example.com/agent",
					AgentToken: "secret",
					AgentShell: "false",
				}

				rendered, err := RenderRunnerInstallScript(string(content), tplCtx)
				if err != nil {
					t.Fatalf("failed to render template: %v", err)
				}
				if strings.Contains(string(rendered), "[proxy]") {
					t.Error("expected proxy section to be absent when no proxy is set")
				}

				tplCtx.HTTPSProxy = "http://user:pass@proxy.example.com:3128"
				tplCtx.NoProxy = "localhost,.internal.example.com"
				rendered, err = RenderRunnerInstallScript(string(content), tplCtx)
				if err != nil {
					t.Fatalf("failed to render template: %v", err)
				}
				needles := []string{
					"[proxy]",
					`https_proxy = "http://user:pass@proxy.example.com:3128"`,
					`no_proxy = "localhost,.internal.example.com"`,
				}
				if forge == params.GithubEndpointType {
					// The github runner reads proxy settings from the
					// .env file in the runner install dir.
					needles = append(needles, "https_proxy=http://user:pass@proxy.example.com:3128")
				}
				for _, needle := range needles {
					if !strings.Contains(string(rendered), needle) {
						t.Errorf("expected %q in the rendered template", needle)
					}
				}
			})
		}
	}
}

func TestInstallTemplatesRenderForceInsecure(t *testing.T) {
	for _, forge := range []params.EndpointType{params.GithubEndpointType, params.GiteaEndpointType} {
		for _, osType := range []commonParams.OSType{commonParams.Linux, commonParams.Windows} {
			t.Run(fmt.Sprintf("%s_%s", forge, osType), func(t *testing.T) {
				content, err := GetTemplateContent(osType, forge)
				if err != nil {
					t.Fatalf("failed to get template content: %v", err)
				}

				tplCtx := InstallContext{
					AgentMode:  true,
					AgentURL:   "wss://garm.example.com/agent",
					AgentToken: "secret",
					AgentShell: "false",
				}

				rendered, err := RenderRunnerInstallScript(string(content), tplCtx)
				if err != nil {
					t.Fatalf("failed to render template: %v", err)
				}
				if strings.Contains(string(rendered), "force_insecure") {
					t.Error("expected force_insecure to be absent when not allowed")
				}

				tplCtx.ForceInsecureGARMAgent = true
				rendered, err = RenderRunnerInstallScript(string(content), tplCtx)
				if err != nil {
					t.Fatalf("failed to render template: %v", err)
				}
				if !strings.Contains(string(rendered), "force_insecure = true") {
					t.Error("expected force_insecure = true in the agent config")
				}
				// The neighboring config keys must survive the conditional.
				if !strings.Contains(string(rendered), "enable_shell = ") {
					t.Error("expected enable_shell to remain in the agent config")
				}
			})
		}
	}
}
