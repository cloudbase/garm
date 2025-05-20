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

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudbase/garm/params"
)

func checkEndpointParamsAreEqual(a, b params.ForgeEndpoint) error {
	if a.Name != b.Name {
		return fmt.Errorf("endpoint name mismatch")
	}

	if a.Description != b.Description {
		return fmt.Errorf("endpoint description mismatch")
	}

	if a.BaseURL != b.BaseURL {
		return fmt.Errorf("endpoint base URL mismatch")
	}

	if a.APIBaseURL != b.APIBaseURL {
		return fmt.Errorf("endpoint API base URL mismatch")
	}

	if a.UploadBaseURL != b.UploadBaseURL {
		return fmt.Errorf("endpoint upload base URL mismatch")
	}

	if string(a.CACertBundle) != string(b.CACertBundle) {
		return fmt.Errorf("endpoint CA cert bundle mismatch")
	}
	return nil
}

func getTestFileContents(relPath string) ([]byte, error) {
	baseDir := os.Getenv("GARM_CHECKOUT_DIR")
	if baseDir == "" {
		return nil, fmt.Errorf("ariable GARM_CHECKOUT_DIR not set")
	}
	contents, err := os.ReadFile(filepath.Join(baseDir, "testdata", relPath))
	if err != nil {
		return nil, err
	}
	return contents, nil
}
