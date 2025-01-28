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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/cloudbase/garm/util/appdefaults"
)

var (
	// nolint: golangci-lint,gosec
	EncryptionPassphrase     = "bocyasicgatEtenOubwonIbsudNutDom"
	WeakEncryptionPassphrase = "1234567890abcdefghijklmnopqrstuv"
)

func getDefaultSectionConfig(configDir string) Default {
	return Default{
		CallbackURL: "https://garm.example.com/",
		MetadataURL: "https://garm.example.com/api/v1/metadata",
		LogFile:     filepath.Join(configDir, "garm.log"),
	}
}

func getDefaultTLSConfig() TLSConfig {
	return TLSConfig{
		CRT: "../testdata/certs/srv-pub.pem",
		Key: "../testdata/certs/srv-key.pem",
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

func getMySQLDefaultConfig() MySQL {
	return MySQL{
		Username:     "test",
		Password:     "test",
		Hostname:     "127.0.0.1",
		DatabaseName: "garm",
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
	return []Provider{}
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
	dir, err := os.MkdirTemp("", "garm-config-test")
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
	require.Nil(t, err)
}

func TestDefaultSectionConfig(t *testing.T) {
	dir, err := os.MkdirTemp("", "garm-config-test")
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
			name: "CallbackURL must be valid if set",
			cfg: Default{
				CallbackURL: "bogus_url",
				MetadataURL: cfg.MetadataURL,
			},
			errString: "invalid callback_url",
		},
		{
			name: "MetadataURL must be valid if set",
			cfg: Default{
				CallbackURL: cfg.CallbackURL,
				MetadataURL: "bogus_url",
			},
			errString: "invalid metadata_url",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Regexp(t, tc.errString, err.Error())
			}
		})
	}
}

func TestValidateAPIServerConfig(t *testing.T) {
	cfg := getDefaultAPIServerConfig()

	tests := []struct {
		name      string
		cfg       APIServer
		errString string
	}{
		{
			name:      "Config is valid",
			cfg:       cfg,
			errString: "",
		},
		{
			name: "Bind address is empty",
			cfg: APIServer{
				Bind: "",
				Port: 9998,
			},
			errString: "invalid IP address",
		},
		{
			name: "Bind address is invalid",
			cfg: APIServer{
				Bind: "not an IP",
				Port: 9998,
			},
			errString: "invalid IP address",
		},
		{
			name: "Bind address is valid IPv6",
			cfg: APIServer{
				Bind: "::",
				Port: 9998,
			},
			errString: "",
		},
		{
			name: "Port is not set",
			cfg: APIServer{
				Bind: cfg.Bind,
				Port: 0,
			},
			errString: "invalid port nr 0",
		},
		{
			name: "Port is not valid",
			cfg: APIServer{
				Bind: cfg.Bind,
				Port: 65536,
			},
			errString: "invalid port nr 65536",
		},
		{
			name: "Invalid TLS config",
			cfg: APIServer{
				Bind:      cfg.Bind,
				Port:      cfg.Port,
				TLSConfig: TLSConfig{},
				UseTLS:    true,
			},
			errString: "invalid tls config: missing crt or key",
		},
		{
			name: "Skip TLS config validation if UseTLS is false",
			cfg: APIServer{
				Bind:      cfg.Bind,
				Port:      cfg.Port,
				TLSConfig: TLSConfig{},
				UseTLS:    false,
			},
			errString: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Regexp(t, tc.errString, err.Error())
			}
		})
	}
}

func TestAPIBindAddress(t *testing.T) {
	cfg := getDefaultAPIServerConfig()

	err := cfg.Validate()
	require.Nil(t, err)
	require.Equal(t, cfg.BindAddress(), "0.0.0.0:9998")
}

