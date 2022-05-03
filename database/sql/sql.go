package sql

import (
	"context"
	"fmt"
	"runner-manager/config"
	"runner-manager/database/common"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/util"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
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
		&Pool{},
		&Repository{},
		&Organization{},
		&Address{},
		&Instance{},
		&ControllerInfo{},
		&User{},
	); err != nil {
		return err
	}

	return nil
}

func (s *sqlDatabase) sqlToCommonTags(tag Tag) params.Tag {
	return params.Tag{
		// ID:   tag.ID.String(),
		ID:   tag.ID.String(),
		Name: tag.Name,
	}
}

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
		Enabled:        pool.Enabled,
		Tags:           make([]params.Tag, len(pool.Tags)),
		Instances:      make([]params.Instance, len(pool.Instances)),
	}

	for idx, val := range pool.Tags {
		ret.Tags[idx] = s.sqlToCommonTags(*val)
	}

	for idx, inst := range pool.Instances {
		ret.Instances[idx] = s.sqlToParamsInstance(inst)
	}

	return ret
}

func (s *sqlDatabase) sqlToCommonRepository(repo Repository) params.Repository {
	ret := params.Repository{
		ID:              repo.ID.String(),
		Name:            repo.Name,
		Owner:           repo.Owner,
		CredentialsName: repo.CredentialsName,
		Pools:           make([]params.Pool, len(repo.Pools)),
	}

	for idx, pool := range repo.Pools {
		ret.Pools[idx] = s.sqlToCommonPool(pool)
	}

	return ret
}

func (s *sqlDatabase) sqlToCommonOrganization(org Organization) params.Organization {
	ret := params.Organization{
		ID:              org.ID.String(),
		Name:            org.Name,
		CredentialsName: org.CredentialsName,
		Pools:           make([]params.Pool, len(org.Pools)),
	}

	return ret
}

func (s *sqlDatabase) CreateRepository(ctx context.Context, owner, name, credentialsName, webhookSecret string) (params.Repository, error) {
	secret := []byte{}
	var err error
	if webhookSecret != "" {
		secret, err = util.Aes256EncodeString(webhookSecret, s.cfg.Passphrase)
		if err != nil {
			return params.Repository{}, fmt.Errorf("failed to encrypt string")
		}
	}
	newRepo := Repository{
		Name:            name,
		Owner:           owner,
		WebhookSecret:   secret,
		CredentialsName: credentialsName,
	}

	q := s.conn.Create(&newRepo)
	if q.Error != nil {
		return params.Repository{}, errors.Wrap(q.Error, "creating repository")
	}

	param := s.sqlToCommonRepository(newRepo)
	param.WebhookSecret = webhookSecret

	return param, nil
}

func (s *sqlDatabase) UpdateRepository(ctx context.Context, repoID string, param params.UpdateRepositoryParams) (params.Repository, error) {
	repo, err := s.getRepoByID(ctx, repoID)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

	if param.CredentialsName != "" {
		repo.CredentialsName = param.CredentialsName
	}

	if param.WebhookSecret != "" {
		secret, err := util.Aes256EncodeString(param.WebhookSecret, s.cfg.Passphrase)
		if err != nil {
			return params.Repository{}, fmt.Errorf("failed to encrypt string")
		}
		repo.WebhookSecret = secret
	}

	q := s.conn.Save(&repo)
	if q.Error != nil {
		return params.Repository{}, errors.Wrap(err, "saving repo")
	}

	newParams := s.sqlToCommonRepository(repo)
	secret, err := util.Aes256DecodeString(repo.WebhookSecret, s.cfg.Passphrase)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "decrypting secret")
	}
	newParams.WebhookSecret = secret
	return newParams, nil
}

func (s *sqlDatabase) getRepo(ctx context.Context, owner, name string, preloadAll bool) (Repository, error) {
	var repo Repository

	q := s.conn.Where("name = ? and owner = ?", name, owner).
		First(&repo)

	if preloadAll {
		q = q.Preload(clause.Associations)
	}

	q = q.First(&repo)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Repository{}, runnerErrors.ErrNotFound
		}
		return Repository{}, errors.Wrap(q.Error, "fetching repository from database")
	}
	return repo, nil
}

