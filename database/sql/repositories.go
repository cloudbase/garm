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

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateRepository(ctx context.Context, owner, name string, credentials params.ForgeCredentials, webhookSecret string, poolBalancerType params.PoolBalancerType) (param params.Repository, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.RepositoryEntityType, common.CreateOperation, param)
		}
	}()

	if webhookSecret == "" {
		return params.Repository{}, errors.New("creating repo: missing secret")
	}
	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Repository{}, fmt.Errorf("failed to encrypt string")
	}

	newRepo := Repository{
		Name:             name,
		Owner:            owner,
		WebhookSecret:    secret,
		PoolBalancerType: poolBalancerType,
	}
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		switch credentials.ForgeType {
		case params.GithubEndpointType:
			newRepo.CredentialsID = &credentials.ID
		case params.GiteaEndpointType:
			newRepo.GiteaCredentialsID = &credentials.ID
		default:
			return errors.Wrap(runnerErrors.ErrBadRequest, "unsupported credentials type")
		}

		newRepo.EndpointName = &credentials.Endpoint.Name
		q := tx.Create(&newRepo)
		if q.Error != nil {
			return errors.Wrap(q.Error, "creating repository")
		}
		return nil
	})
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "creating repository")
	}

	ret, err := s.GetRepositoryByID(ctx, newRepo.ID.String())
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "creating repository")
	}

	return ret, nil
}

func (s *sqlDatabase) GetRepository(ctx context.Context, owner, name, endpointName string) (params.Repository, error) {
	repo, err := s.getRepo(ctx, owner, name, endpointName)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

	param, err := s.sqlToCommonRepository(repo, true)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

	return param, nil
}

func (s *sqlDatabase) ListRepositories(_ context.Context) ([]params.Repository, error) {
	var repos []Repository
	q := s.conn.
		Preload("Credentials").
		Preload("GiteaCredentials").
		Preload("Credentials.Endpoint").
		Preload("GiteaCredentials.Endpoint").
		Preload("Endpoint").
		Find(&repos)
	if q.Error != nil {
		return []params.Repository{}, errors.Wrap(q.Error, "fetching user from database")
	}

	ret := make([]params.Repository, len(repos))
	for idx, val := range repos {
		var err error
		ret[idx], err = s.sqlToCommonRepository(val, true)
		if err != nil {
			return nil, errors.Wrap(err, "fetching repositories")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteRepository(ctx context.Context, repoID string) (err error) {
	repo, err := s.getRepoByID(ctx, s.conn, repoID, "Endpoint", "Credentials", "Credentials.Endpoint", "GiteaCredentials", "GiteaCredentials.Endpoint")
	if err != nil {
		return errors.Wrap(err, "fetching repo")
	}

	defer func(repo Repository) {
		if err == nil {
			asParam, innerErr := s.sqlToCommonRepository(repo, true)
			if innerErr == nil {
				s.sendNotify(common.RepositoryEntityType, common.DeleteOperation, asParam)
			} else {
				slog.With(slog.Any("error", innerErr)).ErrorContext(ctx, "error sending delete notification", "repo", repoID)
			}
		}
	}(repo)

	q := s.conn.Unscoped().Delete(&repo)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting repo")
	}

	return nil
}

func (s *sqlDatabase) UpdateRepository(ctx context.Context, repoID string, param params.UpdateEntityParams) (newParams params.Repository, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.RepositoryEntityType, common.UpdateOperation, newParams)
		}
	}()
	var repo Repository
	var creds GithubCredentials
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var err error
		repo, err = s.getRepoByID(ctx, tx, repoID)
		if err != nil {
			return errors.Wrap(err, "fetching repo")
		}
		if repo.EndpointName == nil {
			return errors.Wrap(runnerErrors.ErrUnprocessable, "repository has no endpoint")
		}

		if param.CredentialsName != "" {
			creds, err = s.getGithubCredentialsByName(ctx, tx, param.CredentialsName, false)
			if err != nil {
				return errors.Wrap(err, "fetching credentials")
			}
			if creds.EndpointName == nil {
				return errors.Wrap(runnerErrors.ErrUnprocessable, "credentials have no endpoint")
			}

			if *creds.EndpointName != *repo.EndpointName {
				return errors.Wrap(runnerErrors.ErrBadRequest, "endpoint mismatch")
			}
			repo.CredentialsID = &creds.ID
		}

		if param.WebhookSecret != "" {
			secret, err := util.Seal([]byte(param.WebhookSecret), []byte(s.cfg.Passphrase))
			if err != nil {
				return fmt.Errorf("saving repo: failed to encrypt string: %w", err)
			}
			repo.WebhookSecret = secret
		}

		if param.PoolBalancerType != "" {
			repo.PoolBalancerType = param.PoolBalancerType
		}

		q := tx.Save(&repo)
		if q.Error != nil {
			return errors.Wrap(q.Error, "saving repo")
		}

		return nil
	})
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "saving repo")
	}

	repo, err = s.getRepoByID(ctx, s.conn, repoID, "Endpoint", "Credentials", "Credentials.Endpoint", "GiteaCredentials", "GiteaCredentials.Endpoint")
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "updating enterprise")
	}

	newParams, err = s.sqlToCommonRepository(repo, true)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "saving repo")
	}
	return newParams, nil
}

func (s *sqlDatabase) GetRepositoryByID(ctx context.Context, repoID string) (params.Repository, error) {
	preloadList := []string{
		"Pools",
		"Credentials",
		"Endpoint",
		"Credentials.Endpoint",
		"GiteaCredentials",
		"GiteaCredentials.Endpoint",
		"Events",
	}
	repo, err := s.getRepoByID(ctx, s.conn, repoID, preloadList...)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

	param, err := s.sqlToCommonRepository(repo, true)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}
	return param, nil
}

func (s *sqlDatabase) getRepo(_ context.Context, owner, name, endpointName string) (Repository, error) {
	var repo Repository

	q := s.conn.Where("name = ? COLLATE NOCASE and owner = ? COLLATE NOCASE", name, owner)

	if endpointName != "" {
		q = q.Where("endpoint_name = ? COLLATE NOCASE", endpointName)
	}

	q = q.Preload("Credentials").
		Preload("Credentials.Endpoint").
		Preload("GiteaCredentials").
		Preload("GiteaCredentials.Endpoint").
		Preload("Endpoint")

	if endpointName == "" && q.Error == nil {
		var cnt int64
		q = q.Model(&Repository{})
		q = q.Count(&cnt)

		if q.Error != nil {
			return Repository{}, errors.Wrap(q.Error, "fetching repository from database")
		}
		if cnt > 1 {
			return Repository{}, errors.Wrap(runnerErrors.ErrBadRequest, "multiple repositories with the same name and owner found")
		} else if cnt == 0 {
			return Repository{}, runnerErrors.ErrNotFound
		}
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

func (s *sqlDatabase) getRepoByID(_ context.Context, tx *gorm.DB, id string, preload ...string) (Repository, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return Repository{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var repo Repository

	q := tx
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