func TestDatabaseConfig(t *testing.T) {
	dir, err := os.MkdirTemp("", "garm-config-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	cfg := getDefaultDatabaseConfig(dir)

	tests := []struct {
		name      string
		cfg       Database
		errString string
	}{
		{
			name:      "Config is valid",
			cfg:       cfg,
			errString: "",
		},
		{
			name: "Missing backend",
			cfg: Database{
				DbBackend:  "",
				SQLite:     cfg.SQLite,
				Passphrase: cfg.Passphrase,
			},
			errString: "invalid databse configuration: backend is required",
		},
		{
			name: "Invalid backend type",
			cfg: Database{
				DbBackend:  DBBackendType("bogus"),
				SQLite:     cfg.SQLite,
				Passphrase: cfg.Passphrase,
			},
			errString: "invalid database backend: bogus",
		},
		{
			name: "Missing passphrase",
			cfg: Database{
				DbBackend:  cfg.DbBackend,
				SQLite:     cfg.SQLite,
				Passphrase: "",
			},
			errString: "passphrase must be set and it must be a string of 32 characters*",
		},
		{
			name: "passphrase has invalid length",
			cfg: Database{
				DbBackend:  cfg.DbBackend,
				SQLite:     cfg.SQLite,
				Passphrase: "testing",
			},
			errString: "passphrase must be set and it must be a string of 32 characters*",
		},
		{
			name: "passphrase is too weak",
			cfg: Database{
				DbBackend:  cfg.DbBackend,
				SQLite:     cfg.SQLite,
				Passphrase: WeakEncryptionPassphrase,
			},
			errString: "database passphrase is too weak",
		},
		{
			name: "sqlite3 backend is missconfigured",
			cfg: Database{
				DbBackend: cfg.DbBackend,
				SQLite: SQLite{
					DBFile: "",
				},
				Passphrase: cfg.Passphrase,
			},
			errString: "validating sqlite3 config: no valid db_file was specified",
		},
		{
			name: "mysql backend is missconfigured",
			cfg: Database{
				DbBackend:  MySQLBackend,
				MySQL:      MySQL{},
				Passphrase: cfg.Passphrase,
			},
			errString: "validating mysql config: database, username, password, hostname are mandatory parameters for the database section",
		},
		{
			name: "mysql backend is configured and valid",
			cfg: Database{
				DbBackend:  MySQLBackend,
				MySQL:      getMySQLDefaultConfig(),
				Passphrase: cfg.Passphrase,
			},
			errString: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Regexp(t, tc.errString, err.Error())
			}
		})
	}
}

func TestGormParams(t *testing.T) {
	dir, err := os.MkdirTemp("", "garm-config-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	cfg := getDefaultDatabaseConfig(dir)

	dbType, uri, err := cfg.GormParams()
	require.Nil(t, err)
	require.Equal(t, SQLiteBackend, dbType)
	require.Equal(t, filepath.Join(dir, "garm.db?_journal_mode=WAL&_foreign_keys=ON"), uri)

	cfg.SQLite.BusyTimeoutSeconds = 5
	dbType, uri, err = cfg.GormParams()
	require.Nil(t, err)
	require.Equal(t, SQLiteBackend, dbType)
	require.Equal(t, filepath.Join(dir, "garm.db?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000"), uri)

	cfg.DbBackend = MySQLBackend
	cfg.MySQL = getMySQLDefaultConfig()
	cfg.SQLite = SQLite{}

	dbType, uri, err = cfg.GormParams()
	require.Nil(t, err)
	require.Equal(t, MySQLBackend, dbType)
	require.Equal(t, "test:test@tcp(127.0.0.1)/garm?charset=utf8&parseTime=True&loc=Local&timeout=5s", uri)
}

func TestSQLiteConfig(t *testing.T) {
	dir, err := os.MkdirTemp("", "garm-config-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	tests := []struct {
		name      string
		cfg       SQLite
		errString string
	}{
		{
			name: "Config is valid",
			cfg: SQLite{
				DBFile: filepath.Join(dir, "garm.db"),
			},
			errString: "",
		},
		{
			name: "db_file is empty",
			cfg: SQLite{
				DBFile: "",
			},
			errString: "no valid db_file was specified",
		},
		{
			name: "db_file must not be a relative path",
			cfg: SQLite{
				DBFile: "../test.db",
			},
			errString: "please specify an absolute path for db_file",
		},
		{
			name: "parent folder must exist",
			cfg: SQLite{
				DBFile: "/i/dont/exist/test.db",
			},
			errString: "parent directory of db_file does not exist: stat.*",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Regexp(t, tc.errString, err.Error())
			}
		})
	}
}

