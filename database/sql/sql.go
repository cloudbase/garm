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
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/internal/templates"
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
		return nil, fmt.Errorf("error getting DB URI string: %w", err)
	}

	gormConfig := &gorm.Config{
		TranslateError: true,
	}
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
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	if dbCfg.Debug {
		conn = conn.Debug()
	}

	return conn, nil
}

func NewSQLDatabase(ctx context.Context, cfg config.Database) (common.Store, error) {
	conn, err := newDBConn(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating DB connection: %w", err)
	}
	producer, err := watcher.RegisterProducer(ctx, "sql")
	if err != nil {
		return nil, fmt.Errorf("error registering producer: %w", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database connection: %w", err)
	}

	db := &sqlDatabase{
		conn:     conn,
		sqlDB:    sqlDB,
		ctx:      ctx,
		cfg:      cfg,
		producer: producer,
	}

	// Create separate connection for objects database (only for SQLite)
	if cfg.DbBackend == config.SQLiteBackend {
		// Get config for objects database
		objectsCfg, err := cfg.SQLiteBlobDatabaseConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get blob DB config: %w", err)
		}

		objectsConn, err := newDBConn(objectsCfg)
		if err != nil {
			return nil, fmt.Errorf("error creating objects DB connection: %w", err)
		}

		objectsSQLDB, err := objectsConn.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get underlying objects database connection: %w", err)
		}
		db.objectsConn = objectsConn
		db.objectsSQLDB = objectsSQLDB
	}

	if err := db.migrateDB(); err != nil {
		return nil, fmt.Errorf("error migrating database: %w", err)
	}
	return db, nil
}

type sqlDatabase struct {
	conn  *gorm.DB
	sqlDB *sql.DB

	// objectsConn is a separate GORM connection to the objects database
	objectsConn  *gorm.DB
	objectsSQLDB *sql.DB

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

	var epCount int64
	if err := s.conn.Model(&GithubEndpoint{}).Count(&epCount).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("error counting github endpoints: %w", err)
		}
	}

	if epCount == 0 {
		if _, err := s.CreateGithubEndpoint(context.Background(), createEndpointParams); err != nil {
			if !errors.Is(err, runnerErrors.ErrDuplicateEntity) {
				return fmt.Errorf("error creating default github endpoint: %w", err)
			}
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
		return fmt.Errorf("error getting admin user: %w", err)
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
		return fmt.Errorf("error migrating github endpoints: %w", err)
	}

	defer func() {
		if err != nil {
			slog.With(slog.Any("error", err)).Error("rolling back github github endpoints table")
			s.conn.Migrator().DropTable(&GithubEndpoint{})
		}
	}()

	slog.Info("creating github credentials table")
	if err := s.conn.AutoMigrate(&GithubCredentials{}); err != nil {
		return fmt.Errorf("error migrating github credentials: %w", err)
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
			return fmt.Errorf("error parsing base URL: %w", err)
		}

		certBundle, err := cred.CACertBundle()
		if err != nil {
			return fmt.Errorf("error getting CA cert bundle: %w", err)
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

		var endpoint params.ForgeEndpoint
		endpoint, err = s.GetGithubEndpoint(adminCtx, hostname)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				return fmt.Errorf("error getting github endpoint: %w", err)
			}
			endpoint, err = s.CreateGithubEndpoint(adminCtx, createParams)
			if err != nil {
				return fmt.Errorf("error creating default github endpoint: %w", err)
			}
		}

		credParams := params.CreateGithubCredentialsParams{
			Name:        cred.Name,
			Description: cred.Description,
			Endpoint:    endpoint.Name,
			AuthType:    params.ForgeAuthType(cred.GetAuthType()),
		}
		switch credParams.AuthType {
		case params.ForgeAuthTypeApp:
			keyBytes, err := cred.App.PrivateKeyBytes()
			if err != nil {
				return fmt.Errorf("error getting private key bytes: %w", err)
			}
			credParams.App = params.GithubApp{
				AppID:           cred.App.AppID,
				InstallationID:  cred.App.InstallationID,
				PrivateKeyBytes: keyBytes,
			}

			if err := credParams.App.Validate(); err != nil {
				return fmt.Errorf("error validating app credentials: %w", err)
			}
		case params.ForgeAuthTypePAT:
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
			return fmt.Errorf("error creating github credentials: %w", err)
		}

		if err := s.conn.Exec("update repositories set credentials_id = ?,endpoint_name = ? where credentials_name = ?", creds.ID, creds.Endpoint.Name, creds.Name).Error; err != nil {
			return fmt.Errorf("error updating repositories: %w", err)
		}

		if err := s.conn.Exec("update organizations set credentials_id = ?,endpoint_name = ? where credentials_name = ?", creds.ID, creds.Endpoint.Name, creds.Name).Error; err != nil {
			return fmt.Errorf("error updating organizations: %w", err)
		}

		if err := s.conn.Exec("update enterprises set credentials_id = ?,endpoint_name = ? where credentials_name = ?", creds.ID, creds.Endpoint.Name, creds.Name).Error; err != nil {
			return fmt.Errorf("error updating enterprises: %w", err)
		}
	}
	return nil
}

