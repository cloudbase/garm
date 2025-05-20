// Copyright 2024 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.

package sql

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

const (
	defaultGithubEndpoint string = "github.com"
)

func (s *sqlDatabase) CreateGithubEndpoint(_ context.Context, param params.CreateGithubEndpointParams) (ghEndpoint params.ForgeEndpoint, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.CreateOperation, ghEndpoint)
		}
	}()
	var endpoint GithubEndpoint
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("name = ?", param.Name).First(&endpoint).Error; err == nil {
			return errors.Wrap(runnerErrors.ErrDuplicateEntity, "github endpoint already exists")
		}
		endpoint = GithubEndpoint{
			Name:          param.Name,
			Description:   param.Description,
			APIBaseURL:    param.APIBaseURL,
			BaseURL:       param.BaseURL,
			UploadBaseURL: param.UploadBaseURL,
			CACertBundle:  param.CACertBundle,
			EndpointType:  params.GithubEndpointType,
		}

		if err := tx.Create(&endpoint).Error; err != nil {
			return errors.Wrap(err, "creating github endpoint")
		}
		return nil
	})
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "creating github endpoint")
	}
	ghEndpoint, err = s.sqlToCommonGithubEndpoint(endpoint)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "converting github endpoint")
	}
	return ghEndpoint, nil
}

func (s *sqlDatabase) ListGithubEndpoints(_ context.Context) ([]params.ForgeEndpoint, error) {
	var endpoints []GithubEndpoint
	err := s.conn.Where("endpoint_type = ?", params.GithubEndpointType).Find(&endpoints).Error
	if err != nil {
		return nil, errors.Wrap(err, "fetching github endpoints")
	}

	var ret []params.ForgeEndpoint
	for _, ep := range endpoints {
		commonEp, err := s.sqlToCommonGithubEndpoint(ep)
		if err != nil {
			return nil, errors.Wrap(err, "converting github endpoint")
		}
		ret = append(ret, commonEp)
	}
	return ret, nil
}

func (s *sqlDatabase) UpdateGithubEndpoint(_ context.Context, name string, param params.UpdateGithubEndpointParams) (ghEndpoint params.ForgeEndpoint, err error) {
	if name == defaultGithubEndpoint {
		return params.ForgeEndpoint{}, errors.Wrap(runnerErrors.ErrBadRequest, "cannot update default github endpoint")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.UpdateOperation, ghEndpoint)
		}
	}()
	var endpoint GithubEndpoint
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("name = ? and endpoint_type = ?", name, params.GithubEndpointType).First(&endpoint).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(runnerErrors.ErrNotFound, "github endpoint not found")
			}
			return errors.Wrap(err, "fetching github endpoint")
		}
		if param.APIBaseURL != nil {
			endpoint.APIBaseURL = *param.APIBaseURL
		}

		if param.BaseURL != nil {
			endpoint.BaseURL = *param.BaseURL
		}

		if param.UploadBaseURL != nil {
			endpoint.UploadBaseURL = *param.UploadBaseURL
		}

		if param.CACertBundle != nil {
			endpoint.CACertBundle = param.CACertBundle
		}

		if param.Description != nil {
			endpoint.Description = *param.Description
		}

		if err := tx.Save(&endpoint).Error; err != nil {
			return errors.Wrap(err, "updating github endpoint")
		}

		return nil
	})
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "updating github endpoint")
	}
	ghEndpoint, err = s.sqlToCommonGithubEndpoint(endpoint)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "converting github endpoint")
	}
	return ghEndpoint, nil
}

func (s *sqlDatabase) GetGithubEndpoint(_ context.Context, name string) (params.ForgeEndpoint, error) {
	var endpoint GithubEndpoint

	err := s.conn.Where("name = ? and endpoint_type = ?", name, params.GithubEndpointType).First(&endpoint).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.ForgeEndpoint{}, errors.Wrap(runnerErrors.ErrNotFound, "github endpoint not found")
		}
		return params.ForgeEndpoint{}, errors.Wrap(err, "fetching github endpoint")
	}

	return s.sqlToCommonGithubEndpoint(endpoint)
}

