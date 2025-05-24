// Copyright 2025 Cloudbase Solutions SRL
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
	"log/slog"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateGiteaEndpoint(_ context.Context, param params.CreateGiteaEndpointParams) (ghEndpoint params.ForgeEndpoint, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.CreateOperation, ghEndpoint)
		}
	}()
	var endpoint GithubEndpoint
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("name = ?", param.Name).First(&endpoint).Error; err == nil {
			return errors.Wrap(runnerErrors.ErrDuplicateEntity, "gitea endpoint already exists")
		}
		endpoint = GithubEndpoint{
			Name:         param.Name,
			Description:  param.Description,
			APIBaseURL:   param.APIBaseURL,
			BaseURL:      param.BaseURL,
			CACertBundle: param.CACertBundle,
			EndpointType: params.GiteaEndpointType,
		}

		if err := tx.Create(&endpoint).Error; err != nil {
			return errors.Wrap(err, "creating gitea endpoint")
		}
		return nil
	})
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "creating gitea endpoint")
	}
	ghEndpoint, err = s.sqlToCommonGithubEndpoint(endpoint)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "converting gitea endpoint")
	}
	return ghEndpoint, nil
}

func (s *sqlDatabase) ListGiteaEndpoints(_ context.Context) ([]params.ForgeEndpoint, error) {
	var endpoints []GithubEndpoint
	err := s.conn.Where("endpoint_type = ?", params.GiteaEndpointType).Find(&endpoints).Error
	if err != nil {
		return nil, errors.Wrap(err, "fetching gitea endpoints")
	}

	var ret []params.ForgeEndpoint
	for _, ep := range endpoints {
		commonEp, err := s.sqlToCommonGithubEndpoint(ep)
		if err != nil {
			return nil, errors.Wrap(err, "converting gitea endpoint")
		}
		ret = append(ret, commonEp)
	}
	return ret, nil
}

func (s *sqlDatabase) UpdateGiteaEndpoint(_ context.Context, name string, param params.UpdateGiteaEndpointParams) (ghEndpoint params.ForgeEndpoint, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.UpdateOperation, ghEndpoint)
		}
	}()
	var endpoint GithubEndpoint
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("name = ? and endpoint_type = ?", name, params.GiteaEndpointType).First(&endpoint).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(runnerErrors.ErrNotFound, "gitea endpoint not found")
			}
			return errors.Wrap(err, "fetching gitea endpoint")
		}

		var credsCount int64
		if err := tx.Model(&GiteaCredentials{}).Where("endpoint_name = ?", endpoint.Name).Count(&credsCount).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "fetching gitea credentials")
			}
		}
		if credsCount > 0 && (param.APIBaseURL != nil || param.BaseURL != nil) {
			return errors.Wrap(runnerErrors.ErrBadRequest, "cannot update endpoint URLs with existing credentials")
		}

		if param.APIBaseURL != nil {
			endpoint.APIBaseURL = *param.APIBaseURL
		}

		if param.BaseURL != nil {
			endpoint.BaseURL = *param.BaseURL
		}

		if param.CACertBundle != nil {
			endpoint.CACertBundle = param.CACertBundle
		}

		if param.Description != nil {
			endpoint.Description = *param.Description
		}

		if err := tx.Save(&endpoint).Error; err != nil {
			return errors.Wrap(err, "updating gitea endpoint")
		}

		return nil
	})
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "updating gitea endpoint")
	}
	ghEndpoint, err = s.sqlToCommonGithubEndpoint(endpoint)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "converting gitea endpoint")
	}
	return ghEndpoint, nil
}

func (s *sqlDatabase) GetGiteaEndpoint(_ context.Context, name string) (params.ForgeEndpoint, error) {
	var endpoint GithubEndpoint
	err := s.conn.Where("name = ? and endpoint_type = ?", name, params.GiteaEndpointType).First(&endpoint).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.ForgeEndpoint{}, errors.Wrap(runnerErrors.ErrNotFound, "gitea endpoint not found")
		}
		return params.ForgeEndpoint{}, errors.Wrap(err, "fetching gitea endpoint")
	}

	return s.sqlToCommonGithubEndpoint(endpoint)
}

