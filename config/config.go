package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

type DBBackendType string
type ProviderType string
type OSType string
type OSArch string

const (
	// MySQLBackend represents the MySQL DB backend
	MySQLBackend DBBackendType = "mysql"
	// SQLiteBackend represents the SQLite3 DB backend
	SQLiteBackend DBBackendType = "sqlite3"
	// DefaultJWTTTL is the default duration in seconds a JWT token
	// will be valid.
	DefaultJWTTTL time.Duration = 24 * time.Hour

	// LXDProvider represents the LXD provider.
	LXDProvider ProviderType = "lxd"

	// DefaultConfigFilePath is the default path on disk to the runner-manager
	// configuration file.
	DefaultConfigFilePath = "/etc/runner-manager/config.toml"
	// DefaultConfigDir is the default path on disk to the config dir. The config
	// file will probably be in the same folder, but it is not mandatory.
	DefaultConfigDir = "/etc/runner-manager"

	// DefaultUser is the default username that should exist on the instances.
	DefaultUser = "runner"
	// DefaultUserShell is the shell for the default user.
	DefaultUserShell = "/bin/bash"
)

var (
	// DefaultUserGroups are the groups the default user will be part of.
	DefaultUserGroups = []string{
		"sudo", "adm", "cdrom", "dialout",
		"dip", "video", "plugdev", "netdev",
	}
)

const (
	Windows OSType = "windows"
	Linux   OSType = "linux"
	Unknown OSType = "unknown"
)

const (
	Amd64 OSArch = "amd64"
	I386  OSArch = "i386"
	Arm64 OSArch = "arm64"
)

// NewConfig returns a new Config
func NewConfig(cfgFile string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(cfgFile, &config); err != nil {
		return nil, errors.Wrap(err, "decoding toml")
	}
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validating config")
	}
	if config.ConfigDir == "" {
		config.ConfigDir = DefaultConfigDir
	}
	return &config, nil
}

type Config struct {
	// ConfigDir is the folder where the runner may save any aditional files
	// or configurations it may need. Things like auto-generated SSH keys that
	// may be used to access the runner instances.
	ConfigDir    string       `toml:"config_dir" json:"config-dir"`
	APIServer    APIServer    `toml:"apiserver" json:"apiserver"`
	Database     Database     `toml:"database" json:"database"`
	Repositories []Repository `toml:"repository" json:"repository"`
	Providers    []Provider   `toml:"provider" json:"provider"`
	Github       Github       `toml:"github"`
	// LogFile is the location of the log file.
	LogFile string `toml:"log_file"`
}

// Validate validates the config
func (c *Config) Validate() error {
	if err := c.APIServer.Validate(); err != nil {
		return errors.Wrap(err, "validating APIServer config")
	}
	if err := c.Database.Validate(); err != nil {
		return errors.Wrap(err, "validating database config")
	}

	if err := c.Github.Validate(); err != nil {
		return errors.Wrap(err, "validating github config")
	}

	for _, provider := range c.Providers {
		if err := provider.Validate(); err != nil {
			return errors.Wrap(err, "validating provider")
		}
	}

	for _, repo := range c.Repositories {
		if err := repo.Validate(); err != nil {
			return errors.Wrap(err, "validating repository")
		}

		// We also need to validate that the provider used for this
		// repo, has been defined in the providers section. Multiple
		// repos can use the same provider.
		found := false
		for _, provider := range c.Providers {
			if provider.Name == repo.Pool.ProviderName {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("provider %s defined in repo %s/%s is not defined", repo.Pool.ProviderName, repo.Owner, repo.Name)
		}
	}

	return nil
}

// Github hold configuration options specific to interacting with github.
// Currently that is just a OAuth2 personal token.
type Github struct {
	OAuth2Token string `toml:"oauth2_token" json:"oauth2-token"`
}

func (g *Github) Validate() error {
	if g.OAuth2Token == "" {
		return fmt.Errorf("missing github oauth2 token")
	}
	return nil
}

// LXD holds connection information for an LXD cluster.
type LXD struct {
	// UnixSocket is the path on disk to the LXD unix socket. If defined,
	// this is prefered over connecting via HTTPs.
	UnixSocket string `toml:"unix_socket_path" json:"unix-socket-path"`

	// Project name is the name of the project in which this runner will create
	// instances. If this option is not set, the default project will be used.
	// The project used here, must have all required profiles created by you
	// beforehand. For LXD, the "flavor" used in the runner definition for a pool
	// equates to a profile in the desired project.
	ProjectName string `toml:"project_name" json:"project-name"`

	// IncludeDefaultProfile specifies whether or not this provider will always add
	// the "default" profile to any newly created instance.
	IncludeDefaultProfile bool `toml:"include_default_profile" json:"include-default-profile"`

	// URL holds the IP address.
	URL string `toml:"address" json:"address"`
	// ClientCertificate is the x509 client certificate path used for authentication.
	ClientCertificate string `toml:"client_certificate" json:"client_certificate"`
	// ClientKey is the key used for client certificate authentication.
	ClientKey string `toml:"client_key" json:"client-key"`
	// TLS certificate of the remote server. If not specified, the system CA is used.
	TLSServerCert string `toml:"tls_server_certificate" json:"tls-server-certificate"`
	// TLSCA is the TLS CA certificate when running LXD in PKI mode.
	TLSCA string `toml:"tls_ca" json:"tls-ca"`

	// TODO: add simplestreams sources
}

func (l *LXD) Validate() error {
	if l.UnixSocket != "" {
		if _, err := os.Stat(l.UnixSocket); err != nil {
			return fmt.Errorf("could not access unix socket %s: %q", l.UnixSocket, err)
		}

		return nil
	}

	if l.URL == "" {
		return fmt.Errorf("unix_socket or address must be specified")
	}

	if _, err := url.Parse(l.URL); err != nil {
		return fmt.Errorf("invalid LXD URL")
	}

	if l.ClientCertificate == "" || l.ClientKey == "" {
		return fmt.Errorf("client_certificate and client_key are mandatory when connecting via HTTPs")
	}

	if _, err := os.Stat(l.ClientCertificate); err != nil {
		return fmt.Errorf("failed to access client certificate %s: %q", l.ClientCertificate, err)
	}

	if _, err := os.Stat(l.ClientKey); err != nil {
		return fmt.Errorf("failed to access client key %s: %q", l.ClientKey, err)
	}

	if l.TLSServerCert != "" {
		if _, err := os.Stat(l.TLSServerCert); err != nil {
			return fmt.Errorf("failed to access tls_server_certificate %s: %q", l.TLSServerCert, err)
		}
	}
	return nil
}

// Provider holds access information for a particular provider.
// A provider offers compute resources on which we spin up self hosted runners.
type Provider struct {
	Name         string       `toml:"name" json:"name"`
	ProviderType ProviderType `toml:"provider_type" json:"provider-type"`
	LXD          LXD          `toml:"lxd" json:"lxd"`
}

func (p *Provider) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("missing provider name")
	}

	switch p.ProviderType {
	case LXDProvider:
		if err := p.LXD.Validate(); err != nil {
			return errors.Wrap(err, "validating LXD provider info")
		}
	default:
		return fmt.Errorf("unknown provider type: %s", p.ProviderType)
	}
	return nil
}

