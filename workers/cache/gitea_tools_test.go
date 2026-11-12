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

package cache

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudbase/garm/params"
)

// giteaRunnerAssetNames is the full asset list of gitea/runner v5.4.1, which
// is what the default tools metadata URL serves. Keeping the real names here
// pins the asset name parsing against what Gitea actually publishes.
var giteaRunnerAssetNames = []string{
	"checksums.txt",
	"checksums.txt.sig",
	"gitea-runner-5.4.1-darwin-amd64",
	"gitea-runner-5.4.1-darwin-amd64.xz",
	"gitea-runner-5.4.1-darwin-amd64.xz.sha256",
	"gitea-runner-5.4.1-darwin-arm64",
	"gitea-runner-5.4.1-darwin-arm64.xz",
	"gitea-runner-5.4.1-darwin-arm64.xz.sha256",
	"gitea-runner-5.4.1-freebsd-amd64",
	"gitea-runner-5.4.1-freebsd-amd64.xz",
	"gitea-runner-5.4.1-freebsd-amd64.xz.sha256",
	"gitea-runner-5.4.1-linux-amd64",
	"gitea-runner-5.4.1-linux-amd64.xz",
	"gitea-runner-5.4.1-linux-amd64.xz.sha256",
	"gitea-runner-5.4.1-linux-arm-5",
	"gitea-runner-5.4.1-linux-arm-5.xz",
	"gitea-runner-5.4.1-linux-arm-5.xz.sha256",
	"gitea-runner-5.4.1-linux-arm-6",
	"gitea-runner-5.4.1-linux-arm-6.xz",
	"gitea-runner-5.4.1-linux-arm-6.xz.sha256",
	"gitea-runner-5.4.1-linux-arm-7",
	"gitea-runner-5.4.1-linux-arm-7.xz",
	"gitea-runner-5.4.1-linux-arm-7.xz.sha256",
	"gitea-runner-5.4.1-linux-arm64",
	"gitea-runner-5.4.1-linux-arm64.xz",
	"gitea-runner-5.4.1-linux-arm64.xz.sha256",
	"gitea-runner-5.4.1-linux-loong64",
	"gitea-runner-5.4.1-linux-loong64.xz",
	"gitea-runner-5.4.1-linux-loong64.xz.sha256",
	"gitea-runner-5.4.1-linux-riscv64",
	"gitea-runner-5.4.1-linux-riscv64.xz",
	"gitea-runner-5.4.1-linux-riscv64.xz.sha256",
	"gitea-runner-5.4.1-linux-s390x",
	"gitea-runner-5.4.1-linux-s390x.xz",
	"gitea-runner-5.4.1-linux-s390x.xz.sha256",
	"gitea-runner-5.4.1-windows-amd64.exe",
	"gitea-runner-5.4.1-windows-amd64.exe.xz",
	"gitea-runner-5.4.1-windows-amd64.exe.xz.sha256",
	"gitea-runner-5.4.1-windows-arm64.exe",
	"gitea-runner-5.4.1-windows-arm64.exe.xz",
	"gitea-runner-5.4.1-windows-arm64.exe.xz.sha256",
}

// giteaReleaseListJSON renders the asset names as the release list the Gitea
// API returns.
func giteaReleaseListJSON(t *testing.T, tagName string, assetNames []string) string {
	t.Helper()
	assets := make([]GiteaToolsAssets, 0, len(assetNames))
	for _, name := range assetNames {
		assets = append(assets, GiteaToolsAssets{
			Name:        name,
			DownloadURL: "https://dl.gitea.com/runner/" + tagName + "/" + name,
		})
	}
	data, err := json.Marshal(GiteaEntityTools{{TagName: tagName, Name: tagName, Assets: assets}})
	if err != nil {
		t.Fatalf("marshaling release list: %v", err)
	}
	return string(data)
}