func (s *sqlDatabase) DeleteGiteaEndpoint(_ context.Context, name string) (err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.DeleteOperation, params.ForgeEndpoint{Name: name})
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var endpoint GithubEndpoint
		if err := tx.Where("name = ? and endpoint_type = ?", name, params.GiteaEndpointType).First(&endpoint).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return errors.Wrap(err, "fetching gitea endpoint")
		}

		var credsCount int64
		if err := tx.Model(&GiteaCredentials{}).Where("endpoint_name = ?", endpoint.Name).Count(&credsCount).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "fetching gitea credentials")
			}
		}

		var repoCnt int64
		if err := tx.Model(&Repository{}).Where("endpoint_name = ?", endpoint.Name).Count(&repoCnt).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "fetching gitea repositories")
			}
		}

		var orgCnt int64
		if err := tx.Model(&Organization{}).Where("endpoint_name = ?", endpoint.Name).Count(&orgCnt).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "fetching gitea organizations")
			}
		}

		if credsCount > 0 || repoCnt > 0 || orgCnt > 0 {
			return errors.Wrap(runnerErrors.ErrBadRequest, "cannot delete endpoint with associated entities")
		}

		if err := tx.Unscoped().Delete(&endpoint).Error; err != nil {
			return errors.Wrap(err, "deleting gitea endpoint")
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "deleting gitea endpoint")
	}
	return nil
}

func (s *sqlDatabase) CreateGiteaCredentials(ctx context.Context, param params.CreateGiteaCredentialsParams) (gtCreds params.ForgeCredentials, err error) {
	userID, err := getUIDFromContext(ctx)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "creating gitea credentials")
	}
	if param.Endpoint == "" {
		return params.ForgeCredentials{}, errors.Wrap(runnerErrors.ErrBadRequest, "endpoint name is required")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.GiteaCredentialsEntityType, common.CreateOperation, gtCreds)
		}
	}()
	var creds GiteaCredentials
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var endpoint GithubEndpoint
		if err := tx.Where("name = ? and endpoint_type = ?", param.Endpoint, params.GiteaEndpointType).First(&endpoint).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(runnerErrors.ErrNotFound, "gitea endpoint not found")
			}
			return errors.Wrap(err, "fetching gitea endpoint")
		}

		if err := tx.Where("name = ? and user_id = ?", param.Name, userID).First(&creds).Error; err == nil {
			return errors.Wrap(runnerErrors.ErrDuplicateEntity, "gitea credentials already exists")
		}

		var data []byte
		var err error
		switch param.AuthType {
		case params.ForgeAuthTypePAT:
			data, err = s.marshalAndSeal(param.PAT)
		default:
			return errors.Wrap(runnerErrors.ErrBadRequest, "invalid auth type")
		}
		if err != nil {
			return errors.Wrap(err, "marshaling and sealing credentials")
		}

		creds = GiteaCredentials{
			Name:         param.Name,
			Description:  param.Description,
			EndpointName: &endpoint.Name,
			AuthType:     param.AuthType,
			Payload:      data,
			UserID:       &userID,
		}

		if err := tx.Create(&creds).Error; err != nil {
			return errors.Wrap(err, "creating gitea credentials")
		}
		// Skip making an extra query.
		creds.Endpoint = endpoint

		return nil
	})
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "creating gitea credentials")
	}
	gtCreds, err = s.sqlGiteaToCommonForgeCredentials(creds)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "converting gitea credentials")
	}
	return gtCreds, nil
}

func (s *sqlDatabase) getGiteaCredentialsByName(ctx context.Context, tx *gorm.DB, name string, detailed bool) (GiteaCredentials, error) {
	var creds GiteaCredentials
	q := tx.Preload("Endpoint")

	if detailed {
		q = q.
			Preload("Repositories").
			Preload("Organizations").
			Preload("Repositories.GiteaCredentials").
			Preload("Organizations.GiteaCredentials").
			Preload("Repositories.Credentials").
			Preload("Organizations.Credentials")
	}

	userID, err := getUIDFromContext(ctx)
	if err != nil {
		return GiteaCredentials{}, errors.Wrap(err, "fetching gitea credentials")
	}
	q = q.Where("user_id = ?", userID)

	err = q.Where("name = ?", name).First(&creds).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return GiteaCredentials{}, errors.Wrap(runnerErrors.ErrNotFound, "gitea credentials not found")
		}
		return GiteaCredentials{}, errors.Wrap(err, "fetching gitea credentials")
	}

	return creds, nil
}

func (s *sqlDatabase) GetGiteaCredentialsByName(ctx context.Context, name string, detailed bool) (params.ForgeCredentials, error) {
	creds, err := s.getGiteaCredentialsByName(ctx, s.conn, name, detailed)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "fetching gitea credentials")
	}

	return s.sqlGiteaToCommonForgeCredentials(creds)
}