func (s *sqlDatabase) getRepoByID(ctx context.Context, id string, preload ...string) (Repository, error) {
	u, err := uuid.FromString(id)
	if err != nil {
		return Repository{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var repo Repository

	q := s.conn
	if len(preload) > 0 {
		for _, field := range preload {
			q = q.Preload(field)
		}
	}
	q = q.Where("id = ?", u).First(&repo)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Repository{}, runnerErrors.ErrNotFound
		}
		return Repository{}, errors.Wrap(q.Error, "fetching repository from database")
	}
	return repo, nil
}

func (s *sqlDatabase) GetRepository(ctx context.Context, owner, name string) (params.Repository, error) {
	repo, err := s.getRepo(ctx, owner, name, false)
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

func (s *sqlDatabase) GetRepositoryByID(ctx context.Context, repoID string) (params.Repository, error) {
	repo, err := s.getRepoByID(ctx, repoID, "Pools")
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
		if len(val.WebhookSecret) > 0 {
			secret, err := util.Aes256DecodeString(val.WebhookSecret, s.cfg.Passphrase)
			if err != nil {
				return nil, errors.Wrap(err, "decrypting secret")
			}
			ret[idx].WebhookSecret = secret
		}
	}

	return ret, nil
}

// func (s *sqlDatabase) DeleteRepository(ctx context.Context, owner, name string, hardDelete bool) error {
func (s *sqlDatabase) DeleteRepository(ctx context.Context, repoID string, hardDelete bool) error {
	repo, err := s.getRepoByID(ctx, repoID)
	if err != nil {
		return errors.Wrap(err, "fetching repo")
	}

	q := s.conn
	if hardDelete {
		q = q.Unscoped()
	}
	q = q.Delete(&repo)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting repo")
	}

	return nil
}

func (s *sqlDatabase) CreateOrganization(ctx context.Context, name, credentialsName, webhookSecret string) (params.Organization, error) {
	secret := []byte{}
	var err error
	if webhookSecret != "" {
		secret, err = util.Aes256EncodeString(webhookSecret, s.cfg.Passphrase)
		if err != nil {
			return params.Organization{}, fmt.Errorf("failed to encrypt string")
		}
	}
	newOrg := Organization{
		Name:            name,
		WebhookSecret:   secret,
		CredentialsName: credentialsName,
	}

	q := s.conn.Create(&newOrg)
	if q.Error != nil {
		return params.Organization{}, errors.Wrap(q.Error, "creating org")
	}

	param := s.sqlToCommonOrganization(newOrg)
	param.WebhookSecret = webhookSecret

	return param, nil
}

func (s *sqlDatabase) getOrg(ctx context.Context, name string, preloadAll bool) (Organization, error) {
	var org Organization

	q := s.conn.Where("name = ?", name)
	if preloadAll {
		q = q.Preload(clause.Associations)
	}

	q = q.First(&org)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Organization{}, runnerErrors.ErrNotFound
		}
		return Organization{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return org, nil
}

