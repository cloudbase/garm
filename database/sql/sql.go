package sql

import (
	"context"
	"fmt"
	"runner-manager/config"
	"runner-manager/database/common"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/util"

	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewSQLDatabase(ctx context.Context, cfg config.Database) (common.Store, error) {
	conn, err := util.NewDBConn(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating DB connection")
	}
	db := &sqlDatabase{
		conn: conn,
		ctx:  ctx,
		cfg:  cfg,
	}

	if err := db.migrateDB(); err != nil {
		return nil, errors.Wrap(err, "migrating database")
	}
	return db, nil
}

type sqlDatabase struct {
	conn *gorm.DB
	ctx  context.Context
	cfg  config.Database
}

func (s *sqlDatabase) migrateDB() error {
	if err := s.conn.AutoMigrate(
		&Tag{},
		// &Runner{},
		&Pool{},
		&Repository{},
		&Organization{},
	); err != nil {
		return err
	}

	return nil
}

func (s *sqlDatabase) sqlToCommonTags(tag Tag) params.Tag {
	return params.Tag{
		ID:   tag.ID.String(),
		Name: tag.Name,
	}
}

// func (s *sqlDatabase) sqlToCommonRunner(runner Runner) params.Runner {
// 	ret := params.Runner{
// 		ID:             runner.ID.String(),
// 		MaxRunners:     runner.MaxRunners,
// 		MinIdleRunners: runner.MinIdleRunners,
// 		Image:          runner.Image,
// 		Flavor:         runner.Flavor,
// 		OSArch:         runner.OSArch,
// 		OSType:         runner.OSType,
// 		Tags:           make([]params.Tag, len(runner.Tags)),
// 	}

// 	for idx, val := range runner.Tags {
// 		ret.Tags[idx] = s.sqlToCommonTags(val)
// 	}

// 	return ret
// }

func (s *sqlDatabase) sqlToCommonPool(pool Pool) params.Pool {
	ret := params.Pool{
		ID:             pool.ID.String(),
		ProviderName:   pool.ProviderName,
		MaxRunners:     pool.MaxRunners,
		MinIdleRunners: pool.MinIdleRunners,
		Image:          pool.Image,
		Flavor:         pool.Flavor,
		OSArch:         pool.OSArch,
		OSType:         pool.OSType,
		Tags:           make([]params.Tag, len(pool.Tags)),
	}

	for idx, val := range pool.Tags {
		ret.Tags[idx] = s.sqlToCommonTags(val)
	}

	return ret
}

func (s *sqlDatabase) sqlToCommonRepository(repo Repository) params.Repository {
	ret := params.Repository{
		ID:    repo.ID.String(),
		Name:  repo.Name,
		Owner: repo.Owner,
		Pools: make([]params.Pool, len(repo.Pools)),
	}

	for idx, pool := range repo.Pools {
		ret.Pools[idx] = s.sqlToCommonPool(pool)
	}

	return ret
}

func (s *sqlDatabase) sqlToCommonOrganization(org Organization) params.Organization {
	ret := params.Organization{
		ID:    org.ID.String(),
		Name:  org.Name,
		Pools: make([]params.Pool, len(org.Pools)),
	}

	return ret
}

func (s *sqlDatabase) CreateRepository(ctx context.Context, owner, name, webhookSecret string) (params.Repository, error) {
	secret := []byte{}
	var err error
	if webhookSecret != "" {
		secret, err = util.Aes256EncodeString(webhookSecret, s.cfg.Passphrase)
		if err != nil {
			return params.Repository{}, fmt.Errorf("failed to encrypt string")
		}
	}
	newRepo := Repository{
		Name:          name,
		Owner:         owner,
		WebhookSecret: secret,
	}

	q := s.conn.Create(&newRepo)
	if q.Error != nil {
		return params.Repository{}, errors.Wrap(q.Error, "creating repository")
	}

	param := s.sqlToCommonRepository(newRepo)
	param.WebhookSecret = webhookSecret

	return param, nil
}

func (s *sqlDatabase) getRepo(ctx context.Context, id string) (Repository, error) {
	u := uuid.Parse(id)
	if u == nil {
		return Repository{}, errors.Wrap(runnerErrors.NewBadRequestError(""), "parsing id")
	}
	var repo Repository
	q := s.conn.Preload(clause.Associations).Where("id = ?", u).First(&repo)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Repository{}, runnerErrors.ErrNotFound
		}
		return Repository{}, errors.Wrap(q.Error, "fetching repository from database")
	}
	return repo, nil
}

