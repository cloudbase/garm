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

package util

import (
	"testing"

	commonParams "github.com/cloudbase/garm-provider-common/params"
)

func tool(osType, arch, downloadURL string) commonParams.RunnerApplicationDownload {
	return commonParams.RunnerApplicationDownload{
		OS:           &osType,
		Architecture: &arch,
		DownloadURL:  &downloadURL,
	}
}

func toolURLs(tools []commonParams.RunnerApplicationDownload) map[string]string {
	ret := make(map[string]string, len(tools))
	for _, t := range tools {
		ret[t.Platform()] = t.GetDownloadURL()
	}
	return ret
}

func TestMergeTools(t *testing.T) {
	forge := []commonParams.RunnerApplicationDownload{
		tool("linux", "x64", "forge/linux-x64"),
		tool("linux", "arm64", "forge/linux-arm64"),
		tool("win", "x64", "forge/win-x64"),
	}
	override := []commonParams.RunnerApplicationDownload{
		// Replaces a combo the forge already has.
		tool("linux", "x64", "override/linux-x64"),
		// Combos only the override knows about.
		tool("linux", "ppc64le", "override/linux-ppc64le"),
		tool("linux", "s390x", "override/linux-s390x"),
	}

	merged := MergeTools(forge, override)
	if len(merged) != 5 {
		t.Fatalf("expected 5 tools, got %d: %v", len(merged), toolURLs(merged))
	}

	urls := toolURLs(merged)
	for key, want := range map[string]string{
		"linux/x64":     "override/linux-x64",
		"linux/arm64":   "forge/linux-arm64",
		"win/x64":       "forge/win-x64",
		"linux/ppc64le": "override/linux-ppc64le",
		"linux/s390x":   "override/linux-s390x",
	} {
		if got := urls[key]; got != want {
			t.Errorf("%s: expected %s, got %s", key, want, got)
		}
	}

	// The override is taken first, so its entries lead the list. Nothing
	// reads the list by position, but a fixed order keeps the merge
	// reproducible across refreshes.
	for i, want := range []string{
		"override/linux-x64",
		"override/linux-ppc64le",
		"override/linux-s390x",
	} {
		if got := merged[i].GetDownloadURL(); got != want {
			t.Errorf("merged[%d]: expected %s, got %s", i, want, got)
		}
	}
}

func TestMergeToolsEmptyInputs(t *testing.T) {
	forge := []commonParams.RunnerApplicationDownload{tool("linux", "x64", "forge/linux-x64")}
	override := []commonParams.RunnerApplicationDownload{tool("linux", "s390x", "override/linux-s390x")}

	if got := MergeTools(nil, nil); len(got) != 0 {
		t.Errorf("expected no tools, got %d", len(got))
	}
	if got := toolURLs(MergeTools(forge, nil)); got["linux/x64"] != "forge/linux-x64" {
		t.Errorf("expected the forge tools unchanged, got %v", got)
	}
	if got := toolURLs(MergeTools(nil, override)); got["linux/s390x"] != "override/linux-s390x" {
		t.Errorf("expected the override tools, got %v", got)
	}
}

func TestMergeToolsDeduplicates(t *testing.T) {
	forge := []commonParams.RunnerApplicationDownload{
		tool("linux", "x64", "forge/first"),
		tool("linux", "x64", "forge/second"),
	}
	override := []commonParams.RunnerApplicationDownload{
		tool("linux", "s390x", "override/first"),
		tool("linux", "s390x", "override/second"),
	}

	merged := MergeTools(forge, override)
	if len(merged) != 2 {
		t.Fatalf("expected 2 tools, got %d: %v", len(merged), toolURLs(merged))
	}
	urls := toolURLs(merged)
	if urls["linux/x64"] != "forge/first" {
		t.Errorf("expected the first forge entry to win, got %s", urls["linux/x64"])
	}
	if urls["linux/s390x"] != "override/first" {
		t.Errorf("expected the first override entry to win, got %s", urls["linux/s390x"])
	}
}
