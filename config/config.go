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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/bradleyfalzon/ghinstallation/v2"
	zxcvbn "github.com/nbutton23/zxcvbn-go"
	"golang.org/x/oauth2"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/appdefaults"
)

type (
	DBBackendType  string
	LogLevel       string
	LogFormat      string
	GithubAuthType string
)

const (
	// MySQLBackend represents the MySQL DB backend
	MySQLBackend DBBackendType = "mysql"
	// SQLiteBackend represents the SQLite3 DB backend
	SQLiteBackend DBBackendType = "sqlite3"
	// EnvironmentVariablePrefix is the prefix for all environment variables
	// that can not be used to get overwritten via the external provider
	EnvironmentVariablePrefix = "GARM"
)

const (
	// LevelDebug is the debug log level
	LevelDebug LogLevel = "debug"
	// LevelInfo is the info log level
	LevelInfo LogLevel = "info"
	// LevelWarn is the warn log level
	LevelWarn LogLevel = "warn"
	// LevelError is the error log level
	LevelError LogLevel = "error"
)

const (
	// FormatText is the text log format
	FormatText LogFormat = "text"
	// FormatJSON is the json log format
	FormatJSON LogFormat = "json"
)

const (
	// GithubAuthTypePAT is the OAuth token based authentication
	GithubAuthTypePAT GithubAuthType = "pat"
	// GithubAuthTypeApp is the GitHub App based authentication
	GithubAuthTypeApp GithubAuthType = "app"
)

// NewConfig returns a new Config
func NewConfig(cfgFile string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(cfgFile, &config); err != nil {
		return nil, fmt.Errorf("error decoding toml: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("error validating config: %w", err)
	}
	return &config, nil
}

type Config struct {
	Default   Default    `toml:"default" json:"default"`
	APIServer APIServer  `toml:"apiserver,omitempty" json:"apiserver,omitempty"`
	Metrics   Metrics    `toml:"metrics,omitempty" json:"metrics,omitempty"`
	Database  Database   `toml:"database,omitempty" json:"database,omitempty"`
	Providers []Provider `toml:"provider,omitempty" json:"provider,omitempty"`
	Github    []Github   `toml:"github,omitempty"`
	JWTAuth   JWTAuth    `toml:"jwt_auth" json:"jwt-auth"`
	Logging   Logging    `toml:"logging" json:"logging"`
}

// Validate validates the config
func (c *Config) Validate() error {
	if err := c.APIServer.Validate(); err != nil {
		return fmt.Errorf("error validating apiserver config: %w", err)
	}
	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("error validating database config: %w", err)
	}

	if err := c.Default.Validate(); err != nil {
		return fmt.Errorf("error validating default config: %w", err)
	}

	for _, gh := range c.Github {
		if err := gh.Validate(); err != nil {
			return fmt.Errorf("error validating github config: %w", err)
		}
	}

	if err := c.JWTAuth.Validate(); err != nil {
		return fmt.Errorf("error validating jwt_auth config: %w", err)
	}

	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("error validating logging config: %w", err)
	}

	providerNames := map[string]int{}

	for _, provider := range c.Providers {
		if err := provider.Validate(); err != nil {
			return fmt.Errorf("error validating provider %s: %w", provider.Name, err)
		}
		providerNames[provider.Name]++
	}

	for name, count := range providerNames {
		if count > 1 {
			return fmt.Errorf("duplicate provider name %s", name)
		}
	}

	return nil
}

func (c *Config) GetLoggingConfig() Logging {
	logging := c.Logging
	if logging.LogFormat == "" {
		logging.LogFormat = FormatText
	}

	if logging.LogLevel == "" {
		logging.LogLevel = LevelInfo
	}

	// maintain backwards compatibility
	if logging.LogFile == "" && c.Default.LogFile != "" {
		logging.LogFile = c.Default.LogFile
	}
	if logging.EnableLogStreamer == nil && c.Default.EnableLogStreamer != nil {
		logging.EnableLogStreamer = c.Default.EnableLogStreamer
	}

	return logging
}