// Runner represents a runner type. The runner type is defined by the labels
// it has, the image it runs on and the size of the compute system that was
// requested.
type Runner struct {
	// Name is the name of this runner.
	Name string `toml:"name" json:"name"`
	// Labels is a list of labels that will be set for this runner in github.
	// The labels will be used in workflows to request a particular kind of
	// runner.
	Labels []string `toml:"labels" json:"labels"`
	// MaxRunners is the maximum number of self hosted action runners
	// of any type that are spun up for this repo. If current worker count
	// is not enough to handle jobs comming in, a new runner will be spun up,
	// until MaxWorkers count is hit.
	MaxRunners int `toml:"max_runners" json:"max-runners"`
	// MinRunners is the minimum number of self hosted runners that will
	// be maintained for this repo. If no jobs are sent to the workers,
	// idle workers will be removed until the MinWorkers setting is reached.
	MinRunners int `toml:"min_runners" json:"min-runners"`

	// Flavor is the size of the VM that will be spun up.
	Flavor string `toml:"flavor" json:"flavor"`
	// Image is the image that the VM will run. Each
	Image string `toml:"image" json:"image"`

	// OSType overrides the OS type that comes in from the Image. If the image
	// on a particular provider does not have this information set within it's metadata
	// you must set this option, so the runner-manager knows how to configure
	// the worker.
	OSType OSType `toml:"os_type" json:"os-type"`
	// OSArch overrides the OS architecture that comes in from the Image.
	// If the image metadata does not include information about the OS architecture,
	// you must set this option, so the runner-manager knows how to configure the worker.
	OSArch OSArch `toml:"os_arch" json:"os-arch"`
}

// TODO: validate rest
func (r *Runner) Validate() error {
	if len(r.Labels) == 0 {
		return fmt.Errorf("missing labels")
	}

	if r.Name == "" {
		return fmt.Errorf("name is not set")
	}

	return nil
}

type Pool struct {
	// ProviderName is the name of the provider that will be used for this pool.
	// A provider with the name specified in this setting, must be defined in
	// the Providers array in the main config.
	ProviderName string `toml:"provider_name" json:"provider-name"`

	// Runners represents a list of runner types defined for this pool.
	Runners []Runner `toml:"runners" json:"runners"`
}

func (p *Pool) Validate() error {
	if p.ProviderName == "" {
		return fmt.Errorf("missing provider_name")
	}

	if len(p.Runners) == 0 {
		return fmt.Errorf("no runners defined for pool")
	}

	for _, runner := range p.Runners {
		if err := runner.Validate(); err != nil {
			return errors.Wrap(err, "validating runner for pool")
		}
	}
	return nil
}