func (s *sqlDatabase) getOrgByID(ctx context.Context, id string, preloadAll bool) (Organization, error) {
	u, err := uuid.FromString(id)
	if err != nil {
		return Organization{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}

	q := s.conn.Where("id = ?", u)
	if preloadAll {
		q = q.Preload(clause.Associations)
	}

	var org Organization
	q = q.First(&org)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Organization{}, runnerErrors.ErrNotFound
		}
		return Organization{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return org, nil
}

func (s *sqlDatabase) GetOrganization(ctx context.Context, name string) (params.Organization, error) {
	org, err := s.getOrg(ctx, name, false)
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

func (s *sqlDatabase) DeleteOrganization(ctx context.Context, name string) error {
	org, err := s.getOrg(ctx, name, false)
	if err != nil {
		return errors.Wrap(err, "fetching repo")
	}

	q := s.conn.Unscoped().Delete(&org)
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

	repo, err := s.getRepoByID(ctx, repoId)
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
		RepoID:         repo.ID,
		Enabled:        param.Enabled,
	}

	_, err = s.getRepoPoolByUniqueFields(ctx, repoId, newPool.ProviderName, newPool.Image, newPool.Flavor)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Pool{}, errors.Wrap(err, "creating pool")
		}
	} else {
		return params.Pool{}, runnerErrors.NewConflictError("pool with the same image and flavor already exists on this provider")
	}

	tags := []Tag{}
	for _, val := range param.Tags {
		t, err := s.getOrCreateTag(val)
		if err != nil {
			return params.Pool{}, errors.Wrap(err, "fetching tag")
		}
		tags = append(tags, t)
	}

	q := s.conn.Create(&newPool)
	if q.Error != nil {
		return params.Pool{}, errors.Wrap(err, "adding pool")
	}

	for _, tt := range tags {
		s.conn.Model(&newPool).Association("Tags").Append(&tt)
	}

	pool, err := s.getPoolByID(ctx, newPool.ID.String(), "Tags")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) CreateOrganizationPool(ctx context.Context, orgId string, param params.CreatePoolParams) (params.Pool, error) {
	if len(param.Tags) == 0 {
		return params.Pool{}, runnerErrors.NewBadRequestError("no tags specified")
	}

	org, err := s.getOrgByID(ctx, orgId, false)
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
		Enabled:        param.Enabled,
	}

	tags := make([]*Tag, len(param.Tags))
	for idx, val := range param.Tags {
		t, err := s.getOrCreateTag(val)
		if err != nil {
			return params.Pool{}, errors.Wrap(err, "fetching tag")
		}
		tags[idx] = &t
	}

	newPool.Tags = append(newPool.Tags, tags...)
	err = s.conn.Model(&org).Association("Pools").Append(&newPool)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "adding pool")
	}
	return s.sqlToCommonPool(newPool), nil
}

func (s *sqlDatabase) getRepoPools(ctx context.Context, repoID string, preload ...string) ([]Pool, error) {
	repo, err := s.getRepoByID(ctx, repoID)
	if err != nil {
		return nil, errors.Wrap(err, "fetching repo")
	}

	var pools []Pool
	q := s.conn.Model(&repo)
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}
	err = q.Association("Pools").Find(&pools)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool")
	}

	return pools, nil
}

func (s *sqlDatabase) getOrgPools(ctx context.Context, orgID string, preloadAll bool) ([]Pool, error) {
	org, err := s.getOrgByID(ctx, orgID, preloadAll)
	if err != nil {
		return nil, errors.Wrap(err, "fetching repo")
	}

	var pools []Pool
	q := s.conn.Model(&org)
	if preloadAll {
		q = q.Preload(clause.Associations)
	}
	err = q.Association("Pools").Find(&pools)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool")
	}

	return pools, nil
}

func (s *sqlDatabase) ListRepoPools(ctx context.Context, repoID string) ([]params.Pool, error) {
	pools, err := s.getRepoPools(ctx, repoID, "Tags")
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}

	ret := make([]params.Pool, len(pools))
	for idx, pool := range pools {
		ret[idx] = s.sqlToCommonPool(pool)
	}

	return ret, nil
}

func (s *sqlDatabase) ListOrgPools(ctx context.Context, orgID string) ([]params.Pool, error) {
	pools, err := s.getOrgPools(ctx, orgID, false)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}

	ret := make([]params.Pool, len(pools))
	for idx, pool := range pools {
		ret[idx] = s.sqlToCommonPool(pool)
	}

	return ret, nil
}

func (s *sqlDatabase) getRepoPoolByUniqueFields(ctx context.Context, repoID string, provider, image, flavor string) (Pool, error) {
	repo, err := s.getRepoByID(ctx, repoID)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching repo")
	}

	q := s.conn
	var pool []Pool
	err = q.Model(&repo).Association("Pools").Find(&pool, "provider_name = ? and image = ? and flavor = ?", provider, image, flavor)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching pool")
	}
	if len(pool) == 0 {
		return Pool{}, runnerErrors.ErrNotFound
	}

	return pool[0], nil
}