func TestJWTAuthConfig(t *testing.T) {
	cfg := JWTAuth{
		Secret:     EncryptionPassphrase,
		TimeToLive: "48h",
	}

	tests := []struct {
		name      string
		cfg       JWTAuth
		errString string
	}{
		{
			name:      "Config is valid",
			cfg:       cfg,
			errString: "",
		},
		{
			name: "secret is empty",
			cfg: JWTAuth{
				Secret:     "",
				TimeToLive: cfg.TimeToLive,
			},
			errString: "invalid JWT secret",
		},
		{
			name: "secret is weak",
			cfg: JWTAuth{
				Secret:     WeakEncryptionPassphrase,
				TimeToLive: cfg.TimeToLive,
			},
			errString: "jwt_secret is too weak",
		},
		{
			name: "time to live is invalid",
			cfg: JWTAuth{
				Secret:     cfg.Secret,
				TimeToLive: "bogus",
			},
			errString: "invalid time_to_live: time: invalid duration*",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Regexp(t, tc.errString, err.Error())
			}
		})
	}
}

func TestTimeToLiveDuration(t *testing.T) {
	cfg := JWTAuth{
		Secret:     EncryptionPassphrase,
		TimeToLive: "48h",
	}

	require.Equal(t, cfg.TimeToLive.Duration(), 48*time.Hour)

	cfg.TimeToLive = "1h"
	require.Equal(t, cfg.TimeToLive.Duration(), appdefaults.DefaultJWTTTL)

	cfg.TimeToLive = "72h"
	require.Equal(t, cfg.TimeToLive.Duration(), 72*time.Hour)

	cfg.TimeToLive = "2d"
	_, err := cfg.TimeToLive.ParseDuration()
	require.NotNil(t, err)
	require.EqualError(t, err, "time: unknown unit \"d\" in duration \"2d\"")
}

func TestNewConfig(t *testing.T) {
	cfg, err := NewConfig("testdata/test-valid-config.toml")
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "https://garm.example.com/", cfg.Default.CallbackURL)
	require.Equal(t, "0.0.0.0", cfg.APIServer.Bind)
	require.Equal(t, 9998, cfg.APIServer.Port)
	require.Equal(t, false, cfg.APIServer.UseTLS)
	require.Equal(t, DBBackendType("mysql"), cfg.Database.DbBackend)
	require.Equal(t, "bocyasicgatEtenOubwonIbsudNutDom", cfg.Database.Passphrase)
	require.Equal(t, "test", cfg.Database.MySQL.Username)
	require.Equal(t, "test", cfg.Database.MySQL.Password)
	require.Equal(t, "127.0.0.1", cfg.Database.MySQL.Hostname)
	require.Equal(t, "garm", cfg.Database.MySQL.DatabaseName)
	require.Equal(t, "bocyasicgatEtenOubwonIbsudNutDom", cfg.JWTAuth.Secret)
	require.Equal(t, timeToLive("48h"), cfg.JWTAuth.TimeToLive)
}

func TestNewConfigInvalidTomlPath(t *testing.T) {
	cfg, err := NewConfig("this is not a file path")
	require.Nil(t, cfg)
	require.NotNil(t, err)
	require.Regexp(t, "decoding toml", err.Error())
}

func TestNewConfigInvalidConfig(t *testing.T) {
	cfg, err := NewConfig("testdata/test-invalid-config.toml")
	require.Nil(t, cfg)
	require.NotNil(t, err)
	require.Regexp(t, "validating config", err.Error())
}