func TestGetToolsRealGiteaAssets(t *testing.T) {
	body := giteaReleaseListJSON(t, "v5.4.1", giteaRunnerAssetNames)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	tools, err := getTools(context.Background(), server.URL, false, params.ForgeCredentials{})
	if err != nil {
		t.Fatalf("getTools: %v", err)
	}

	resolved := map[string]string{}
	for _, tool := range tools {
		key := tool.Platform()
		if existing, ok := resolved[key]; ok {
			t.Errorf("%s resolved twice: %s and %s", key, existing, tool.GetFilename())
		}
		resolved[key] = tool.GetFilename()
	}

	// Every OS/arch combo GARM can run a Gitea runner on, and the asset it
	// has to come from. The compressed and checksum assets must never win.
	//
	// Note the OS is "windows", not the "win" the GitHub API reports. Gitea
	// asset names carry the Go OS name and it is passed through as is;
	// commonUtil.GetTools accepts either spelling.
	for key, want := range map[string]string{
		"linux/x64":     "gitea-runner-5.4.1-linux-amd64",
		"linux/arm64":   "gitea-runner-5.4.1-linux-arm64",
		"linux/arm":     "gitea-runner-5.4.1-linux-arm-7",
		"linux/riscv64": "gitea-runner-5.4.1-linux-riscv64",
		"linux/s390x":   "gitea-runner-5.4.1-linux-s390x",
		"windows/x64":   "gitea-runner-5.4.1-windows-amd64.exe",
		"windows/arm64": "gitea-runner-5.4.1-windows-arm64.exe",
	} {
		got, ok := resolved[key]
		if !ok {
			t.Errorf("missing tools for %s", key)
			continue
		}
		if got != want {
			t.Errorf("%s: expected %s, got %s", key, want, got)
		}
	}
}

// Gitea names its ARM builds GOARCH-GOARM. Only the ARMv7 build matches what
// GitHub calls "arm", and the older variants must not take its place.
func TestGiteaAssetARMVariants(t *testing.T) {
	cases := map[string]string{
		"gitea-runner-5.4.1-linux-arm-7":       "arm",
		"gitea-runner-5.4.1-linux-arm-5":       "",
		"gitea-runner-5.4.1-linux-arm-6":       "",
		"gitea-runner-5.4.1-linux-arm64":       "arm64",
		"gitea-runner-5.4.1-linux-amd64":       "x64",
		"gitea-runner-5.4.1-linux-s390x":       "s390x",
		"gitea-runner-5.4.1-windows-amd64.exe": "x64",
	}

	for name, want := range cases {
		arch, err := GiteaToolsAssets{Name: name}.GetArch()
		if want == "" {
			if err == nil {
				t.Errorf("%s: expected to be skipped, resolved to %s", name, *arch)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if *arch != want {
			t.Errorf("%s: expected %s, got %s", name, want, *arch)
		}
	}
}

// Assets built for an architecture GARM does not know are skipped rather than
// resolved under some other name. Gitea publishes a linux/loong64 build that
// GARM has no architecture for yet, alongside genuinely unknown ones.
func TestGiteaAssetUnknownArchSkipped(t *testing.T) {
	for _, name := range []string{
		"gitea-runner-5.4.1-linux-loong64",
		"gitea-runner-5.4.1-linux-m68k",
	} {
		if arch, err := (GiteaToolsAssets{Name: name}).GetArch(); err == nil {
			t.Errorf("%s: expected to be skipped, resolved to %s", name, *arch)
		}
	}
}

// A Gitea tools metadata URL may sit behind the private CA configured on the
// endpoint, the same as a GitHub one.
func TestGetToolsGiteaPrivateCA(t *testing.T) {
	body := giteaReleaseListJSON(t, "v5.4.1", giteaRunnerAssetNames)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer server.Close()

	if _, err := getTools(context.Background(), server.URL, false, params.ForgeCredentials{}); err == nil {
		t.Fatal("expected an untrusted certificate to fail")
	}

	caBundle := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: server.Certificate().Raw,
	})

	tools, err := getTools(context.Background(), server.URL, false, params.ForgeCredentials{CABundle: caBundle})
	if err != nil {
		t.Fatalf("expected the endpoint CA to be trusted: %v", err)
	}
	if len(tools) == 0 {
		t.Error("expected tools to be resolved")
	}
}