type Logging struct {
	// LogFile is the location of the log file.
	LogFile string `toml:"log_file,omitempty" json:"log-file"`
	// EnableLogStreamer enables the log streamer over websockets.
	EnableLogStreamer *bool `toml:"enable_log_streamer,omitempty" json:"enable-log-streamer,omitempty"`
	// LogLevel is the log level.
	LogLevel LogLevel `toml:"log_level" json:"log-format"`
	// LogFormat is the log format.
	LogFormat LogFormat `toml:"log_format" json:"log-level"`
	// LogSource enables the log source.
	LogSource bool `toml:"log_source" json:"log-source"`
}

func (l *Logging) Validate() error {
	if l.LogLevel != LevelDebug && l.LogLevel != LevelInfo && l.LogLevel != LevelWarn && l.LogLevel != LevelError && l.LogLevel != "" {
		return fmt.Errorf("invalid log level: %s", l.LogLevel)
	}

	if l.LogFormat != FormatText && l.LogFormat != FormatJSON && l.LogFormat != "" {
		return fmt.Errorf("invalid log format: %s", l.LogFormat)
	}

	return nil
}

type Default struct {
	// CallbackURL is the URL where the instances can send back status reports.
	CallbackURL string `toml:"callback_url" json:"callback-url"`
	// MetadataURL is the URL where instances can fetch information they may need
	// to set themselves up.
	MetadataURL string `toml:"metadata_url" json:"metadata-url"`
	// WebhookURL is the URL that will be installed as a webhook target in github.
	WebhookURL string `toml:"webhook_url" json:"webhook-url"`
	// EnableWebhookManagement enables the webhook management API.
	EnableWebhookManagement bool `toml:"enable_webhook_management" json:"enable-webhook-management"`

	// LogFile is the location of the log file.
	LogFile           string `toml:"log_file,omitempty" json:"log-file"`
	EnableLogStreamer *bool  `toml:"enable_log_streamer,omitempty" json:"enable-log-streamer,omitempty"`
	DebugServer       bool   `toml:"debug_server" json:"debug-server"`
}

func (d *Default) Validate() error {
	if d.CallbackURL != "" {
		_, err := url.ParseRequestURI(d.CallbackURL)
		if err != nil {
			return fmt.Errorf("invalid callback_url: %w", err)
		}
	}

	if d.MetadataURL != "" {
		if _, err := url.ParseRequestURI(d.MetadataURL); err != nil {
			return fmt.Errorf("invalid metadata_url: %w", err)
		}
	}

	if d.WebhookURL != "" {
		if _, err := url.ParseRequestURI(d.WebhookURL); err != nil {
			return fmt.Errorf("invalid webhook_url: %w", err)
		}
	}
	return nil
}

type GithubPAT struct {
	OAuth2Token string `toml:"oauth2_token" json:"oauth2-token"`
}

type GithubApp struct {
	AppID          int64  `toml:"app_id" json:"app-id"`
	PrivateKeyPath string `toml:"private_key_path" json:"private-key-path"`
	InstallationID int64  `toml:"installation_id" json:"installation-id"`
}

func (a *GithubApp) PrivateKeyBytes() ([]byte, error) {
	keyBytes, err := os.ReadFile(a.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading private_key_path: %w", err)
	}
	return keyBytes, nil
}

func (a *GithubApp) Validate() error {
	if a.AppID == 0 {
		return fmt.Errorf("missing app_id")
	}
	if a.PrivateKeyPath == "" {
		return fmt.Errorf("missing private_key_path")
	}
	if a.InstallationID == 0 {
		return fmt.Errorf("missing installation_id")
	}

	if _, err := os.Stat(a.PrivateKeyPath); err != nil {
		return fmt.Errorf("error accessing private_key_path: %w", err)
	}
	// Read the private key as bytes
	keyBytes, err := os.ReadFile(a.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("reading private_key_path: %w", err)
	}
	block, _ := pem.Decode(keyBytes)
	// Parse the private key as PCKS1
	_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parsing private_key_path: %w", err)
	}

	return nil
}