func (s *sqlDatabase) migrateWorkflow() error {
	if s.conn.Migrator().HasTable(&WorkflowJob{}) {
		if s.conn.Migrator().HasColumn(&WorkflowJob{}, "runner_name") {
			// Remove jobs that are not in "queued" status. We really only care about queued jobs. Once they transition
			// to something else, we don't really consume them anyway.
			if err := s.conn.Exec("delete from workflow_jobs where status is not 'queued'").Error; err != nil {
				return fmt.Errorf("error updating workflow_jobs: %w", err)
			}
			if err := s.conn.Migrator().DropColumn(&WorkflowJob{}, "runner_name"); err != nil {
				return fmt.Errorf("error updating workflow_jobs: %w", err)
			}
		}
	}
	return nil
}

func (s *sqlDatabase) migrateFileObjects() error {
	// Only migrate for SQLite backend
	if s.cfg.DbBackend != config.SQLiteBackend {
		return nil
	}

	// Use the separate objects database connection
	if s.objectsConn == nil {
		return fmt.Errorf("objects database connection not initialized")
	}

	// Use GORM AutoMigrate on the separate connection
	if err := s.objectsConn.AutoMigrate(&FileObject{}, &FileBlob{}, &FileObjectTag{}); err != nil {
		return fmt.Errorf("failed to migrate file objects: %w", err)
	}

	return nil
}

