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

package sql

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/appdefaults"
)

const (
	repositoryFieldName   string = "Repository"
	organizationFieldName string = "Organization"
	enterpriseFieldName   string = "Enterprise"
)

// newDBConn returns a new gorm db connection, given the config
func newDBConn(dbCfg config.Database) (conn *gorm.DB, err error) {
	dbType, connURI, err := dbCfg.GormParams()
	if err != nil {
		return nil, errors.Wrap(err, "getting DB URI string")
	}

	gormConfig := &gorm.Config{}
	if !dbCfg.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	switch dbType {
	case config.MySQLBackend:
		conn, err = gorm.Open(mysql.Open(connURI), gormConfig)
	case config.SQLiteBackend:
		conn, err = gorm.Open(sqlite.Open(connURI), gormConfig)
	}
	if err != nil {
		return nil, errors.Wrap(err, "connecting to database")
	}

	if dbCfg.Debug {
		conn = conn.Debug()
	}
	return conn, nil
}

func NewSQLDatabase(ctx context.Context, cfg config.Database) (common.Store, error) {
	conn, err := newDBConn(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating DB connection")
	}
	producer, err := watcher.RegisterProducer(ctx, "sql")
	if err != nil {
		return nil, errors.Wrap(err, "registering producer")
	}
	db := &sqlDatabase{
		conn:     conn,
		ctx:      ctx,
		cfg:      cfg,
		producer: producer,
	}

	if err := db.migrateDB(); err != nil {
		return nil, errors.Wrap(err, "migrating database")
	}
	return db, nil
}

type sqlDatabase struct {
	conn     *gorm.DB
	ctx      context.Context
	cfg      config.Database
	producer common.Producer
}

var renameTemplate = `
PRAGMA foreign_keys = OFF;
BEGIN TRANSACTION;

ALTER TABLE %s RENAME TO %s_old;
COMMIT;
`

var restoreNameTemplate = `
PRAGMA foreign_keys = OFF;
BEGIN TRANSACTION;
DROP TABLE IF EXISTS %s;
ALTER TABLE %s_old RENAME TO %s;
COMMIT;
`

var copyContentsTemplate = `
PRAGMA foreign_keys = OFF;
BEGIN TRANSACTION;
INSERT INTO %s SELECT * FROM %s_old;
DROP TABLE %s_old;

COMMIT;
`

func (s *sqlDatabase) cascadeMigrationSQLite(model interface{}, name string, justDrop bool) error {
	if !s.conn.Migrator().HasTable(name) {
		return nil
	}
	defer s.conn.Exec("PRAGMA foreign_keys = ON;")

	var data string
	var indexes []string
	if err := s.conn.Raw(fmt.Sprintf("select sql from sqlite_master where type='table' and tbl_name='%s'", name)).Scan(&data).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to get table %s: %w", name, err)
		}
	}

	if err := s.conn.Raw(fmt.Sprintf("SELECT name FROM sqlite_master WHERE type == 'index' AND tbl_name == '%s' and name not like 'sqlite_%%'", name)).Scan(&indexes).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to get table indexes %s: %w", name, err)
		}
	}

	if strings.Contains(data, "ON DELETE") {
		return nil
	}

	if justDrop {
		if err := s.conn.Migrator().DropTable(model); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", name, err)
		}
		return nil
	}

	for _, index := range indexes {
		if err := s.conn.Migrator().DropIndex(model, index); err != nil {
			return fmt.Errorf("failed to drop index %s: %w", index, err)
		}
	}

	err := s.conn.Exec(fmt.Sprintf(renameTemplate, name, name)).Error
	if err != nil {
		return fmt.Errorf("failed to rename table %s: %w", name, err)
	}

	if model != nil {
		if err := s.conn.Migrator().AutoMigrate(model); err != nil {
			if err := s.conn.Exec(fmt.Sprintf(restoreNameTemplate, name, name, name)).Error; err != nil {
				slog.With(slog.Any("error", err)).Error("failed to restore table", "table", name)
			}
			return fmt.Errorf("failed to create table %s: %w", name, err)
		}
	}
	err = s.conn.Exec(fmt.Sprintf(copyContentsTemplate, name, name, name)).Error
	if err != nil {
		return fmt.Errorf("failed to copy contents to table %s: %w", name, err)
	}

	return nil
}