// Github hold configuration options specific to interacting with github.
// Currently that is just a OAuth2 personal token.
type Github struct {
	Name        string `toml:"name" json:"name"`
	Description string `toml:"description" json:"description"`
	// OAuth2Token is the personal access token used to authenticate with the
	// github API. This is deprecated and will be removed in the future.
	// Use the PAT section instead.
	OAuth2Token   string `toml:"oauth2_token" json:"oauth2-token"`
	APIBaseURL    string `toml:"api_base_url" json:"api-base-url"`
	UploadBaseURL string `toml:"upload_base_url" json:"upload-base-url"`
	BaseURL       string `toml:"base_url" json:"base-url"`
	// CACertBundlePath is the path on disk to a CA certificate bundle that
	// can validate the endpoints defined above. Leave empty if not using a
	// self signed certificate.
	CACertBundlePath string         `toml:"ca_cert_bundle" json:"ca-cert-bundle"`
	AuthType         GithubAuthType `toml:"auth_type" json:"auth-type"`
	PAT              GithubPAT      `toml:"pat" json:"pat"`
	App              GithubApp      `toml:"app" json:"app"`
}

func (g *Github) GetAuthType() GithubAuthType {
	if g.AuthType == "" {
		return GithubAuthTypePAT
	}
	return g.AuthType
}

func (g *Github) APIEndpoint() string {
	if g.APIBaseURL != "" {
		return g.APIBaseURL
	}
	return appdefaults.GithubDefaultBaseURL
}

func (g *Github) CACertBundle() ([]byte, error) {
	if g.CACertBundlePath == "" {
		// No CA bundle defined.
		return nil, nil
	}
	if _, err := os.Stat(g.CACertBundlePath); err != nil {
		return nil, fmt.Errorf("error accessing ca_cert_bundle: %w", err)
	}

	contents, err := os.ReadFile(g.CACertBundlePath)
	if err != nil {
		return nil, fmt.Errorf("reading ca_cert_bundle: %w", err)
	}

	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM(contents); !ok {
		return nil, fmt.Errorf("failed to parse CA cert bundle")
	}

	return contents, nil
}

func (g *Github) UploadEndpoint() string {
	if g.UploadBaseURL == "" {
		if g.APIBaseURL != "" {
			return g.APIBaseURL
		}
		return appdefaults.GithubDefaultUploadBaseURL
	}
	return g.UploadBaseURL
}

func (g *Github) BaseEndpoint() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	return appdefaults.DefaultGithubURL
}

func (g *Github) Validate() error {
	if g.Name == "" {
		return fmt.Errorf("missing credentials name")
	}

	if g.APIBaseURL != "" {
		if _, err := url.ParseRequestURI(g.APIBaseURL); err != nil {
			return fmt.Errorf("invalid api_base_url: %w", err)
		}
	}

	if g.UploadBaseURL != "" {
		if _, err := url.ParseRequestURI(g.UploadBaseURL); err != nil {
			return fmt.Errorf("invalid upload_base_url: %w", err)
		}
	}

	if g.BaseURL != "" {
		if _, err := url.ParseRequestURI(g.BaseURL); err != nil {
			return fmt.Errorf("invalid base_url: %w", err)
		}
	}

	switch g.AuthType {
	case GithubAuthTypeApp:
		if err := g.App.Validate(); err != nil {
			return fmt.Errorf("invalid github app config: %w", err)
		}
	default:
		if g.OAuth2Token == "" && g.PAT.OAuth2Token == "" {
			return fmt.Errorf("missing github oauth2 token")
		}
		if g.OAuth2Token != "" {
			slog.Warn("the github.oauth2_token option is deprecated, please use the PAT section")
		}
	}

	return nil
}