func (s *sqlDatabase) getRepoPool(ctx context.Context, repoID, poolID string, preload ...string) (Pool, error) {
	repo, err := s.getRepoByID(ctx, repoID)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching repo")
	}

	u, err := uuid.FromString(poolID)
	if err != nil {
		return Pool{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}

	q := s.conn
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	var pool []Pool
	err = q.Model(&repo).Association("Pools").Find(&pool, "id = ?", u)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching pool")
	}
	if len(pool) == 0 {
		return Pool{}, runnerErrors.ErrNotFound
	}

	return pool[0], nil
}

func (s *sqlDatabase) GetRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error) {
	pool, err := s.getRepoPool(ctx, repoID, poolID, "Tags", "Instances")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) getOrgPool(ctx context.Context, orgID, poolID string, preloadAll bool) (Pool, error) {
	org, err := s.getOrgByID(ctx, orgID, preloadAll)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching repo")
	}
	u, err := uuid.FromString(poolID)
	if err != nil {
		return Pool{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var pool []Pool
	q := s.conn.Model(&org)
	if preloadAll {
		q = q.Preload(clause.Associations)
	}
	q = q.Find(&pool, "id = ?", u)

	if q.Error != nil {
		return Pool{}, errors.Wrap(q.Error, "fetching pool")
	}
	if len(pool) == 0 {
		return Pool{}, runnerErrors.ErrNotFound
	}

	return pool[0], nil
}

func (s *sqlDatabase) getPoolByID(ctx context.Context, poolID string, preload ...string) (Pool, error) {
	u, err := uuid.FromString(poolID)
	if err != nil {
		return Pool{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var pool Pool
	q := s.conn.Model(&Pool{})
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	q = q.Where("id = ?", u).First(&pool)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Pool{}, runnerErrors.ErrNotFound
		}
		return Pool{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return pool, nil
}

func (s *sqlDatabase) GetOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error) {
	pool, err := s.getOrgPool(ctx, orgID, poolID, false)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) DeleteRepositoryPool(ctx context.Context, repoID, poolID string) error {
	pool, err := s.getRepoPool(ctx, repoID, poolID)
	if err != nil {
		return errors.Wrap(err, "looking up repo pool")
	}
	q := s.conn.Unscoped().Delete(&pool)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting pool")
	}
	return nil
}

func (s *sqlDatabase) DeleteOrganizationPool(ctx context.Context, orgID, poolID string) error {
	pool, err := s.getOrgPool(ctx, orgID, poolID, false)
	if err != nil {
		return errors.Wrap(err, "looking up repo pool")
	}
	q := s.conn.Unscoped().Delete(&pool)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting pool")
	}
	return nil
}