func (s *sqlDatabase) DeleteGithubEndpoint(_ context.Context, name string) (err error) {
	if name == defaultGithubEndpoint {
		return errors.Wrap(runnerErrors.ErrBadRequest, "cannot delete default github endpoint")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.DeleteOperation, params.ForgeEndpoint{Name: name})
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var endpoint GithubEndpoint
		if err := tx.Where("name = ? and endpoint_type = ?", name, params.GithubEndpointType).First(&endpoint).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return errors.Wrap(err, "fetching github endpoint")
		}

		var credsCount int64
		if err := tx.Model(&GithubCredentials{}).Where("endpoint_name = ?", endpoint.Name).Count(&credsCount).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "fetching github credentials")
			}
		}

		var repoCnt int64
		if err := tx.Model(&Repository{}).Where("endpoint_name = ?", endpoint.Name).Count(&repoCnt).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "fetching github repositories")
			}
		}

		var orgCnt int64
		if err := tx.Model(&Organization{}).Where("endpoint_name = ?", endpoint.Name).Count(&orgCnt).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "fetching github organizations")
			}
		}

		var entCnt int64
		if err := tx.Model(&Enterprise{}).Where("endpoint_name = ?", endpoint.Name).Count(&entCnt).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(err, "fetching github enterprises")
			}
		}

		if credsCount > 0 || repoCnt > 0 || orgCnt > 0 || entCnt > 0 {
			return runnerErrors.NewBadRequestError("cannot delete endpoint with associated entities")
		}

		if err := tx.Unscoped().Delete(&endpoint).Error; err != nil {
			return errors.Wrap(err, "deleting github endpoint")
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "deleting github endpoint")
	}
	return nil
}

func (s *sqlDatabase) CreateGithubCredentials(ctx context.Context, param params.CreateGithubCredentialsParams) (ghCreds params.ForgeCredentials, err error) {
	userID, err := getUIDFromContext(ctx)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "creating github credentials")
	}
	if param.Endpoint == "" {
		return params.ForgeCredentials{}, errors.Wrap(runnerErrors.ErrBadRequest, "endpoint name is required")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.GithubCredentialsEntityType, common.CreateOperation, ghCreds)
		}
	}()
	var creds GithubCredentials
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var endpoint GithubEndpoint
		if err := tx.Where("name = ? and endpoint_type = ?", param.Endpoint, params.GithubEndpointType).First(&endpoint).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(runnerErrors.ErrNotFound, "github endpoint not found")
			}
			return errors.Wrap(err, "fetching github endpoint")
		}

		if err := tx.Where("name = ? and user_id = ?", param.Name, userID).First(&creds).Error; err == nil {
			return errors.Wrap(runnerErrors.ErrDuplicateEntity, "github credentials already exists")
		}

		var data []byte
		var err error
		switch param.AuthType {
		case params.ForgeAuthTypePAT:
			data, err = s.marshalAndSeal(param.PAT)
		case params.ForgeAuthTypeApp:
			data, err = s.marshalAndSeal(param.App)
		default:
			return errors.Wrap(runnerErrors.ErrBadRequest, "invalid auth type")
		}
		if err != nil {
			return errors.Wrap(err, "marshaling and sealing credentials")
		}

		creds = GithubCredentials{
			Name:         param.Name,
			Description:  param.Description,
			EndpointName: &endpoint.Name,
			AuthType:     param.AuthType,
			Payload:      data,
			UserID:       &userID,
		}

		if err := tx.Create(&creds).Error; err != nil {
			return errors.Wrap(err, "creating github credentials")
		}
		// Skip making an extra query.
		creds.Endpoint = endpoint

		return nil
	})
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "creating github credentials")
	}
	ghCreds, err = s.sqlToCommonForgeCredentials(creds)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "converting github credentials")
	}
	return ghCreds, nil
}

func (s *sqlDatabase) getGithubCredentialsByName(ctx context.Context, tx *gorm.DB, name string, detailed bool) (GithubCredentials, error) {
	var creds GithubCredentials
	q := tx.Preload("Endpoint")

	if detailed {
		q = q.
			Preload("Repositories").
			Preload("Repositories.Credentials").
			Preload("Organizations").
			Preload("Organizations.Credentials").
			Preload("Enterprises").
			Preload("Enterprises.Credentials")
	}

	userID, err := getUIDFromContext(ctx)
	if err != nil {
		return GithubCredentials{}, errors.Wrap(err, "fetching github credentials")
	}
	q = q.Where("user_id = ?", userID)

	err = q.Where("name = ?", name).First(&creds).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return GithubCredentials{}, errors.Wrap(runnerErrors.ErrNotFound, "github credentials not found")
		}
		return GithubCredentials{}, errors.Wrap(err, "fetching github credentials")
	}

	return creds, nil
}

func (s *sqlDatabase) GetGithubCredentialsByName(ctx context.Context, name string, detailed bool) (params.ForgeCredentials, error) {
	creds, err := s.getGithubCredentialsByName(ctx, s.conn, name, detailed)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "fetching github credentials")
	}
	return s.sqlToCommonForgeCredentials(creds)
}

