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

package util

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"github.com/h2non/filetype"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/internal/templates"
	"github.com/cloudbase/garm/runner/common"
)

func FetchTools(ctx context.Context, cli common.GithubClient) ([]commonParams.RunnerApplicationDownload, error) {
	tools, ghResp, err := cli.ListEntityRunnerApplicationDownloads(ctx)
	if err != nil {
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("error fetching tools: %w", runnerErrors.ErrUnauthorized)
		}
		return nil, fmt.Errorf("error fetching runner tools: %w", err)
	}

	ret := []commonParams.RunnerApplicationDownload{}
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		ret = append(ret, commonParams.RunnerApplicationDownload(*tool))
	}
	return ret, nil
}

func ASCIIEqualFold(s, t string) bool {
	// Fast ASCII path for equal-length ASCII strings
	if len(s) == len(t) && isASCII(s) && isASCII(t) {
		for i := 0; i < len(s); i++ {
			a, b := s[i], t[i]
			if a != b {
				if 'A' <= a && a <= 'Z' {
					a = a + 'a' - 'A'
				}
				if 'A' <= b && b <= 'Z' {
					b = b + 'a' - 'A'
				}
				if a != b {
					return false
				}
			}
		}
		return true
	}

	// UTF-8 path - handle different byte lengths correctly
	i, j := 0, 0
	for i < len(s) && j < len(t) {
		sr, sizeS := utf8.DecodeRuneInString(s[i:])
		tr, sizeT := utf8.DecodeRuneInString(t[j:])

		// Handle invalid UTF-8 - they must be identical
		if sr == utf8.RuneError || tr == utf8.RuneError {
			// For invalid UTF-8, compare the raw bytes
			if sr == utf8.RuneError && tr == utf8.RuneError {
				if sizeS == sizeT && s[i:i+sizeS] == t[j:j+sizeT] {
					i += sizeS
					j += sizeT
					continue
				}
			}
			return false
		}

		if sr != tr {
			// Apply ASCII case folding only
			if 'A' <= sr && sr <= 'Z' {
				sr = sr + 'a' - 'A'
			}
			if 'A' <= tr && tr <= 'Z' {
				tr = tr + 'a' - 'A'
			}
			if sr != tr {
				return false
			}
		}

		i += sizeS
		j += sizeT
	}
	return i == len(s) && j == len(t)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func GetCloudConfigSpecFromExtraSpecs(extraSpecs json.RawMessage) (cloudconfig.CloudConfigSpec, error) {
	boot := commonParams.BootstrapInstance{
		ExtraSpecs: extraSpecs,
	}

	specs, err := cloudconfig.GetSpecs(boot)
	if err != nil {
		return cloudconfig.CloudConfigSpec{}, fmt.Errorf("failed to decode extra specs: %w", err)
	}

	return specs, nil
}

func MaybeAddWrapperToExtraSpecs(ctx context.Context, specs json.RawMessage, osType commonParams.OSType, metadataURL, token string) json.RawMessage {
	data := map[string]any{}
	if len(specs) > 0 {
		if err := json.Unmarshal(specs, &data); err != nil {
			slog.WarnContext(ctx, "failed to unmarshal extra specs", "error", err)
			return specs
		}
	}

	if _, ok := data["runner_install_template"]; ok {
		// User has already set a runner install template override. Do not touch.
		return specs
	}

	wrapper, err := templates.RenderRunnerInstallWrapper(osType, metadataURL, token)
	if err != nil {
		slog.WarnContext(ctx, "failed to get runner install wrapper", "os_type", osType, "error", err)
		return specs
	}

	data["runner_install_template"] = wrapper
	ret, err := json.Marshal(data)
	if err != nil {
		slog.WarnContext(ctx, "failed to marshal extra specs", "error", err)
		return specs
	}

	return json.RawMessage(ret)
}

// DetectFileType detects the MIME type from file content
func DetectFileType(data []byte) string {
	// First, try http.DetectContentType (good for text files)
	httpType := http.DetectContentType(data)

	// If http detected text, use that
	if httpType != "application/octet-stream" {
		return httpType
	}

	// For binary files, use filetype library for specific format detection
	kind, err := filetype.Match(data)
	if err == nil && kind != filetype.Unknown {
		return kind.MIME.Value
	}

	// Default to application/octet-stream for unknown types
	return "application/octet-stream"
}
