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

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/params"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *sqlDatabase) CreateRepository(ctx context.Context, owner, name, credentialsName, webhookSecret string) (params.Repository, error) {
	if webhookSecret == "" {
		return params.Repository{}, errors.New("creating repo: missing secret")
	}
	secret, err := util.Aes256EncodeString(webhookSecret, s.cfg.Passphrase)
	if err != nil {
		return params.Repository{}, fmt.Errorf("failed to encrypt string")
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

	param, err := s.sqlToCommonRepository(newRepo)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "creating repository")
	}

	return param, nil
}

func (s *sqlDatabase) GetRepository(ctx context.Context, owner, name string) (params.Repository, error) {
	repo, err := s.getRepo(ctx, owner, name)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

	param, err := s.sqlToCommonRepository(repo)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

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
		var err error
		ret[idx], err = s.sqlToCommonRepository(val)
		if err != nil {
			return nil, errors.Wrap(err, "fetching repositories")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteRepository(ctx context.Context, repoID string) error {
	repo, err := s.getRepoByID(ctx, repoID)
	if err != nil {
		return errors.Wrap(err, "fetching repo")
	}

	q := s.conn.Unscoped().Delete(&repo)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting repo")
	}

	return nil
}

func (s *sqlDatabase) UpdateRepository(ctx context.Context, repoID string, param params.UpdateEntityParams) (params.Repository, error) {
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
			return params.Repository{}, fmt.Errorf("saving repo: failed to encrypt string: %w", err)
		}
		repo.WebhookSecret = secret
	}

	q := s.conn.Save(&repo)
	if q.Error != nil {
		return params.Repository{}, errors.Wrap(q.Error, "saving repo")
	}

	newParams, err := s.sqlToCommonRepository(repo)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "saving repo")
	}
	return newParams, nil
}

func (s *sqlDatabase) GetRepositoryByID(ctx context.Context, repoID string) (params.Repository, error) {
	repo, err := s.getRepoByID(ctx, repoID, "Pools")
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

	param, err := s.sqlToCommonRepository(repo)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}
	return param, nil
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
		ProviderName:           param.ProviderName,
		MaxRunners:             param.MaxRunners,
		MinIdleRunners:         param.MinIdleRunners,
		RunnerPrefix:           param.GetRunnerPrefix(),
		Image:                  param.Image,
		Flavor:                 param.Flavor,
		OSType:                 param.OSType,
		OSArch:                 param.OSArch,
		RepoID:                 &repo.ID,
		Enabled:                param.Enabled,
		RunnerBootstrapTimeout: param.RunnerBootstrapTimeout,
	}

	if len(param.ExtraSpecs) > 0 {
		newPool.ExtraSpecs = datatypes.JSON(param.ExtraSpecs)
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
		return params.Pool{}, errors.Wrap(q.Error, "adding pool")
	}

	for _, tt := range tags {
		if err := s.conn.Model(&newPool).Association("Tags").Append(&tt); err != nil {
			return params.Pool{}, errors.Wrap(err, "saving tag")
		}
	}

	pool, err := s.getPoolByID(ctx, newPool.ID.String(), "Tags", "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) ListRepoPools(ctx context.Context, repoID string) ([]params.Pool, error) {
	pools, err := s.listEntityPools(ctx, params.RepositoryPool, repoID, "Tags", "Instances")
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}

	ret := make([]params.Pool, len(pools))
	for idx, pool := range pools {
		ret[idx] = s.sqlToCommonPool(pool)
	}

	return ret, nil
}

func (s *sqlDatabase) GetRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error) {
	pool, err := s.getEntityPool(ctx, params.RepositoryPool, repoID, poolID, "Tags", "Instances")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) DeleteRepositoryPool(ctx context.Context, repoID, poolID string) error {
	pool, err := s.getEntityPool(ctx, params.RepositoryPool, repoID, poolID)
	if err != nil {
		return errors.Wrap(err, "looking up repo pool")
	}
	q := s.conn.Unscoped().Delete(&pool)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting pool")
	}
	return nil
}

func (s *sqlDatabase) FindRepositoryPoolByTags(ctx context.Context, repoID string, tags []string) (params.Pool, error) {
	pool, err := s.findPoolByTags(repoID, params.RepositoryPool, tags)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool[0], nil
}

func (s *sqlDatabase) ListRepoInstances(ctx context.Context, repoID string) ([]params.Instance, error) {
	pools, err := s.listEntityPools(ctx, params.RepositoryPool, repoID, "Tags", "Instances")
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

func (s *sqlDatabase) UpdateRepositoryPool(ctx context.Context, repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	pool, err := s.getEntityPool(ctx, params.RepositoryPool, repoID, poolID, "Tags", "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.updatePool(pool, param)
}

func (s *sqlDatabase) getRepo(ctx context.Context, owner, name string) (Repository, error) {
	var repo Repository

	q := s.conn.Where("name = ? COLLATE NOCASE and owner = ? COLLATE NOCASE", name, owner).
		First(&repo)

	q = q.First(&repo)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Repository{}, runnerErrors.ErrNotFound
		}
		return Repository{}, errors.Wrap(q.Error, "fetching repository from database")
	}
	return repo, nil
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

func (s *sqlDatabase) getRepoByID(ctx context.Context, id string, preload ...string) (Repository, error) {
	u, err := uuid.Parse(id)
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