func (g *Github) HTTPClient(ctx context.Context) (*http.Client, error) {
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("invalid github config: %w", err)
	}
	var roots *x509.CertPool
	caBundle, err := g.CACertBundle()
	if err != nil {
		return nil, fmt.Errorf("fetching CA cert bundle: %w", err)
	}
	if caBundle != nil {
		roots = x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(caBundle)
		if !ok {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
	}
	// nolint:golangci-lint,gosec,godox
	// TODO: set TLS MinVersion
	httpTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: roots,
		},
	}

	var tc *http.Client
	switch g.AuthType {
	case GithubAuthTypeApp:
		itr, err := ghinstallation.NewKeyFromFile(httpTransport, g.App.AppID, g.App.InstallationID, g.App.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create github app installation transport: %w", err)
		}

		tc = &http.Client{Transport: itr}
	default:
		httpClient := &http.Client{Transport: httpTransport}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

		token := g.PAT.OAuth2Token
		if token == "" {
			token = g.OAuth2Token
		}

		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc = oauth2.NewClient(ctx, ts)
	}

	return tc, nil
}

// Provider holds access information for a particular provider.
// A provider offers compute resources on which we spin up self hosted runners.
type Provider struct {
	Name         string              `toml:"name" json:"name"`
	ProviderType params.ProviderType `toml:"provider_type" json:"provider-type"`
	Description  string              `toml:"description" json:"description"`
	// DisableJITConfig explicitly disables JIT configuration and forces runner registration
	// tokens to be used. This may happen if a provider has not yet been updated to support
	// JIT configuration.
	DisableJITConfig bool     `toml:"disable_jit_config" json:"disable-jit-config"`
	External         External `toml:"external" json:"external"`
}

func (p *Provider) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("missing provider name")
	}

	switch p.ProviderType {
	case params.ExternalProvider:
		if err := p.External.Validate(); err != nil {
			return fmt.Errorf("invalid external provider config: %w", err)
		}
	default:
		return fmt.Errorf("unknown provider type: %s", p.ProviderType)
	}
	return nil
}

// Database is the database config entry
type Database struct {
	Debug     bool          `toml:"debug" json:"debug"`
	DbBackend DBBackendType `toml:"backend" json:"backend"`
	MySQL     MySQL         `toml:"mysql" json:"mysql"`
	SQLite    SQLite        `toml:"sqlite3" json:"sqlite3"`
	// Passphrase is used to encrypt any sensitive info before
	// inserting it into the database. This is just temporary until
	// we move to something like vault or barbican for secrets storage.
	// Don't lose or change this. It will invalidate all encrypted data
	// in the DB. This field must be set and must be exactly 32 characters.
	Passphrase string `toml:"passphrase"`

	// MigrateCredentials is a list of github credentials that need to be migrated
	// from the config file to the database. This field will be removed once GARM
	// reaches version 0.2.x. It's only meant to be used for the migration process.
	MigrateCredentials []Github `toml:"-"`
}

// GormParams returns the database type and connection URI
func (d *Database) GormParams() (dbType DBBackendType, uri string, err error) {
	if err := d.Validate(); err != nil {
		return "", "", fmt.Errorf("error validating database config: %w", err)
	}
	dbType = d.DbBackend
	switch dbType {
	case MySQLBackend:
		uri, err = d.MySQL.ConnectionString()
		if err != nil {
			return "", "", fmt.Errorf("error fetching mysql connection string: %w", err)
		}
	case SQLiteBackend:
		uri, err = d.SQLite.ConnectionString()
		if err != nil {
			return "", "", fmt.Errorf("error fetching sqlite3 connection string: %w", err)
		}
	default:
		return "", "", fmt.Errorf("invalid database backend: %s", dbType)
	}
	return
}

