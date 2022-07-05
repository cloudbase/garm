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

package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	EncryptionPassphrase = "bocyasicgatEtenOubwonIbsudNutDom"
)

func getDefaultSectionConfig(configDir string) Default {
	return Default{
		ConfigDir:   configDir,
		CallbackURL: "https://garm.example.com/",
		LogFile:     filepath.Join(configDir, "garm.log"),
	}
}

func getDefaultTLSConfig() TLSConfig {
	return TLSConfig{
		CRT:    "../testdata/certs/srv-pub.pem",
		Key:    "../testdata/certs/srv-key.pem",
		CACert: "../testdata/certs/ca-pub.pem",
	}
}

func getDefaultAPIServerConfig() APIServer {
	return APIServer{
		Bind:        "0.0.0.0",
		Port:        9998,
		UseTLS:      true,
		TLSConfig:   getDefaultTLSConfig(),
		CORSOrigins: []string{},
	}
}

func getDefaultDatabaseConfig(dir string) Database {
	return Database{
		Debug:     false,
		DbBackend: SQLiteBackend,
		SQLite: SQLite{
			DBFile: filepath.Join(dir, "garm.db"),
		},
		Passphrase: EncryptionPassphrase,
	}
}

func getDefaultProvidersConfig() []Provider {
	lxdConfig := getDefaultLXDConfig()
	return []Provider{
		{
			Name:         "test_lxd",
			ProviderType: LXDProvider,
			Description:  "test LXD provider",
			LXD:          lxdConfig,
		},
	}
}

func getDefaultGithubConfig() []Github {
	return []Github{
		{
			Name:        "dummy_creds",
			Description: "dummy github credentials",
			OAuth2Token: "bogus",
		},
	}
}

func getDefaultJWTCofig() JWTAuth {
	return JWTAuth{
		Secret:     EncryptionPassphrase,
		TimeToLive: "48h",
	}
}

func getDefaultConfig(t *testing.T) Config {
	dir, err := ioutil.TempDir("", "garm-config-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	return Config{
		Default:   getDefaultSectionConfig(dir),
		APIServer: getDefaultAPIServerConfig(),
		Database:  getDefaultDatabaseConfig(dir),
		Providers: getDefaultProvidersConfig(),
		Github:    getDefaultGithubConfig(),
		JWTAuth:   getDefaultJWTCofig(),
	}
}

func TestConfig(t *testing.T) {
	cfg := getDefaultConfig(t)

	err := cfg.Validate()
	assert.Nil(t, err)
}

func TestDefaultSectionConfig(t *testing.T) {
	dir, err := ioutil.TempDir("", "garm-config-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	cfg := getDefaultSectionConfig(dir)

	tests := []struct {
		name      string
		cfg       Default
		errString string
	}{
		{
			name:      "Config is valid",
			cfg:       cfg,
			errString: "",
		},
		{
			name: "CallbackURL cannot be empty",
			cfg: Default{
				CallbackURL: "",
				ConfigDir:   cfg.ConfigDir,
			},
			errString: "missing callback_url",
		},
		{
			name: "ConfigDir cannot be empty",
			cfg: Default{
				CallbackURL: cfg.CallbackURL,
				ConfigDir:   "",
			},
			errString: "config_dir cannot be empty",
		},
		{
			name: "config_dir must exist and be accessible",
			cfg: Default{
				CallbackURL: cfg.CallbackURL,
				ConfigDir:   "/i/do/not/exist",
			},
			errString: "accessing config dir: stat /i/do/not/exist:.*",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Regexp(t, tc.errString, err.Error())
			}
		})
	}
}
