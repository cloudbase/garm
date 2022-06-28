package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getDefaultExternalConfig(t *testing.T) External {
	dir, err := ioutil.TempDir("", "garm-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	err = ioutil.WriteFile(filepath.Join(dir, "garm-external-provider"), []byte{}, 0755)
	if err != nil {
		t.Fatalf("failed to write file: %s", err)
	}

	return External{
		ConfigFile:  "",
		ProviderDir: dir,
	}
}

func TestExternal(t *testing.T) {
	cfg := getDefaultExternalConfig(t)

	tests := []struct {
		name      string
		cfg       External
		errString string
	}{
		{
			name:      "Config is valid",
			cfg:       cfg,
			errString: "",
		},
		{
			name: "Config path cannot be relative path",
			cfg: External{
				ConfigFile:  "../test",
				ProviderDir: cfg.ProviderDir,
			},
			errString: "path to config file must be an absolute path",
		},
		{
			name: "Config must exist if specified",
			cfg: External{
				ConfigFile:  "/there/is/no/config/here",
				ProviderDir: cfg.ProviderDir,
			},
			errString: "failed to access config file /there/is/no/config/here",
		},
		{
			name: "Missing provider dir",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "",
			},
			errString: "missing provider dir",
		},
		{
			name: "Provider dir must not be relative",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "../test",
			},
			errString: "path to provider dir must be absolute",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.EqualError(t, err, tc.errString)
			}
		})
	}
}