func (s *sqlDatabase) findPoolByTags(id, poolType string, tags []string) (params.Pool, error) {
	if len(tags) == 0 {
		return params.Pool{}, runnerErrors.NewBadRequestError("missing tags")
	}
	u, err := uuid.FromString(id)
	if err != nil {
		return params.Pool{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}

	var pool Pool
	where := fmt.Sprintf("tags.name in ? and %s = ?", poolType)
	q := s.conn.Joins("JOIN pool_tags on pool_tags.pool_id=pools.id").
		Joins("JOIN tags on tags.id=pool_tags.tag_id").
		Group("pools.id").
		Preload("Tags").
		Having("count(1) = ?", len(tags)).
		Where(where, tags, u).First(&pool)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.Pool{}, runnerErrors.ErrNotFound
		}
		return params.Pool{}, errors.Wrap(q.Error, "fetching pool")
	}

	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) FindRepositoryPoolByTags(ctx context.Context, repoID string, tags []string) (params.Pool, error) {
	pool, err := s.findPoolByTags(repoID, "repo_id", tags)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (s *sqlDatabase) FindOrganizationPoolByTags(ctx context.Context, orgID string, tags []string) (params.Pool, error) {
	pool, err := s.findPoolByTags(orgID, "org_id", tags)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (s *sqlDatabase) sqlAddressToParamsAddress(addr Address) params.Address {
	return params.Address{
		Address: addr.Address,
		Type:    params.AddressType(addr.Type),
	}
}

func (s *sqlDatabase) sqlToParamsInstance(instance Instance) params.Instance {
	var id string
	if instance.ProviderID != nil {
		id = *instance.ProviderID
	}
	ret := params.Instance{
		ID:           instance.ID.String(),
		ProviderID:   id,
		Name:         instance.Name,
		OSType:       instance.OSType,
		OSName:       instance.OSName,
		OSVersion:    instance.OSVersion,
		OSArch:       instance.OSArch,
		Status:       instance.Status,
		RunnerStatus: instance.RunnerStatus,
		PoolID:       instance.PoolID.String(),
		CallbackURL:  instance.CallbackURL,
	}

	for _, addr := range instance.Addresses {
		ret.Addresses = append(ret.Addresses, s.sqlAddressToParamsAddress(addr))
	}
	return ret
}

func (s *sqlDatabase) CreateInstance(ctx context.Context, poolID string, param params.CreateInstanceParams) (params.Instance, error) {
	pool, err := s.getPoolByID(ctx, param.Pool)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching pool")
	}
	newInstance := Instance{
		Pool:         pool,
		Name:         param.Name,
		Status:       param.Status,
		RunnerStatus: param.RunnerStatus,
		OSType:       param.OSType,
		OSArch:       param.OSArch,
		CallbackURL:  param.CallbackURL,
	}
	q := s.conn.Create(&newInstance)
	if q.Error != nil {
		return params.Instance{}, errors.Wrap(q.Error, "creating repository")
	}

	return s.sqlToParamsInstance(newInstance), nil
}

// func (s *sqlDatabase) GetInstance(ctx context.Context, poolID string, instanceID string) (params.Instance, error) {
// 	return params.Instance{}, nil
// }

func (s *sqlDatabase) getInstanceByID(ctx context.Context, instanceID string) (Instance, error) {
	u, err := uuid.FromString(instanceID)
	if err != nil {
		return Instance{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("id = ?", u).
		First(&instance)
	if q.Error != nil {
		return Instance{}, errors.Wrap(q.Error, "fetching instance")
	}
	return instance, nil
}

func (s *sqlDatabase) getPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (Instance, error) {
	pool, err := s.getPoolByID(ctx, poolID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Instance{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching instance")
		}
		return Instance{}, errors.Wrap(err, "fetching pool")
	}

	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("name = ? and pool_id = ?", instanceName, pool.ID).
		First(&instance)
	if q.Error != nil {
		return Instance{}, errors.Wrap(q.Error, "fetching instance")
	}
	return instance, nil
}

func (s *sqlDatabase) getInstanceByName(ctx context.Context, instanceName string) (Instance, error) {
	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("name = ?", instanceName).
		First(&instance)
	if q.Error != nil {
		return Instance{}, errors.Wrap(q.Error, "fetching instance")
	}
	return instance, nil
}

func (s *sqlDatabase) GetPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (params.Instance, error) {
	instance, err := s.getPoolInstanceByName(ctx, poolID, instanceName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}
	return s.sqlToParamsInstance(instance), nil
}

func (s *sqlDatabase) GetInstanceByName(ctx context.Context, instanceName string) (params.Instance, error) {
	instance, err := s.getInstanceByName(ctx, instanceName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}
	return s.sqlToParamsInstance(instance), nil
}

func (s *sqlDatabase) DeleteInstance(ctx context.Context, poolID string, instanceName string) error {
	instance, err := s.getPoolInstanceByName(ctx, poolID, instanceName)
	if err != nil {
		return errors.Wrap(err, "deleting instance")
	}
	if q := s.conn.Unscoped().Delete(&instance); q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "deleting instance")
	}
	return nil
}

func (s *sqlDatabase) UpdateInstance(ctx context.Context, instanceID string, param params.UpdateInstanceParams) (params.Instance, error) {
	instance, err := s.getInstanceByID(ctx, instanceID)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "updating instance")
	}

	if param.ProviderID != "" {
		instance.ProviderID = &param.ProviderID
	}

	if param.OSName != "" {
		instance.OSName = param.OSName
	}

	if param.OSVersion != "" {
		instance.OSVersion = param.OSVersion
	}

	if string(param.RunnerStatus) != "" {
		instance.RunnerStatus = param.RunnerStatus
	}

	if string(param.Status) != "" {
		instance.Status = param.Status
	}

	q := s.conn.Save(&instance)
	if q.Error != nil {
		return params.Instance{}, errors.Wrap(q.Error, "updating instance")
	}

	if len(param.Addresses) > 0 {
		addrs := []Address{}
		for _, addr := range param.Addresses {
			addrs = append(addrs, Address{
				Address: addr.Address,
				Type:    string(addr.Type),
			})
		}
		if err := s.conn.Model(&instance).Association("Addresses").Replace(addrs); err != nil {
			return params.Instance{}, errors.Wrap(err, "updating addresses")
		}
	}
	return s.sqlToParamsInstance(instance), nil
}