func (s *sqlDatabase) GetRepository(ctx context.Context, id string) (params.Repository, error) {
	repo, err := s.getRepo(ctx, id)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

	param := s.sqlToCommonRepository(repo)
	secret, err := util.Aes256DecodeString(repo.WebhookSecret, s.cfg.Passphrase)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "decrypting secret")
	}
	param.WebhookSecret = secret

	return param, nil
}

func (s *sqlDatabase) ListRepositories(ctx context.Context) ([]params.Repository, error) {
	var repos []Repository
	q := s.conn.Find(&repos)
	if q.Error != nil {
		return []params.Repository{}, errors.Wrap(q.Error, "fetching user from database")
	}

	ret := make([]params.Repository, len(repos))
	for idx, val := range repos {
		ret[idx] = s.sqlToCommonRepository(val)
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteRepository(ctx context.Context, id string) error {
	repo, err := s.getRepo(ctx, id)
	if err != nil {
		if err == runnerErrors.ErrNotFound {
			return nil
		}
		return errors.Wrap(err, "fetching repo")
	}

	q := s.conn.Delete(&repo)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting repo")
	}

	return nil
}

func (s *sqlDatabase) CreateOrganization(ctx context.Context, name, webhookSecret string) (params.Organization, error) {
	secret := []byte{}
	var err error
	if webhookSecret != "" {
		secret, err = util.Aes256EncodeString(webhookSecret, s.cfg.Passphrase)
		if err != nil {
			return params.Organization{}, fmt.Errorf("failed to encrypt string")
		}
	}
	newOrg := Organization{
		Name:          name,
		WebhookSecret: secret,
	}

	q := s.conn.Create(&newOrg)
	if q.Error != nil {
		return params.Organization{}, errors.Wrap(q.Error, "creating org")
	}

	param := s.sqlToCommonOrganization(newOrg)
	param.WebhookSecret = webhookSecret

	return param, nil
}

func (s *sqlDatabase) getOrg(ctx context.Context, id string) (Organization, error) {
	u := uuid.Parse(id)
	if u == nil {
		return Organization{}, errors.Wrap(runnerErrors.NewBadRequestError(""), "parsing id")
	}
	var org Organization
	q := s.conn.Preload(clause.Associations).Where("id = ?", u).First(&org)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Organization{}, runnerErrors.ErrNotFound
		}
		return Organization{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return org, nil
}

func (s *sqlDatabase) GetOrganization(ctx context.Context, id string) (params.Organization, error) {
	org, err := s.getOrg(ctx, id)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching repo")
	}

	param := s.sqlToCommonOrganization(org)
	secret, err := util.Aes256DecodeString(org.WebhookSecret, s.cfg.Passphrase)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "decrypting secret")
	}
	param.WebhookSecret = secret

	return param, nil
}

func (s *sqlDatabase) ListOrganizations(ctx context.Context) ([]params.Organization, error) {
	var orgs []Organization
	q := s.conn.Find(&orgs)
	if q.Error != nil {
		return []params.Organization{}, errors.Wrap(q.Error, "fetching user from database")
	}

	ret := make([]params.Organization, len(orgs))
	for idx, val := range orgs {
		ret[idx] = s.sqlToCommonOrganization(val)
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteOrganization(ctx context.Context, id string) error {
	org, err := s.getOrg(ctx, id)
	if err != nil {
		if err == runnerErrors.ErrNotFound {
			return nil
		}
		return errors.Wrap(err, "fetching repo")
	}

	q := s.conn.Delete(&org)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting org")
	}

	return nil
}

func (s *sqlDatabase) getOrCreateTag(tagName string) (Tag, error) {
	var tag Tag
	q := s.conn.Where("name = ?", tagName).First(&tag)
	if q.Error == nil {
		return tag, nil
	}
	if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return Tag{}, errors.Wrap(q.Error, "fetching tag from database")
	}
	newTag := Tag{
		Name: tagName,
	}

	q = s.conn.Create(&newTag)
	if q.Error != nil {
		return Tag{}, errors.Wrap(q.Error, "creating tag")
	}
	return newTag, nil
}