func (s *sqlDatabase) cascadeMigration() error {
	switch s.cfg.DbBackend {
	case config.SQLiteBackend:
		if err := s.cascadeMigrationSQLite(&Address{}, "addresses", true); err != nil {
			return fmt.Errorf("failed to drop table addresses: %w", err)
		}

		if err := s.cascadeMigrationSQLite(&InstanceStatusUpdate{}, "instance_status_updates", true); err != nil {
			return fmt.Errorf("failed to drop table instance_status_updates: %w", err)
		}

		if err := s.cascadeMigrationSQLite(&Tag{}, "pool_tags", false); err != nil {
			return fmt.Errorf("failed to migrate addresses: %w", err)
		}

		if err := s.cascadeMigrationSQLite(&WorkflowJob{}, "workflow_jobs", false); err != nil {
			return fmt.Errorf("failed to migrate addresses: %w", err)
		}
	case config.MySQLBackend:
		return nil
	default:
		return fmt.Errorf("invalid db backend: %s", s.cfg.DbBackend)
	}
	return nil
}

func (s *sqlDatabase) ensureGithubEndpoint() error {
	// Create the default Github endpoint.
	createEndpointParams := params.CreateGithubEndpointParams{
		Name:          "github.com",
		Description:   "The github.com endpoint",
		APIBaseURL:    appdefaults.GithubDefaultBaseURL,
		BaseURL:       appdefaults.DefaultGithubURL,
		UploadBaseURL: appdefaults.GithubDefaultUploadBaseURL,
	}

	if _, err := s.CreateGithubEndpoint(context.Background(), createEndpointParams); err != nil {
		if !errors.Is(err, runnerErrors.ErrDuplicateEntity) {
			return errors.Wrap(err, "creating default github endpoint")
		}
	}

	return nil
}

