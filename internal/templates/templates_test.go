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