func (s *sqlDatabase) ensureTemplates(migrateTemplates bool) error {
	if !migrateTemplates {
		return nil
	}
	// make sure we have a default forge/OSType template. Currently we have Windows
	// and Linux for GitHub and Linux for Gitea.
	githubWindowsData, err := templates.GetTemplateContent(commonParams.Windows, params.GithubEndpointType)
	if err != nil {
		return fmt.Errorf("failed to get windows template for github: %w", err)
	}

	githubLinuxData, err := templates.GetTemplateContent(commonParams.Linux, params.GithubEndpointType)
	if err != nil {
		return fmt.Errorf("failed to get linux template for github: %w", err)
	}

	giteaLinuxData, err := templates.GetTemplateContent(commonParams.Linux, params.GiteaEndpointType)
	if err != nil {
		return fmt.Errorf("failed to get linux template for gitea: %w", err)
	}

	giteaWindowsData, err := templates.GetTemplateContent(commonParams.Windows, params.GiteaEndpointType)
	if err != nil {
		return fmt.Errorf("failed to get windows template for gitea: %w", err)
	}

	adminCtx := auth.GetAdminContext(s.ctx)

	githubWindowsParams := params.CreateTemplateParams{
		Name:        "github_windows",
		Description: "Default Windows runner install template for GitHub",
		OSType:      commonParams.Windows,
		ForgeType:   params.GithubEndpointType,
		Data:        githubWindowsData,
		IsSystem:    true,
	}
	githubWindowsSystemTemplate, err := s.CreateTemplate(adminCtx, githubWindowsParams)
	if err != nil {
		return fmt.Errorf("failed to create github windows template: %w", err)
	}

	githubLinuxParams := params.CreateTemplateParams{
		Name:        "github_linux",
		Description: "Default Linux runner install template for GitHub",
		OSType:      commonParams.Linux,
		ForgeType:   params.GithubEndpointType,
		Data:        githubLinuxData,
		IsSystem:    true,
	}
	githubLinuxSystemTemplate, err := s.CreateTemplate(adminCtx, githubLinuxParams)
	if err != nil {
		return fmt.Errorf("failed to create github linux template: %w", err)
	}

	giteaLinuxParams := params.CreateTemplateParams{
		Name:        "gitea_linux",
		Description: "Default Linux runner install template for Gitea",
		OSType:      commonParams.Linux,
		ForgeType:   params.GiteaEndpointType,
		Data:        giteaLinuxData,
		IsSystem:    true,
	}
	giteaLinuxSystemTemplate, err := s.CreateTemplate(adminCtx, giteaLinuxParams)
	if err != nil {
		return fmt.Errorf("failed to create gitea linux template: %w", err)
	}

	giteaWindowsParams := params.CreateTemplateParams{
		Name:        "gitea_windows",
		Description: "Default Windows runner install template for Gitea",
		OSType:      commonParams.Windows,
		ForgeType:   params.GiteaEndpointType,
		Data:        giteaWindowsData,
		IsSystem:    true,
	}
	giteaWindowsSystemTemplate, err := s.CreateTemplate(adminCtx, giteaWindowsParams)
	if err != nil {
		return fmt.Errorf("failed to create gitea windows template: %w", err)
	}

	getTplID := func(forgeType params.EndpointType, osType commonParams.OSType) uint {
		var templateID uint
		switch forgeType {
		case params.GiteaEndpointType:
			switch osType {
			case commonParams.Linux:
				templateID = giteaLinuxSystemTemplate.ID
			case commonParams.Windows:
				templateID = giteaWindowsSystemTemplate.ID
			default:
				return 0
			}
		case params.GithubEndpointType:
			switch osType {
			case commonParams.Linux:
				templateID = githubLinuxSystemTemplate.ID
			case commonParams.Windows:
				templateID = githubWindowsSystemTemplate.ID
			default:
				return 0
			}
		default:
			return 0
		}
		return templateID
	}

	pools, err := s.ListAllPools(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to list pools: %w", err)
	}

	for _, pool := range pools {
		forgeType := pool.Endpoint.EndpointType
		osType := pool.OSType
		entity, err := pool.GetEntity()
		if err != nil {
			return fmt.Errorf("failed to get pool entity: %w", err)
		}
		templateID := getTplID(forgeType, osType)
		if pool.TemplateID == 0 && templateID != 0 {
			updateParams := params.UpdatePoolParams{
				TemplateID: &templateID,
			}
			if _, err := s.UpdateEntityPool(adminCtx, entity, pool.ID, updateParams); err != nil {
				return fmt.Errorf("failed to update pool template: %w", err)
			}
		}
	}

	scaleSets, err := s.ListAllScaleSets(adminCtx)
	if err != nil {
		return fmt.Errorf("failed to list scale sets: %w", err)
	}

	for _, scaleSet := range scaleSets {
		forgeType := scaleSet.Endpoint.EndpointType
		osType := scaleSet.OSType
		entity, err := scaleSet.GetEntity()
		if err != nil {
			return fmt.Errorf("failed to get scale set entity: %w", err)
		}
		templateID := getTplID(forgeType, osType)
		if scaleSet.TemplateID == 0 && templateID != 0 {
			updateParams := params.UpdateScaleSetParams{
				TemplateID: &templateID,
			}
			if _, err := s.UpdateEntityScaleSet(adminCtx, entity, scaleSet.ID, updateParams, nil); err != nil {
				return fmt.Errorf("failed to update pool template: %w", err)
			}
		}
	}

	return nil
}

// dropIndexIfExists drops an index if it exists
func (s *sqlDatabase) dropIndexIfExists(model interface{}, indexName string) {
	if s.conn.Migrator().HasIndex(model, indexName) {
		if err := s.conn.Migrator().DropIndex(model, indexName); err != nil {
			slog.With(slog.Any("error", err)).
				Error(fmt.Sprintf("failed to drop index %s", indexName))
		}
	}
}

// migratePoolNullIDs updates pools to set null IDs instead of zero UUIDs
func (s *sqlDatabase) migratePoolNullIDs() error {
	if !s.conn.Migrator().HasTable(&Pool{}) {
		return nil
	}

	zeroUUID := "00000000-0000-0000-0000-000000000000"
	updates := []struct {
		column string
		query  string
	}{
		{"repo_id", fmt.Sprintf("update pools set repo_id=NULL where repo_id='%s'", zeroUUID)},
		{"org_id", fmt.Sprintf("update pools set org_id=NULL where org_id='%s'", zeroUUID)},
		{"enterprise_id", fmt.Sprintf("update pools set enterprise_id=NULL where enterprise_id='%s'", zeroUUID)},
	}

	for _, update := range updates {
		if err := s.conn.Exec(update.query).Error; err != nil {
			return fmt.Errorf("error updating pools %s: %w", update.column, err)
		}
	}
	return nil
}