func (s *sqlDatabase) migrateCredentialsToDB() (err error) {
	s.conn.Exec("PRAGMA foreign_keys = OFF")
	defer s.conn.Exec("PRAGMA foreign_keys = ON")

	adminUser, err := s.GetAdminUser(s.ctx)
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			// Admin user doesn't exist. This is a new deploy. Nothing to migrate.
			return nil
		}
		return errors.Wrap(err, "getting admin user")
	}

	// Impersonate the admin user. We're migrating from config credentials to
	// database credentials. At this point, there is no other user than the admin
	// user. GARM is not yet multi-user, so it's safe to assume we only have this
	// one user.
	adminCtx := context.Background()
	adminCtx = auth.PopulateContext(adminCtx, adminUser, nil)

	slog.Info("migrating credentials to DB")
	slog.Info("creating github endpoints table")
	if err := s.conn.AutoMigrate(&GithubEndpoint{}); err != nil {
		return errors.Wrap(err, "migrating github endpoints")
	}

	defer func() {
		if err != nil {
			slog.With(slog.Any("error", err)).Error("rolling back github github endpoints table")
			s.conn.Migrator().DropTable(&GithubEndpoint{})
		}
	}()

	slog.Info("creating github credentials table")
	if err := s.conn.AutoMigrate(&GithubCredentials{}); err != nil {
		return errors.Wrap(err, "migrating github credentials")
	}

	defer func() {
		if err != nil {
			slog.With(slog.Any("error", err)).Error("rolling back github github credentials table")
			s.conn.Migrator().DropTable(&GithubCredentials{})
		}
	}()

	// Nothing to migrate.
	if len(s.cfg.MigrateCredentials) == 0 {
		return nil
	}

	slog.Info("importing credentials from config")
	for _, cred := range s.cfg.MigrateCredentials {
		slog.Info("importing credential", "name", cred.Name)
		parsed, err := url.Parse(cred.BaseEndpoint())
		if err != nil {
			return errors.Wrap(err, "parsing base URL")
		}

		certBundle, err := cred.CACertBundle()
		if err != nil {
			return errors.Wrap(err, "getting CA cert bundle")
		}
		hostname := parsed.Hostname()
		createParams := params.CreateGithubEndpointParams{
			Name:          hostname,
			Description:   fmt.Sprintf("Endpoint for %s", hostname),
			APIBaseURL:    cred.APIEndpoint(),
			BaseURL:       cred.BaseEndpoint(),
			UploadBaseURL: cred.UploadEndpoint(),
			CACertBundle:  certBundle,
		}

		var endpoint params.GithubEndpoint
		endpoint, err = s.GetGithubEndpoint(adminCtx, hostname)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				return errors.Wrap(err, "getting github endpoint")
			}
			endpoint, err = s.CreateGithubEndpoint(adminCtx, createParams)
			if err != nil {
				return errors.Wrap(err, "creating default github endpoint")
			}
		}

		credParams := params.CreateGithubCredentialsParams{
			Name:        cred.Name,
			Description: cred.Description,
			Endpoint:    endpoint.Name,
			AuthType:    params.GithubAuthType(cred.GetAuthType()),
		}
		switch credParams.AuthType {
		case params.GithubAuthTypeApp:
			keyBytes, err := cred.App.PrivateKeyBytes()
			if err != nil {
				return errors.Wrap(err, "getting private key bytes")
			}
			credParams.App = params.GithubApp{
				AppID:           cred.App.AppID,
				InstallationID:  cred.App.InstallationID,
				PrivateKeyBytes: keyBytes,
			}

			if err := credParams.App.Validate(); err != nil {
				return errors.Wrap(err, "validating app credentials")
			}
		case params.GithubAuthTypePAT:
			token := cred.PAT.OAuth2Token
			if token == "" {
				token = cred.OAuth2Token
			}
			if token == "" {
				return errors.New("missing OAuth2 token")
			}
			credParams.PAT = params.GithubPAT{
				OAuth2Token: token,
			}
		}

		creds, err := s.CreateGithubCredentials(adminCtx, credParams)
		if err != nil {
			return errors.Wrap(err, "creating github credentials")
		}

		if err := s.conn.Exec("update repositories set credentials_id = ?,endpoint_name = ? where credentials_name = ?", creds.ID, creds.Endpoint.Name, creds.Name).Error; err != nil {
			return errors.Wrap(err, "updating repositories")
		}

		if err := s.conn.Exec("update organizations set credentials_id = ?,endpoint_name = ? where credentials_name = ?", creds.ID, creds.Endpoint.Name, creds.Name).Error; err != nil {
			return errors.Wrap(err, "updating organizations")
		}

		if err := s.conn.Exec("update enterprises set credentials_id = ?,endpoint_name = ? where credentials_name = ?", creds.ID, creds.Endpoint.Name, creds.Name).Error; err != nil {
			return errors.Wrap(err, "updating enterprises")
		}
	}
	return nil
}

