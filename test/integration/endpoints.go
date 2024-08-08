package integration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudbase/garm/params"
)

func checkEndpointParamsAreEqual(a, b params.GithubEndpoint) error {
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