// Validate validates the database config entry
func (d *Database) Validate() error {
	if d.DbBackend == "" {
		return fmt.Errorf("invalid databse configuration: backend is required")
	}

	if len(d.Passphrase) != 32 {
		return fmt.Errorf("passphrase must be set and it must be a string of 32 characters (aes 256)")
	}

	passwordStenght := zxcvbn.PasswordStrength(d.Passphrase, nil)
	if passwordStenght.Score < 4 {
		return fmt.Errorf("database passphrase is too weak")
	}

	switch d.DbBackend {
	case MySQLBackend:
		if err := d.MySQL.Validate(); err != nil {
			return fmt.Errorf("validating mysql config: %w", err)
		}
	case SQLiteBackend:
		if err := d.SQLite.Validate(); err != nil {
			return fmt.Errorf("validating sqlite3 config: %w", err)
		}
	default:
		return fmt.Errorf("invalid database backend: %s", d.DbBackend)
	}
	return nil
}

// SQLite is the config entry for the sqlite3 section
type SQLite struct {
	DBFile             string `toml:"db_file" json:"db-file"`
	BusyTimeoutSeconds int    `toml:"busy_timeout_seconds" json:"busy-timeout-seconds"`
}

func (s *SQLite) Validate() error {
	if s.DBFile == "" {
		return fmt.Errorf("no valid db_file was specified")
	}

	if !filepath.IsAbs(s.DBFile) {
		return fmt.Errorf("please specify an absolute path for db_file")
	}

	parent := filepath.Dir(s.DBFile)
	if _, err := os.Stat(parent); err != nil {
		return fmt.Errorf("parent directory of db_file does not exist: %w", err)
	}
	return nil
}

func (s *SQLite) ConnectionString() (string, error) {
	connectionString := fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=ON&_txlock=immediate", s.DBFile)
	if s.BusyTimeoutSeconds > 0 {
		timeout := s.BusyTimeoutSeconds * 1000
		connectionString = fmt.Sprintf("%s&_busy_timeout=%d", connectionString, timeout)
	}
	return connectionString, nil
}

// MySQL is the config entry for the mysql section
type MySQL struct {
	Username     string `toml:"username" json:"username"`
	Password     string `toml:"password" json:"password"`
	Hostname     string `toml:"hostname" json:"hostname"`
	DatabaseName string `toml:"database" json:"database"`
}

// Validate validates a Database config entry
func (m *MySQL) Validate() error {
	if m.Username == "" || m.Password == "" || m.Hostname == "" || m.DatabaseName == "" {
		return fmt.Errorf(
			"database, username, password, hostname are mandatory parameters for the database section")
	}
	return nil
}

// ConnectionString returns a gorm compatible connection string
func (m *MySQL) ConnectionString() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}

	connString := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=utf8&parseTime=True&loc=Local&timeout=5s",
		m.Username, m.Password,
		m.Hostname, m.DatabaseName,
	)
	return connString, nil
}

// TLSConfig is the API server TLS config
type TLSConfig struct {
	CRT string `toml:"certificate" json:"certificate"`
	Key string `toml:"key" json:"key"`
}

// Validate validates the TLS config
func (t *TLSConfig) Validate() error {
	if t.CRT == "" || t.Key == "" {
		return fmt.Errorf("missing crt or key")
	}

	_, err := tls.LoadX509KeyPair(t.CRT, t.Key)
	if err != nil {
		return err
	}
	return nil
}

type Metrics struct {
	// DisableAuth defines if the API endpoint will be protected by
	// JWT authentication
	DisableAuth bool `toml:"disable_auth" json:"disable-auth"`
	// Enable define if the API endpoint for metrics collection will
	// be enabled
	Enable bool `toml:"enable" json:"enable"`
	// Period defines the internal period at which internal metrics are getting updated
	// and propagated to the /metrics endpoint
	Period time.Duration `toml:"period" json:"period"`
}

// ParseDuration parses the configured duration and returns a time.Duration of 0
// if the duration is invalid.
func (m *Metrics) ParseDuration() (time.Duration, error) {
	duration, err := time.ParseDuration(fmt.Sprint(m.Period))
	if err != nil {
		return 0, err
	}
	return duration, nil
}