func (s *sqlDatabase) ListInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	pool, err := s.getPoolByID(ctx, poolID, "Tags", "Instances")
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool")
	}

	ret := make([]params.Instance, len(pool.Instances))
	for idx, inst := range pool.Instances {
		ret[idx] = s.sqlToParamsInstance(inst)
	}
	return ret, nil
}

func (s *sqlDatabase) ListRepoInstances(ctx context.Context, repoID string) ([]params.Instance, error) {
	pools, err := s.getRepoPools(ctx, repoID, "Instances")
	if err != nil {
		return nil, errors.Wrap(err, "fetching repo")
	}

	ret := []params.Instance{}
	for _, pool := range pools {
		for _, instance := range pool.Instances {
			ret = append(ret, s.sqlToParamsInstance(instance))
		}
	}
	return ret, nil
}

func (s *sqlDatabase) ListOrgInstances(ctx context.Context, orgID string) ([]params.Instance, error) {
	org, err := s.getOrgByID(ctx, orgID, true)
	if err != nil {
		return nil, errors.Wrap(err, "fetching org")
	}
	ret := []params.Instance{}
	for _, pool := range org.Pools {
		for _, instance := range pool.Instances {
			ret = append(ret, s.sqlToParamsInstance(instance))
		}
	}
	return ret, nil
}

func (s *sqlDatabase) updatePool(pool Pool, param params.UpdatePoolParams) (params.Pool, error) {
	if param.Enabled != nil && pool.Enabled != *param.Enabled {
		pool.Enabled = *param.Enabled
	}

	if param.Flavor != "" {
		pool.Flavor = param.Flavor
	}

	if param.Image != "" {
		pool.Image = param.Image
	}

	if param.MaxRunners != nil {
		pool.MaxRunners = *param.MaxRunners
	}

	if param.MinIdleRunners != nil {
		pool.MinIdleRunners = *param.MinIdleRunners
	}

	if param.OSArch != "" {
		pool.OSArch = param.OSArch
	}

	if param.OSType != "" {
		pool.OSType = param.OSType
	}

	if q := s.conn.Save(&pool); q.Error != nil {
		return params.Pool{}, errors.Wrap(q.Error, "saving database entry")
	}

	if param.Tags != nil && len(param.Tags) > 0 {
		tags := make([]Tag, len(param.Tags))
		for idx, t := range param.Tags {
			tags[idx] = Tag{
				Name: t,
			}
		}

		if err := s.conn.Model(&pool).Association("Tags").Replace(&tags); err != nil {
			return params.Pool{}, errors.Wrap(err, "replacing tags")
		}
	}

	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) UpdateRepositoryPool(ctx context.Context, repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	pool, err := s.getRepoPool(ctx, repoID, poolID, "Tags")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.updatePool(pool, param)
}

func (s *sqlDatabase) UpdateOrganizationPool(ctx context.Context, orgID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	pool, err := s.getOrgPool(ctx, orgID, poolID, true)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.updatePool(pool, param)
}

func (s *sqlDatabase) sqlToParamsUser(user User) params.User {
	return params.User{
		ID:        user.ID.String(),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		Password:  user.Password,
		Enabled:   user.Enabled,
		IsAdmin:   user.IsAdmin,
	}
}

func (s *sqlDatabase) getUserByUsernameOrEmail(user string) (User, error) {
	field := "username"
	if util.IsValidEmail(user) {
		field = "email"
	}
	query := fmt.Sprintf("%s = ?", field)

	var dbUser User
	q := s.conn.Model(&User{}).Where(query, user).First(&dbUser)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return User{}, runnerErrors.ErrNotFound
		}
		return User{}, errors.Wrap(q.Error, "fetching user")
	}
	return dbUser, nil
}