// migrateGithubEndpointType adds and initializes endpoint_type column
func (s *sqlDatabase) migrateGithubEndpointType() error {
	if !s.conn.Migrator().HasTable(&GithubEndpoint{}) {
		return nil
	}

	if s.conn.Migrator().HasColumn(&GithubEndpoint{}, "endpoint_type") {
		return nil
	}

	if err := s.conn.Migrator().AutoMigrate(&GithubEndpoint{}); err != nil {
		return fmt.Errorf("error migrating github endpoints: %w", err)
	}

	if err := s.conn.Exec("update github_endpoints set endpoint_type = 'github' where endpoint_type is null").Error; err != nil {
		return fmt.Errorf("error updating github endpoints: %w", err)
	}

	return nil
}

// migrateControllerInfo updates controller info with new fields
func (s *sqlDatabase) migrateControllerInfo(hasMinAgeField, hasAgentURL bool) error {
	if hasMinAgeField && hasAgentURL {
		return nil
	}

	var controller ControllerInfo
	if err := s.conn.First(&controller).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("error fetching controller info: %w", err)
	}

	if !hasMinAgeField {
		controller.MinimumJobAgeBackoff = 30
	}

	if controller.GARMAgentReleasesURL == "" {
		controller.GARMAgentReleasesURL = appdefaults.GARMAgentDefaultReleasesURL
	}

	if !hasAgentURL && controller.WebhookBaseURL != "" {
		matchWebhooksPath := regexp.MustCompile(`/webhooks(/)?$`)
		controller.AgentURL = matchWebhooksPath.ReplaceAllLiteralString(controller.WebhookBaseURL, `/agent`)
	}

	if err := s.conn.Save(&controller).Error; err != nil {
		return fmt.Errorf("error updating controller info: %w", err)
	}

	return nil
}

// preMigrationChecks performs checks before running migrations
func (s *sqlDatabase) preMigrationChecks() (needsCredentialMigration, migrateTemplates, hasMinAgeField, hasAgentURL bool) {
	// Check if credentials need migration
	needsCredentialMigration = !s.conn.Migrator().HasTable(&GithubCredentials{}) ||
		!s.conn.Migrator().HasTable(&GithubEndpoint{})

	// Check if templates need migration
	migrateTemplates = !s.conn.Migrator().HasTable(&Template{})

	// Check for controller info fields
	if s.conn.Migrator().HasTable(&ControllerInfo{}) {
		hasMinAgeField = s.conn.Migrator().HasColumn(&ControllerInfo{}, "minimum_job_age_backoff")
		hasAgentURL = s.conn.Migrator().HasColumn(&ControllerInfo{}, "agent_url")
	}

	return
}

func (s *sqlDatabase) migrateDB() error {
	// Drop obsolete indexes
	s.dropIndexIfExists(&Organization{}, "idx_organizations_name")
	s.dropIndexIfExists(&Repository{}, "idx_owner")

	// Run cascade migration
	if err := s.cascadeMigration(); err != nil {
		return fmt.Errorf("error running cascade migration: %w", err)
	}

	// Migrate pool null IDs
	if err := s.migratePoolNullIDs(); err != nil {
		return err
	}

	// Migrate workflows
	if err := s.migrateWorkflow(); err != nil {
		return fmt.Errorf("error migrating workflows: %w", err)
	}

	// Migrate GitHub endpoint type
	if err := s.migrateGithubEndpointType(); err != nil {
		return err
	}

	// Check if we need to migrate credentials and templates
	needsCredentialMigration, migrateTemplates, hasMinAgeField, hasAgentURL := s.preMigrationChecks()

	// Run main schema migration
	s.conn.Exec("PRAGMA foreign_keys = OFF")
	if err := s.conn.AutoMigrate(
		&User{},
		&GithubEndpoint{},
		&GithubCredentials{},
		&GiteaCredentials{},
		&Tag{},
		&Template{},
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
		return fmt.Errorf("error running auto migrate: %w", err)
	}

	// Migrate file object tables in the attached objectsdb schema
	if err := s.migrateFileObjects(); err != nil {
		return fmt.Errorf("error migrating file objects: %w", err)
	}

	s.conn.Exec("PRAGMA foreign_keys = ON")

	// Migrate controller info if needed
	if err := s.migrateControllerInfo(hasMinAgeField, hasAgentURL); err != nil {
		return err
	}

	// Ensure github endpoint exists
	if err := s.ensureGithubEndpoint(); err != nil {
		return fmt.Errorf("error ensuring github endpoint: %w", err)
	}

	// Migrate credentials if needed
	if needsCredentialMigration {
		if err := s.migrateCredentialsToDB(); err != nil {
			return fmt.Errorf("error migrating credentials: %w", err)
		}
	}

	// Ensure templates exist
	if err := s.ensureTemplates(migrateTemplates); err != nil {
		return fmt.Errorf("failed to create default templates: %w", err)
	}

	return nil
}