// Duration returns the configured duration or the default duration if no value
// is configured or the configured value is invalid.
func (m *Metrics) Duration() time.Duration {
	duration, err := m.ParseDuration()
	if err != nil {
		slog.With(slog.Any("error", err)).Error(fmt.Sprintf("defined duration %s is invalid", m.Period))
	}
	if duration == 0 {
		slog.Debug(fmt.Sprintf("using default duration %s for metrics update interval", appdefaults.DefaultMetricsUpdateInterval))
		return appdefaults.DefaultMetricsUpdateInterval
	}
	return duration
}

// WebUI holds configuration for the web UI
type WebUI struct {
	EnableWebUI bool `toml:"enable" json:"enable"`
}

// Validate validates the WebUI config
func (w *WebUI) Validate() error {
	return nil
}

// GetWebappPath returns the webapp path with proper formatting
func (w *WebUI) GetWebappPath() string {
	return "/ui/"
}

// APIServer holds configuration for the API server
// worker
type APIServer struct {
	Bind        string    `toml:"bind" json:"bind"`
	Port        int       `toml:"port" json:"port"`
	UseTLS      bool      `toml:"use_tls" json:"use-tls"`
	TLSConfig   TLSConfig `toml:"tls" json:"tls"`
	CORSOrigins []string  `toml:"cors_origins" json:"cors-origins"`
	WebUI       WebUI     `toml:"webui" json:"webui"`
}

// BindAddress returns a host:port string.
func (a *APIServer) BindAddress() string {
	return fmt.Sprintf("%s:%d", a.Bind, a.Port)
}

// Validate validates the API server config
func (a *APIServer) Validate() error {
	if a.UseTLS {
		if err := a.TLSConfig.Validate(); err != nil {
			return fmt.Errorf("invalid tls config: %w", err)
		}
	}
	if a.Port > 65535 || a.Port < 1 {
		return fmt.Errorf("invalid port nr %d", a.Port)
	}

	ip := net.ParseIP(a.Bind)
	if ip == nil {
		// No need for deeper validation here, as any invalid
		// IP address specified in this setting will raise an error
		// when we try to bind to it.
		return fmt.Errorf("invalid IP address")
	}

	if err := a.WebUI.Validate(); err != nil {
		return fmt.Errorf("invalid webui config: %w", err)
	}

	return nil
}

type timeToLive string

func (d *timeToLive) ParseDuration() (time.Duration, error) {
	duration, err := time.ParseDuration(string(*d))
	if err != nil {
		return 0, err
	}
	return duration, nil
}

func (d *timeToLive) Duration() time.Duration {
	duration, err := d.ParseDuration()
	if err != nil {
		slog.With(slog.Any("error", err)).Error("failed to parse duration")
		return appdefaults.DefaultJWTTTL
	}
	// nolint:golangci-lint,godox
	// TODO(gabriel-samfira): should we have a minimum TTL?
	if duration < appdefaults.DefaultJWTTTL {
		return appdefaults.DefaultJWTTTL
	}

	return duration
}

func (d *timeToLive) UnmarshalText(text []byte) error {
	_, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	*d = timeToLive(text)
	return nil
}

// JWTAuth holds settings used to generate JWT tokens
type JWTAuth struct {
	Secret     string     `toml:"secret" json:"secret"`
	TimeToLive timeToLive `toml:"time_to_live" json:"time-to-live"`
}

// Validate validates the JWTAuth config
func (j *JWTAuth) Validate() error {
	if _, err := j.TimeToLive.ParseDuration(); err != nil {
		return fmt.Errorf("invalid time_to_live: %w", err)
	}

	if j.Secret == "" {
		return fmt.Errorf("invalid JWT secret")
	}
	passwordStenght := zxcvbn.PasswordStrength(j.Secret, nil)
	if passwordStenght.Score < 4 {
		return fmt.Errorf("jwt_secret is too weak")
	}
	return nil
}