func (s *sqlDatabase) getUserByID(userID string) (User, error) {
	var dbUser User
	q := s.conn.Model(&User{}).Where("id = ?", userID).First(&dbUser)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return User{}, runnerErrors.ErrNotFound
		}
		return User{}, errors.Wrap(q.Error, "fetching user")
	}
	return dbUser, nil
}

func (s *sqlDatabase) CreateUser(ctx context.Context, user params.NewUserParams) (params.User, error) {
	if user.Username == "" || user.Email == "" {
		return params.User{}, runnerErrors.NewBadRequestError("missing username or email")
	}
	if _, err := s.getUserByUsernameOrEmail(user.Username); err == nil || !errors.Is(err, runnerErrors.ErrNotFound) {
		return params.User{}, runnerErrors.NewConflictError("username already exists")
	}
	if _, err := s.getUserByUsernameOrEmail(user.Email); err == nil || !errors.Is(err, runnerErrors.ErrNotFound) {
		return params.User{}, runnerErrors.NewConflictError("email already exists")
	}

	newUser := User{
		Username: user.Username,
		Password: user.Password,
		FullName: user.FullName,
		Enabled:  user.Enabled,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
	}

	q := s.conn.Save(&newUser)
	if q.Error != nil {
		return params.User{}, errors.Wrap(q.Error, "creating user")
	}
	return s.sqlToParamsUser(newUser), nil
}

func (s *sqlDatabase) HasAdminUser(ctx context.Context) bool {
	var user User
	q := s.conn.Model(&User{}).Where("is_admin = ?", true).First(&user)
	if q.Error != nil {
		return false
	}
	return true
}

func (s *sqlDatabase) GetUser(ctx context.Context, user string) (params.User, error) {
	dbUser, err := s.getUserByUsernameOrEmail(user)
	if err != nil {
		return params.User{}, errors.Wrap(err, "fetching user")
	}
	return s.sqlToParamsUser(dbUser), nil
}

func (s *sqlDatabase) GetUserByID(ctx context.Context, userID string) (params.User, error) {
	dbUser, err := s.getUserByID(userID)
	if err != nil {
		return params.User{}, errors.Wrap(err, "fetching user")
	}
	return s.sqlToParamsUser(dbUser), nil
}

func (s *sqlDatabase) UpdateUser(ctx context.Context, user string, param params.UpdateUserParams) (params.User, error) {
	dbUser, err := s.getUserByUsernameOrEmail(user)
	if err != nil {
		return params.User{}, errors.Wrap(err, "fetching user")
	}

	if param.FullName != "" {
		dbUser.FullName = param.FullName
	}

	if param.Enabled != nil {
		dbUser.Enabled = *param.Enabled
	}

	if param.Password != "" {
		dbUser.Password = param.Password
	}

	if q := s.conn.Save(&dbUser); q.Error != nil {
		return params.User{}, errors.Wrap(q.Error, "saving user")
	}

	return s.sqlToParamsUser(dbUser), nil
}

func (s *sqlDatabase) ControllerInfo() (params.ControllerInfo, error) {
	var info ControllerInfo
	q := s.conn.Model(&ControllerInfo{}).First(&info)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.ControllerInfo{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching controller info")
		}
		return params.ControllerInfo{}, errors.Wrap(q.Error, "fetching controller info")
	}
	return params.ControllerInfo{
		ControllerID: info.ControllerID,
	}, nil
}

func (s *sqlDatabase) InitController() (params.ControllerInfo, error) {
	if _, err := s.ControllerInfo(); err == nil {
		return params.ControllerInfo{}, runnerErrors.NewConflictError("controller already initialized")
	}

	newID, err := uuid.NewV4()
	if err != nil {
		return params.ControllerInfo{}, errors.Wrap(err, "generating UUID")
	}

	newInfo := ControllerInfo{
		ControllerID: newID,
	}

	q := s.conn.Save(&newInfo)
	if q.Error != nil {
		return params.ControllerInfo{}, errors.Wrap(q.Error, "saving controller info")
	}

	return params.ControllerInfo{
		ControllerID: newInfo.ControllerID,
	}, nil
}