func (s *sqlDatabase) GetGithubCredentials(ctx context.Context, id uint, detailed bool) (params.ForgeCredentials, error) {
	var creds GithubCredentials
	q := s.conn.Preload("Endpoint")

	if detailed {
		q = q.
			Preload("Repositories").
			Preload("Repositories.Credentials").
			Preload("Organizations").
			Preload("Organizations.Credentials").
			Preload("Enterprises").
			Preload("Enterprises.Credentials")
	}

	if !auth.IsAdmin(ctx) {
		userID, err := getUIDFromContext(ctx)
		if err != nil {
			return params.ForgeCredentials{}, errors.Wrap(err, "fetching github credentials")
		}
		q = q.Where("user_id = ?", userID)
	}

	err := q.Where("id = ?", id).First(&creds).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.ForgeCredentials{}, errors.Wrap(runnerErrors.ErrNotFound, "github credentials not found")
		}
		return params.ForgeCredentials{}, errors.Wrap(err, "fetching github credentials")
	}

	return s.sqlToCommonForgeCredentials(creds)
}

func (s *sqlDatabase) ListGithubCredentials(ctx context.Context) ([]params.ForgeCredentials, error) {
	q := s.conn.Preload("Endpoint")
	if !auth.IsAdmin(ctx) {
		userID, err := getUIDFromContext(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "fetching github credentials")
		}
		q = q.Where("user_id = ?", userID)
	}

	var creds []GithubCredentials
	err := q.Preload("Endpoint").Find(&creds).Error
	if err != nil {
		return nil, errors.Wrap(err, "fetching github credentials")
	}

	var ret []params.ForgeCredentials
	for _, c := range creds {
		commonCreds, err := s.sqlToCommonForgeCredentials(c)
		if err != nil {
			return nil, errors.Wrap(err, "converting github credentials")
		}
		ret = append(ret, commonCreds)
	}
	return ret, nil
}

func (s *sqlDatabase) UpdateGithubCredentials(ctx context.Context, id uint, param params.UpdateGithubCredentialsParams) (ghCreds params.ForgeCredentials, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.GithubCredentialsEntityType, common.UpdateOperation, ghCreds)
		}
	}()
	var creds GithubCredentials
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.Preload("Endpoint")
		if !auth.IsAdmin(ctx) {
			userID, err := getUIDFromContext(ctx)
			if err != nil {
				return errors.Wrap(err, "updating github credentials")
			}
			q = q.Where("user_id = ?", userID)
		}

		if err := q.Where("id = ?", id).First(&creds).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.Wrap(runnerErrors.ErrNotFound, "github credentials not found")
			}
			return errors.Wrap(err, "fetching github credentials")
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

			if param.App != nil {
				return errors.Wrap(runnerErrors.ErrBadRequest, "cannot update app credentials for PAT")
			}
		case params.ForgeAuthTypeApp:
			if param.App != nil {
				data, err = s.marshalAndSeal(param.App)
			}

			if param.PAT != nil {
				return errors.Wrap(runnerErrors.ErrBadRequest, "cannot update PAT credentials for app")
			}
		default:
			// This should never happen, unless there was a bug in the DB migration code,
			// or the DB was manually modified.
			return errors.Wrap(runnerErrors.ErrBadRequest, "invalid auth type")
		}

		if err != nil {
			return errors.Wrap(err, "marshaling and sealing credentials")
		}
		if len(data) > 0 {
			creds.Payload = data
		}

		if err := tx.Save(&creds).Error; err != nil {
			return errors.Wrap(err, "updating github credentials")
		}
		return nil
	})
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "updating github credentials")
	}

	ghCreds, err = s.sqlToCommonForgeCredentials(creds)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "converting github credentials")
	}
	return ghCreds, nil
}

func (s *sqlDatabase) DeleteGithubCredentials(ctx context.Context, id uint) (err error) {
	var name string
	defer func() {
		if err == nil {
			s.sendNotify(common.GithubCredentialsEntityType, common.DeleteOperation, params.ForgeCredentials{ID: id, Name: name})
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.Where("id = ?", id).
			Preload("Repositories").
			Preload("Organizations").
			Preload("Enterprises")
		if !auth.IsAdmin(ctx) {
			userID, err := getUIDFromContext(ctx)
			if err != nil {
				return errors.Wrap(err, "deleting github credentials")
			}
			q = q.Where("user_id = ?", userID)
		}

		var creds GithubCredentials
		err := q.First(&creds).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return errors.Wrap(err, "fetching github credentials")
		}
		name = creds.Name

		if len(creds.Repositories) > 0 {
			return errors.Wrap(runnerErrors.ErrBadRequest, "cannot delete credentials with repositories")
		}
		if len(creds.Organizations) > 0 {
			return errors.Wrap(runnerErrors.ErrBadRequest, "cannot delete credentials with organizations")
		}
		if len(creds.Enterprises) > 0 {
			return errors.Wrap(runnerErrors.ErrBadRequest, "cannot delete credentials with enterprises")
		}

		if err := tx.Unscoped().Delete(&creds).Error; err != nil {
			return errors.Wrap(err, "deleting github credentials")
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "deleting github credentials")
	}
	return nil
}