func (s *sqlDatabase) CreateRepositoryPool(ctx context.Context, repoId string, param params.CreatePoolParams) (params.Pool, error) {
	if len(param.Tags) == 0 {
		return params.Pool{}, runnerErrors.NewBadRequestError("no tags specified")
	}

	repo, err := s.getRepo(ctx, repoId)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching repo")
	}

	newPool := Pool{
		ProviderName:   param.ProviderName,
		MaxRunners:     param.MaxRunners,
		MinIdleRunners: param.MinIdleRunners,
		Image:          param.Image,
		Flavor:         param.Flavor,
		OSType:         param.OSType,
		OSArch:         param.OSArch,
	}

	tags := make([]Tag, len(param.Tags))
	for idx, val := range param.Tags {
		t, err := s.getOrCreateTag(val)
		if err != nil {
			return params.Pool{}, errors.Wrap(err, "fetching tag")
		}
		tags[idx] = t
	}

	err = s.conn.Model(&repo).Association("Pools").Append(&newPool)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "adding pool")
	}
	return s.sqlToCommonPool(newPool), nil
}

func (s *sqlDatabase) CreateOrganizationPool(ctx context.Context, orgId string, param params.CreatePoolParams) (params.Pool, error) {
	if len(param.Tags) == 0 {
		return params.Pool{}, runnerErrors.NewBadRequestError("no tags specified")
	}

	org, err := s.getOrg(ctx, orgId)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching org")
	}

	newPool := Pool{
		ProviderName:   param.ProviderName,
		MaxRunners:     param.MaxRunners,
		MinIdleRunners: param.MinIdleRunners,
		Image:          param.Image,
		Flavor:         param.Flavor,
		OSType:         param.OSType,
		OSArch:         param.OSArch,
	}

	tags := make([]Tag, len(param.Tags))
	for idx, val := range param.Tags {
		t, err := s.getOrCreateTag(val)
		if err != nil {
			return params.Pool{}, errors.Wrap(err, "fetching tag")
		}
		tags[idx] = t
	}

	err = s.conn.Model(&org).Association("Pools").Append(&newPool)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "adding pool")
	}
	return s.sqlToCommonPool(newPool), nil
}

func (s *sqlDatabase) GetRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error) {
	repo, err := s.getRepo(ctx, repoID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching repo")
	}
	u := uuid.Parse(poolID)
	if u == nil {
		return params.Pool{}, fmt.Errorf("invalid pool id")
	}
	var pool []Pool
	err = s.conn.Model(&repo).Association("Pools").Find(&pool, "id = ?", u)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	if len(pool) == 0 {
		return params.Pool{}, runnerErrors.ErrNotFound
	}
	return s.sqlToCommonPool(pool[0]), nil
}

func (s *sqlDatabase) GetOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error) {
	org, err := s.getOrg(ctx, orgID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching org")
	}
	u := uuid.Parse(poolID)
	if u == nil {
		return params.Pool{}, fmt.Errorf("invalid pool id")
	}
	var pool []Pool
	err = s.conn.Model(&org).Association("Pools").Find(&pool, "id = ?", u)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	if len(pool) == 0 {
		return params.Pool{}, runnerErrors.ErrNotFound
	}
	return s.sqlToCommonPool(pool[0]), nil
}

func (s *sqlDatabase) DeleteRepositoryPool(ctx context.Context, repoID, poolID string) error {
	return nil
}

func (s *sqlDatabase) DeleteOrganizationPool(ctx context.Context, orgID, poolID string) error {
	return nil
}

func (s *sqlDatabase) UpdateRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error) {
	return params.Pool{}, nil
}

func (s *sqlDatabase) UpdateOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error) {
	return params.Pool{}, nil
}
