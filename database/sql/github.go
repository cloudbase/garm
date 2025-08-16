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
	"errors"
	"fmt"

	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
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
			return fmt.Errorf("error github endpoint already exists: %w", runnerErrors.ErrDuplicateEntity)
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
			return fmt.Errorf("error creating github endpoint: %w", err)
		}
		return nil
	})
	if err != nil {
		return params.ForgeEndpoint{}, fmt.Errorf("error creating github endpoint: %w", err)
	}
	ghEndpoint, err = s.sqlToCommonGithubEndpoint(endpoint)
	if err != nil {
		return params.ForgeEndpoint{}, fmt.Errorf("error converting github endpoint: %w", err)
	}
	return ghEndpoint, nil
}

func (s *sqlDatabase) ListGithubEndpoints(_ context.Context) ([]params.ForgeEndpoint, error) {
	var endpoints []GithubEndpoint
	err := s.conn.Where("endpoint_type = ?", params.GithubEndpointType).Find(&endpoints).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching github endpoints: %w", err)
	}

	var ret []params.ForgeEndpoint
	for _, ep := range endpoints {
		commonEp, err := s.sqlToCommonGithubEndpoint(ep)
		if err != nil {
			return nil, fmt.Errorf("error converting github endpoint: %w", err)
		}
		ret = append(ret, commonEp)
	}
	return ret, nil
}

func (s *sqlDatabase) UpdateGithubEndpoint(_ context.Context, name string, param params.UpdateGithubEndpointParams) (ghEndpoint params.ForgeEndpoint, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.UpdateOperation, ghEndpoint)
		}
	}()
	var endpoint GithubEndpoint
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("name = ? and endpoint_type = ?", name, params.GithubEndpointType).First(&endpoint).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error github endpoint not found: %w", runnerErrors.ErrNotFound)
			}
			return fmt.Errorf("error fetching github endpoint: %w", err)
		}

		var credsCount int64
		if err := tx.Model(&GithubCredentials{}).Where("endpoint_name = ?", endpoint.Name).Count(&credsCount).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching github credentials: %w", err)
			}
		}
		if credsCount > 0 && (param.APIBaseURL != nil || param.BaseURL != nil || param.UploadBaseURL != nil) {
			return fmt.Errorf("cannot update endpoint URLs with existing credentials: %w", runnerErrors.ErrBadRequest)
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
			return fmt.Errorf("error updating github endpoint: %w", err)
		}

		return nil
	})
	if err != nil {
		return params.ForgeEndpoint{}, fmt.Errorf("error updating github endpoint: %w", err)
	}
	ghEndpoint, err = s.sqlToCommonGithubEndpoint(endpoint)
	if err != nil {
		return params.ForgeEndpoint{}, fmt.Errorf("error converting github endpoint: %w", err)
	}
	return ghEndpoint, nil
}

func (s *sqlDatabase) GetGithubEndpoint(_ context.Context, name string) (params.ForgeEndpoint, error) {
	var endpoint GithubEndpoint

	err := s.conn.Where("name = ? and endpoint_type = ?", name, params.GithubEndpointType).First(&endpoint).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.ForgeEndpoint{}, fmt.Errorf("github endpoint not found: %w", runnerErrors.ErrNotFound)
		}
		return params.ForgeEndpoint{}, fmt.Errorf("error fetching github endpoint: %w", err)
	}

	return s.sqlToCommonGithubEndpoint(endpoint)
}

func (s *sqlDatabase) DeleteGithubEndpoint(_ context.Context, name string) (err error) {
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
			return fmt.Errorf("error fetching github endpoint: %w", err)
		}

		var credsCount int64
		if err := tx.Model(&GithubCredentials{}).Where("endpoint_name = ?", endpoint.Name).Count(&credsCount).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching github credentials: %w", err)
			}
		}

		var repoCnt int64
		if err := tx.Model(&Repository{}).Where("endpoint_name = ?", endpoint.Name).Count(&repoCnt).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching github repositories: %w", err)
			}
		}

		var orgCnt int64
		if err := tx.Model(&Organization{}).Where("endpoint_name = ?", endpoint.Name).Count(&orgCnt).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching github organizations: %w", err)
			}
		}

		var entCnt int64
		if err := tx.Model(&Enterprise{}).Where("endpoint_name = ?", endpoint.Name).Count(&entCnt).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching github enterprises: %w", err)
			}
		}

		if credsCount > 0 || repoCnt > 0 || orgCnt > 0 || entCnt > 0 {
			return fmt.Errorf("cannot delete endpoint with associated entities: %w", runnerErrors.ErrBadRequest)
		}

		if err := tx.Unscoped().Delete(&endpoint).Error; err != nil {
			return fmt.Errorf("error deleting github endpoint: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error deleting github endpoint: %w", err)
	}
	return nil
}