func TestGithubConfig(t *testing.T) {
	cfg := getDefaultGithubConfig()

	tests := []struct {
		name      string
		cfg       Github
		errString string
	}{
		{
			name:      "Config is valid",
			cfg:       cfg[0],
			errString: "",
		},
		{
			name: "BaseURL is invalid",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				BaseURL:     "bogus",
				AuthType:    GithubAuthTypePAT,
				PAT: GithubPAT{
					OAuth2Token: "bogus",
				},
			},
			errString: "invalid base_url: parse.*",
		},
		{
			name: "APIBaseURL is invalid",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				APIBaseURL:  "bogus",
				AuthType:    GithubAuthTypePAT,
				PAT: GithubPAT{
					OAuth2Token: "bogus",
				},
			},
			errString: "invalid api_base_url: parse.*",
		},
		{
			name: "UploadBaseURL is invalid",
			cfg: Github{
				Name:          "dummy_creds",
				Description:   "dummy github credentials",
				UploadBaseURL: "bogus",
				AuthType:      GithubAuthTypePAT,
				PAT: GithubPAT{
					OAuth2Token: "bogus",
				},
			},
			errString: "invalid upload_base_url: parse.*",
		},
		{
			name: "BaseURL is set and is valid",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				BaseURL:     "https://github.example.com/",
				AuthType:    GithubAuthTypePAT,
				PAT: GithubPAT{
					OAuth2Token: "bogus",
				},
			},
		},
		{
			name: "APIBaseURL is set and is valid",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				APIBaseURL:  "https://github.example.com/api/v3",
				AuthType:    GithubAuthTypePAT,
				PAT: GithubPAT{
					OAuth2Token: "bogus",
				},
			},
		},
		{
			name: "UploadBaseURL is set and is valid",
			cfg: Github{
				Name:          "dummy_creds",
				Description:   "dummy github credentials",
				UploadBaseURL: "https://github.example.com/uploads",
				AuthType:      GithubAuthTypePAT,
				PAT: GithubPAT{
					OAuth2Token: "bogus",
				},
			},
		},
		{
			name: "OAuth2Token is empty",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
			},
			errString: "missing github oauth2 token",
		},
		{
			name: "Name is empty",
			cfg: Github{
				Name:        "",
				Description: "dummy github credentials",
				OAuth2Token: "bogus",
			},
			errString: "missing credentials name",
		},
		{
			name: "Description is empty",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "",
				OAuth2Token: "bogus",
			},
			errString: "missing credentials description",
		},
		{
			name: "OAuth token is set in the PAT section",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
				AuthType:    GithubAuthTypePAT,
				PAT: GithubPAT{
					OAuth2Token: "bogus",
				},
			},
		},
		{
			name: "OAuth token is empty in the PAT section",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
				AuthType:    GithubAuthTypePAT,
				PAT: GithubPAT{
					OAuth2Token: "",
				},
			},
			errString: "missing github oauth2 token",
		},
		{
			name: "Valid App section",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
				AuthType:    GithubAuthTypeApp,
				App: GithubApp{
					AppID:          1,
					InstallationID: 99,
					PrivateKeyPath: "../testdata/certs/srv-key.pem",
				},
			},
		},
		{
			name: "AppID is missing",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
				AuthType:    GithubAuthTypeApp,
				App: GithubApp{
					AppID:          0,
					InstallationID: 99,
					PrivateKeyPath: "../testdata/certs/srv-key.pem",
				},
			},
			errString: "missing app_id",
		},
		{
			name: "InstallationID is missing",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
				AuthType:    GithubAuthTypeApp,
				App: GithubApp{
					AppID:          1,
					InstallationID: 0,
					PrivateKeyPath: "../testdata/certs/srv-key.pem",
				},
			},
			errString: "missing installation_id",
		},
		{
			name: "PrivateKeyPath is missing",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
				AuthType:    GithubAuthTypeApp,
				App: GithubApp{
					AppID:          1,
					InstallationID: 99,
					PrivateKeyPath: "",
				},
			},
			errString: "missing private_key_path",
		},
		{
			name: "PrivateKeyPath is invalid",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
				AuthType:    GithubAuthTypeApp,
				App: GithubApp{
					AppID:          1,
					InstallationID: 99,
					PrivateKeyPath: "/i/dont/exist",
				},
			},
			errString: "invalid github app config: error accessing private_key_path: stat /i/dont/exist: no such file or directory",
		},
		{
			name: "PrivateKeyPath is not a valid RSA private key",
			cfg: Github{
				Name:        "dummy_creds",
				Description: "dummy github credentials",
				OAuth2Token: "",
				AuthType:    GithubAuthTypeApp,
				App: GithubApp{
					AppID:          1,
					InstallationID: 99,
					PrivateKeyPath: "../testdata/certs/srv-pub.pem",
				},
			},
			errString: "invalid github app config: parsing private_key_path: asn1: structure error:.*",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Regexp(t, tc.errString, err.Error())
			}
		})
	}
}

func TestGithubAPIEndpoint(t *testing.T) {
	cfg := getDefaultGithubConfig()

	require.Equal(t, "https://api.github.com/", cfg[0].APIEndpoint())
}