func (s *sqlDatabase) GetGiteaCredentials(ctx context.Context, id uint, detailed bool) (params.ForgeCredentials, error) {
	var creds GiteaCredentials
	q := s.conn.Preload("Endpoint")

	if detailed {
		q = q.
			Preload("Repositories").
			Preload("Organizations").
			Preload("Repositories.GiteaCredentials").
			Preload("Organizations.GiteaCredentials").
			Preload("Repositories.Credentials").
			Preload("Organizations.Credentials")
	}

	if !auth.IsAdmin(ctx) {
		userID, err := getUIDFromContext(ctx)
		if err != nil {
			return params.ForgeCredentials{}, errors.Wrap(err, "fetching gitea credentials")
		}
		q = q.Where("user_id = ?", userID)
	}

	err := q.Where("id = ?", id).First(&creds).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.ForgeCredentials{}, errors.Wrap(runnerErrors.ErrNotFound, "gitea credentials not found")
		}
		return params.ForgeCredentials{}, errors.Wrap(err, "fetching gitea credentials")
	}

	return s.sqlGiteaToCommonForgeCredentials(creds)
}

func (s *sqlDatabase) ListGiteaCredentials(ctx context.Context) ([]params.ForgeCredentials, error) {
	q := s.conn.Preload("Endpoint")
	if !auth.IsAdmin(ctx) {
		userID, err := getUIDFromContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "fetching gitea credentials")
		}
		q = q.Where("user_id = ?", userID)
	}

	var creds []GiteaCredentials
	err := q.Preload("Endpoint").Find(&creds).Error
	if err != nil {
		return nil, errors.Wrap(err, "fetching gitea credentials")
	}

	var ret []params.ForgeCredentials
	for _, c := range creds {
		commonCreds, err := s.sqlGiteaToCommonForgeCredentials(c)
		if err != nil {
			return nil, errors.Wrap(err, "converting gitea credentials")
		}
		ret = append(ret, commonCreds)
	}
	return ret, nil
}

func (s *sqlDatabase) UpdateGiteaCredentials(ctx context.Context, id uint, param params.UpdateGiteaCredentialsParams) (gtCreds params.ForgeCredentials, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.GiteaCredentialsEntityType, common.UpdateOperation, gtCreds)
		}
	}()
	var creds GiteaCredentials
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.Preload("Endpoint")
		if !auth.IsAdmin(ctx) {
			userID, err := getUIDFromContext(ctx)
			if err != nil {
				return errors.Wrap(err, "updating gitea credentials")
			}
			q = q.Where("user_id = ?", userID)
		}

		if err := q.Where("id = ?", id).First(&creds).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(runnerErrors.ErrNotFound, "gitea credentials not found")
			}
			return errors.Wrap(err, "fetching gitea credentials")
		}

		if param.Name != nil {
			creds.Name = *param.Name
		}
		if param.Description != nil {
			creds.Description = *param.Description
		}

		var data []byte
		var err error
		switch creds.AuthType {
		case params.ForgeAuthTypePAT:
			if param.PAT != nil {
				data, err = s.marshalAndSeal(param.PAT)
			}
		default:
			return errors.Wrap(runnerErrors.ErrBadRequest, "invalid auth type")
		}

		if err != nil {
			return errors.Wrap(err, "marshaling and sealing credentials")
		}
		if len(data) > 0 {
			creds.Payload = data
		}

		if err := tx.Save(&creds).Error; err != nil {
			return errors.Wrap(err, "updating gitea credentials")
		}
		return nil
	})
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "updating gitea credentials")
	}

	gtCreds, err = s.sqlGiteaToCommonForgeCredentials(creds)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "converting gitea credentials")
	}
	return gtCreds, nil
}

func (s *sqlDatabase) DeleteGiteaCredentials(ctx context.Context, id uint) (err error) {
	var creds GiteaCredentials
	defer func() {
		if err == nil {
			forgeCreds, innerErr := s.sqlGiteaToCommonForgeCredentials(creds)
			if innerErr != nil {
				slog.ErrorContext(ctx, "converting gitea credentials", "error", innerErr)
			}
			if creds.ID == 0 || creds.Name == "" {
				return
			}
			s.sendNotify(common.GiteaCredentialsEntityType, common.DeleteOperation, forgeCreds)
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.Where("id = ?", id).
			Preload("Repositories").
			Preload("Organizations")
		if !auth.IsAdmin(ctx) {
			userID, err := getUIDFromContext(ctx)
			if err != nil {
				return errors.Wrap(err, "deleting gitea credentials")
			}
			q = q.Where("user_id = ?", userID)
		}

		err := q.First(&creds).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return errors.Wrap(err, "fetching gitea credentials")
		}

		if len(creds.Repositories) > 0 {
			return errors.Wrap(runnerErrors.ErrBadRequest, "cannot delete credentials with repositories")
		}
		if len(creds.Organizations) > 0 {
			return errors.Wrap(runnerErrors.ErrBadRequest, "cannot delete credentials with organizations")
		}
		if err := tx.Unscoped().Delete(&creds).Error; err != nil {
			return errors.Wrap(err, "deleting gitea credentials")
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "deleting gitea credentials")
	}
	return nil
}