func (s *sqlDatabase) CreateGithubCredentials(ctx context.Context, param params.CreateGithubCredentialsParams) (ghCreds params.ForgeCredentials, err error) {
	userID, err := getUIDFromContext(ctx)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error creating github credentials: %w", err)
	}
	if param.Endpoint == "" {
		return params.ForgeCredentials{}, fmt.Errorf("endpoint name is required: %w", runnerErrors.ErrBadRequest)
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
				return fmt.Errorf("github endpoint not found: %w", runnerErrors.ErrNotFound)
			}
			return fmt.Errorf("error fetching github endpoint: %w", err)
		}

		if err := tx.Where("name = ? and user_id = ?", param.Name, userID).First(&creds).Error; err == nil {
			return fmt.Errorf("github credentials already exists: %w", runnerErrors.ErrDuplicateEntity)
		}

		var data []byte
		var err error
		switch param.AuthType {
		case params.ForgeAuthTypePAT:
			data, err = s.marshalAndSeal(param.PAT)
		case params.ForgeAuthTypeApp:
			data, err = s.marshalAndSeal(param.App)
		default:
			return fmt.Errorf("invalid auth type: %w", runnerErrors.ErrBadRequest)
		}
		if err != nil {
			return fmt.Errorf("error marshaling and sealing credentials: %w", err)
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
			return fmt.Errorf("error creating github credentials: %w", err)
		}
		// Skip making an extra query.
		creds.Endpoint = endpoint

		return nil
	})
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error creating github credentials: %w", err)
	}
	ghCreds, err = s.sqlToCommonForgeCredentials(creds)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error converting github credentials: %w", err)
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
		return GithubCredentials{}, fmt.Errorf("error fetching github credentials: %w", err)
	}
	q = q.Where("user_id = ?", userID)

	err = q.Where("name = ?", name).First(&creds).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return GithubCredentials{}, fmt.Errorf("github credentials not found: %w", runnerErrors.ErrNotFound)
		}
		return GithubCredentials{}, fmt.Errorf("error fetching github credentials: %w", err)
	}

	return creds, nil
}

func (s *sqlDatabase) GetGithubCredentialsByName(ctx context.Context, name string, detailed bool) (params.ForgeCredentials, error) {
	creds, err := s.getGithubCredentialsByName(ctx, s.conn, name, detailed)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error fetching github credentials: %w", err)
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
			return params.ForgeCredentials{}, fmt.Errorf("error fetching github credentials: %w", err)
		}
		q = q.Where("user_id = ?", userID)
	}

	err := q.Where("id = ?", id).First(&creds).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.ForgeCredentials{}, fmt.Errorf("github credentials not found: %w", runnerErrors.ErrNotFound)
		}
		return params.ForgeCredentials{}, fmt.Errorf("error fetching github credentials: %w", err)
	}

	return s.sqlToCommonForgeCredentials(creds)
}

func (s *sqlDatabase) ListGithubCredentials(ctx context.Context) ([]params.ForgeCredentials, error) {
	q := s.conn.Preload("Endpoint")
	if !auth.IsAdmin(ctx) {
		userID, err := getUIDFromContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("error fetching github credentials: %w", err)
		}
		q = q.Where("user_id = ?", userID)
	}

	var creds []GithubCredentials
	err := q.Preload("Endpoint").Find(&creds).Error
	if err != nil {
		return nil, fmt.Errorf("error fetching github credentials: %w", err)
	}

	var ret []params.ForgeCredentials
	for _, c := range creds {
		commonCreds, err := s.sqlToCommonForgeCredentials(c)
		if err != nil {
			return nil, fmt.Errorf("error converting github credentials: %w", err)
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
				return fmt.Errorf("error updating github credentials: %w", err)
			}
			q = q.Where("user_id = ?", userID)
		}

		if err := q.Where("id = ?", id).First(&creds).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("github credentials not found: %w", runnerErrors.ErrNotFound)
			}
			return fmt.Errorf("error fetching github credentials: %w", err)
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
				return fmt.Errorf("cannot update app credentials for PAT: %w", runnerErrors.ErrBadRequest)
			}
		case params.ForgeAuthTypeApp:
			if param.App != nil {
				data, err = s.marshalAndSeal(param.App)
			}

			if param.PAT != nil {
				return fmt.Errorf("cannot update PAT credentials for app: %w", runnerErrors.ErrBadRequest)
			}
		default:
			// This should never happen, unless there was a bug in the DB migration code,
			// or the DB was manually modified.
			return fmt.Errorf("invalid auth type: %w", runnerErrors.ErrBadRequest)
		}

		if err != nil {
			return fmt.Errorf("error marshaling and sealing credentials: %w", err)
		}
		if len(data) > 0 {
			creds.Payload = data
		}

		if err := tx.Save(&creds).Error; err != nil {
			return fmt.Errorf("error updating github credentials: %w", err)
		}
		return nil
	})
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error updating github credentials: %w", err)
	}

	ghCreds, err = s.sqlToCommonForgeCredentials(creds)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error converting github credentials: %w", err)
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
				return fmt.Errorf("error deleting github credentials: %w", err)
			}
			q = q.Where("user_id = ?", userID)
		}

		var creds GithubCredentials
		err := q.First(&creds).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("error fetching github credentials: %w", err)
		}
		name = creds.Name

		if len(creds.Repositories) > 0 {
			return fmt.Errorf("cannot delete credentials with repositories: %w", runnerErrors.ErrBadRequest)
		}
		if len(creds.Organizations) > 0 {
			return fmt.Errorf("cannot delete credentials with organizations: %w", runnerErrors.ErrBadRequest)
		}
		if len(creds.Enterprises) > 0 {
			return fmt.Errorf("cannot delete credentials with enterprises: %w", runnerErrors.ErrBadRequest)
		}

		if err := tx.Unscoped().Delete(&creds).Error; err != nil {
			return fmt.Errorf("error deleting github credentials: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error deleting github credentials: %w", err)
	}
	return nil
}