func TestGithubAPIEndpointIsSet(t *testing.T) {
	cfg := getDefaultGithubConfig()
	cfg[0].APIBaseURL = "https://github.example.com/api/v3"

	require.Equal(t, "https://github.example.com/api/v3", cfg[0].APIEndpoint())
}

func TestUploadEndpoint(t *testing.T) {
	cfg := getDefaultGithubConfig()

	require.Equal(t, "https://uploads.github.com/", cfg[0].UploadEndpoint())
}

func TestUploadEndpointIsSet(t *testing.T) {
	cfg := getDefaultGithubConfig()
	cfg[0].UploadBaseURL = "https://github.example.com/uploads"

	require.Equal(t, "https://github.example.com/uploads", cfg[0].UploadEndpoint())
}

func TestGithubBaseURL(t *testing.T) {
	cfg := getDefaultGithubConfig()

	require.Equal(t, "https://github.com", cfg[0].BaseEndpoint())
}

func TestGithubBaseURLIsSet(t *testing.T) {
	cfg := getDefaultGithubConfig()
	cfg[0].BaseURL = "https://github.example.com"

	require.Equal(t, "https://github.example.com", cfg[0].BaseEndpoint())
}

func TestCACertBundle(t *testing.T) {
	cfg := Github{
		Name:             "dummy_creds",
		Description:      "dummy github credentials",
		OAuth2Token:      "bogus",
		CACertBundlePath: "../testdata/certs/srv-pub.pem",
	}

	cert, err := cfg.CACertBundle()
	require.Nil(t, err)
	require.NotNil(t, cert)
}

func TestCACertBundleInvalidPath(t *testing.T) {
	cfg := Github{
		Name:             "dummy_creds",
		Description:      "dummy github credentials",
		OAuth2Token:      "bogus",
		CACertBundlePath: "/i/dont/exist",
	}

	cert, err := cfg.CACertBundle()
	require.NotNil(t, err)
	require.EqualError(t, err, "error accessing ca_cert_bundle: stat /i/dont/exist: no such file or directory")
	require.Nil(t, cert)
}

func TestCACertBundleInvalidFile(t *testing.T) {
	cfg := Github{
		Name:             "dummy_creds",
		Description:      "dummy github credentials",
		OAuth2Token:      "bogus",
		CACertBundlePath: "../testdata/config.toml",
	}

	cert, err := cfg.CACertBundle()
	require.NotNil(t, err)
	require.EqualError(t, err, "failed to parse CA cert bundle")
	require.Nil(t, cert)
}

func TestGithubHTTPClientDeprecatedPAT(t *testing.T) {
	cfg := Github{
		Name:        "dummy_creds",
		Description: "dummy github credentials",
		OAuth2Token: "bogus",
	}

	client, err := cfg.HTTPClient(context.Background())
	require.Nil(t, err)
	require.NotNil(t, client)

	transport, ok := client.Transport.(*oauth2.Transport)
	require.True(t, ok)
	require.NotNil(t, transport)
}

func TestGithubHTTPClientPAT(t *testing.T) {
	cfg := Github{
		Name:        "dummy_creds",
		Description: "dummy github credentials",
		AuthType:    GithubAuthTypePAT,
		PAT: GithubPAT{
			OAuth2Token: "bogus",
		},
	}

	client, err := cfg.HTTPClient(context.Background())
	require.Nil(t, err)
	require.NotNil(t, client)

	transport, ok := client.Transport.(*oauth2.Transport)
	require.True(t, ok)
	require.NotNil(t, transport)
}

func TestGithubHTTPClientApp(t *testing.T) {
	cfg := Github{
		Name:        "dummy_creds",
		Description: "dummy github credentials",
		AuthType:    GithubAuthTypeApp,
		App: GithubApp{
			AppID:          1,
			InstallationID: 99,
			PrivateKeyPath: "../testdata/certs/srv-key.pem",
		},
	}

	client, err := cfg.HTTPClient(context.Background())
	require.Nil(t, err)
	require.NotNil(t, client)

	transport, ok := client.Transport.(*ghinstallation.Transport)
	require.True(t, ok)
	require.NotNil(t, transport)
}