// Repository defines the settings for a pool associated with a particular repository.
type Repository struct {
	// Owner is the user under which the repo is created
	Owner string `toml:"owner" json:"owner"`
	// Name is the name of the repo.
	Name string `toml:"name" json:"name"`
	// WebsocketSecret is the shared secret used to create the hash of
	// the webhook body. We use this to validate that the webhook message
	// came in from the correct repo.
	WebhookSecret string `toml:"webhook_secret" json:"webhook-secret"`

	// Pool is the pool defined for this repository.
	Pool Pool `toml:"pool" json:"pool"`
}

func (r *Repository) String() string {
	return fmt.Sprintf("https://github.com/%s/%s", r.Owner, r.Name)
}

func (r *Repository) Validate() error {
	if r.Owner == "" {
		return fmt.Errorf("missing owner")
	}

	if r.Name == "" {
		return fmt.Errorf("missing repo name")
	}

	if r.WebhookSecret == "" {
		return fmt.Errorf("missing webhook_secret")
	}

	if err := r.Pool.Validate(); err != nil {
		return errors.Wrapf(err, "validating pool for %s", r)
	}

	return nil
}

// Database is the database config entry
type Database struct {
	Debug     bool          `toml:"debug" json:"debug"`
	DbBackend DBBackendType `toml:"backend" json:"backend"`
	MySQL     MySQL         `toml:"mysql" json:"mysql"`
	SQLite    SQLite        `toml:"sqlite3" json:"sqlite3"`
}

// GormParams returns the database type and connection URI
func (d *Database) GormParams() (dbType DBBackendType, uri string, err error) {
	if err := d.Validate(); err != nil {
		return "", "", errors.Wrap(err, "validating database config")
	}
	dbType = d.DbBackend
	switch dbType {
	case MySQLBackend:
		uri, err = d.MySQL.ConnectionString()
		if err != nil {
			return "", "", errors.Wrap(err, "validating mysql config")
		}
	case SQLiteBackend:
		uri, err = d.SQLite.ConnectionString()
		if err != nil {
			return "", "", errors.Wrap(err, "validating mysql config")
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
	switch d.DbBackend {
	case MySQLBackend:
		if err := d.MySQL.Validate(); err != nil {
			return errors.Wrap(err, "validating mysql config")
		}
	case SQLiteBackend:
		if err := d.SQLite.Validate(); err != nil {
			return errors.Wrap(err, "validating sqlite3 config")
		}
	default:
		return fmt.Errorf("invalid database backend: %s", d.DbBackend)
	}
	return nil
}

// SQLite is the config entry for the sqlite3 section
type SQLite struct {
	DBFile string `toml:"db_file" json:"db-file"`
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
		return errors.Wrapf(err, "accessing db_file parent dir: %s", parent)
	}
	return nil
}

func (s *SQLite) ConnectionString() (string, error) {
	return s.DBFile, nil
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
	CRT    string `toml:"certificate" json:"certificate"`
	Key    string `toml:"key" json:"key"`
	CACert string `toml:"ca_certificate" json:"ca-certificate"`
}

// TLSConfig returns a new TLSConfig suitable for use in the
// API server
func (t *TLSConfig) TLSConfig() (*tls.Config, error) {
	// TLS config not present.
	if t.CRT == "" && t.Key == "" {
		return nil, fmt.Errorf("missing crt or key")
	}

	var roots *x509.CertPool
	if t.CACert != "" {
		caCertPEM, err := ioutil.ReadFile(t.CACert)
		if err != nil {
			return nil, err
		}
		roots = x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(caCertPEM)
		if !ok {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
	}

	cert, err := tls.LoadX509KeyPair(t.CRT, t.Key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    roots,
	}, nil
}

// Validate validates the TLS config
func (t *TLSConfig) Validate() error {
	if _, err := t.TLSConfig(); err != nil {
		return err
	}
	return nil
}

// APIServer holds configuration for the API server
// worker
type APIServer struct {
	Bind        string    `toml:"bind" json:"bind"`
	Port        int       `toml:"port" json:"port"`
	UseTLS      bool      `toml:"use_tls" json:"use-tls"`
	TLSConfig   TLSConfig `toml:"tls" json:"tls"`
	CORSOrigins []string  `toml:"cors_origins" json:"cors-origins"`
}

// Validate validates the API server config
func (a *APIServer) Validate() error {
	if a.UseTLS {
		if err := a.TLSConfig.Validate(); err != nil {
			return errors.Wrap(err, "TLS validation failed")
		}
	}
	if a.Port > 65535 || a.Port < 1 {
		return fmt.Errorf("invalid port nr %q", a.Port)
	}

	ip := net.ParseIP(a.Bind)
	if ip == nil {
		// No need for deeper validation here, as any invalid
		// IP address specified in this setting will raise an error
		// when we try to bind to it.
		return fmt.Errorf("invalid IP address")
	}
	return nil
}