func (s *sqlDatabase) migrateDB() error {
	if s.conn.Migrator().HasIndex(&Organization{}, "idx_organizations_name") {
		if err := s.conn.Migrator().DropIndex(&Organization{}, "idx_organizations_name"); err != nil {
			slog.With(slog.Any("error", err)).Error("failed to drop index idx_organizations_name")
		}
	}

	if s.conn.Migrator().HasIndex(&Repository{}, "idx_owner") {
		if err := s.conn.Migrator().DropIndex(&Repository{}, "idx_owner"); err != nil {
			slog.With(slog.Any("error", err)).Error("failed to drop index idx_owner")
		}
	}

	if err := s.cascadeMigration(); err != nil {
		return errors.Wrap(err, "running cascade migration")
	}

	if s.conn.Migrator().HasTable(&Pool{}) {
		if err := s.conn.Exec("update pools set repo_id=NULL where repo_id='00000000-0000-0000-0000-000000000000'").Error; err != nil {
			return errors.Wrap(err, "updating pools")
		}

		if err := s.conn.Exec("update pools set org_id=NULL where org_id='00000000-0000-0000-0000-000000000000'").Error; err != nil {
			return errors.Wrap(err, "updating pools")
		}

		if err := s.conn.Exec("update pools set enterprise_id=NULL where enterprise_id='00000000-0000-0000-0000-000000000000'").Error; err != nil {
			return errors.Wrap(err, "updating pools")
		}
	}

	if s.conn.Migrator().HasTable(&WorkflowJob{}) {
		if s.conn.Migrator().HasColumn(&WorkflowJob{}, "runner_name") {
			// Remove jobs that are not in "queued" status. We really only care about queued jobs. Once they transition
			// to something else, we don't really consume them anyway.
			if err := s.conn.Exec("delete from workflow_jobs where status is not 'queued'").Error; err != nil {
				return errors.Wrap(err, "updating workflow_jobs")
			}
			if err := s.conn.Migrator().DropColumn(&WorkflowJob{}, "runner_name"); err != nil {
				return errors.Wrap(err, "updating workflow_jobs")
			}
		}
	}

	if s.conn.Migrator().HasTable(&GithubEndpoint{}) {
		if !s.conn.Migrator().HasColumn(&GithubEndpoint{}, "endpoint_type") {
			if err := s.conn.Migrator().AutoMigrate(&GithubEndpoint{}); err != nil {
				return errors.Wrap(err, "migrating github endpoints")
			}
			if err := s.conn.Exec("update github_endpoints set endpoint_type = 'github' where endpoint_type is null").Error; err != nil {
				return errors.Wrap(err, "updating github endpoints")
			}
		}
	}

	var needsCredentialMigration bool
	if !s.conn.Migrator().HasTable(&GithubCredentials{}) || !s.conn.Migrator().HasTable(&GithubEndpoint{}) {
		needsCredentialMigration = true
	}

	var hasMinAgeField bool
	if s.conn.Migrator().HasTable(&ControllerInfo{}) && s.conn.Migrator().HasColumn(&ControllerInfo{}, "minimum_job_age_backoff") {
		hasMinAgeField = true
	}

	s.conn.Exec("PRAGMA foreign_keys = OFF")
	if err := s.conn.AutoMigrate(
		&User{},
		&GithubEndpoint{},
		&GithubCredentials{},
		&Tag{},
		&Pool{},
		&Repository{},
		&Organization{},
		&Enterprise{},
		&EnterpriseEvent{},
		&OrganizationEvent{},
		&RepositoryEvent{},
		&Address{},
		&InstanceStatusUpdate{},
		&Instance{},
		&ControllerInfo{},
		&WorkflowJob{},
		&ScaleSet{},
	); err != nil {
		return errors.Wrap(err, "running auto migrate")
	}
	s.conn.Exec("PRAGMA foreign_keys = ON")

	if !hasMinAgeField {
		var controller ControllerInfo
		if err := s.conn.First(&controller).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "updating controller info")
			}
		} else {
			controller.MinimumJobAgeBackoff = 30
			if err := s.conn.Save(&controller).Error; err != nil {
				return errors.Wrap(err, "updating controller info")
			}
		}
	}

	if err := s.ensureGithubEndpoint(); err != nil {
		return errors.Wrap(err, "ensuring github endpoint")
	}

	if needsCredentialMigration {
		if err := s.migrateCredentialsToDB(); err != nil {
			return errors.Wrap(err, "migrating credentials")
		}
	}
	return nil
}
