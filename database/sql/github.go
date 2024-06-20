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

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

const (
	defaultGithubEndpoint string = "github.com"
)

func (s *sqlDatabase) sqlToCommonGithubCredentials(creds GithubCredentials) (params.GithubCredentials, error) {
	if len(creds.Payload) == 0 {
		return params.GithubCredentials{}, errors.New("empty credentials payload")
	}
	data, err := util.Unseal(creds.Payload, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "unsealing credentials")
	}

	ep, err := s.sqlToCommonGithubEndpoint(creds.Endpoint)
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "converting github endpoint")
	}

	commonCreds := params.GithubCredentials{
		ID:                 creds.ID,
		Name:               creds.Name,
		Description:        creds.Description,
		APIBaseURL:         creds.Endpoint.APIBaseURL,
		BaseURL:            creds.Endpoint.BaseURL,
		UploadBaseURL:      creds.Endpoint.UploadBaseURL,
		CABundle:           creds.Endpoint.CACertBundle,
		AuthType:           creds.AuthType,
		Endpoint:           ep,
		CredentialsPayload: data,
	}

	for _, repo := range creds.Repositories {
		commonRepo, err := s.sqlToCommonRepository(repo, false)
		if err != nil {
			return params.GithubCredentials{}, errors.Wrap(err, "converting github repository")
		}
		commonCreds.Repositories = append(commonCreds.Repositories, commonRepo)
	}

	for _, org := range creds.Organizations {
		commonOrg, err := s.sqlToCommonOrganization(org, false)
		if err != nil {
			return params.GithubCredentials{}, errors.Wrap(err, "converting github organization")
		}
		commonCreds.Organizations = append(commonCreds.Organizations, commonOrg)
	}

	for _, ent := range creds.Enterprises {
		commonEnt, err := s.sqlToCommonEnterprise(ent, false)
		if err != nil {
			return params.GithubCredentials{}, errors.Wrapf(err, "converting github enterprise: %s", ent.Name)
		}
		commonCreds.Enterprises = append(commonCreds.Enterprises, commonEnt)
	}

	return commonCreds, nil
}

func (s *sqlDatabase) sqlToCommonGithubEndpoint(ep GithubEndpoint) (params.GithubEndpoint, error) {
	return params.GithubEndpoint{
		Name:          ep.Name,
		Description:   ep.Description,
		APIBaseURL:    ep.APIBaseURL,
		BaseURL:       ep.BaseURL,
		UploadBaseURL: ep.UploadBaseURL,
		CACertBundle:  ep.CACertBundle,
	}, nil
}

func getUIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID := auth.UserID(ctx)
	if userID == "" {
		return uuid.Nil, errors.Wrap(runnerErrors.ErrUnauthorized, "getting UID from context")
	}

	asUUID, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, errors.Wrap(runnerErrors.ErrUnauthorized, "parsing UID from context")
	}
	return asUUID, nil
}

func (s *sqlDatabase) CreateGithubEndpoint(_ context.Context, param params.CreateGithubEndpointParams) (ghEndpoint params.GithubEndpoint, err error) {
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
		}

		if err := tx.Create(&endpoint).Error; err != nil {
			return errors.Wrap(err, "creating github endpoint")
		}
		return nil
	})
	if err != nil {
		return params.GithubEndpoint{}, errors.Wrap(err, "creating github endpoint")
	}
	ghEndpoint, err = s.sqlToCommonGithubEndpoint(endpoint)
	if err != nil {
		return params.GithubEndpoint{}, errors.Wrap(err, "converting github endpoint")
	}
	return ghEndpoint, nil
}

func (s *sqlDatabase) ListGithubEndpoints(_ context.Context) ([]params.GithubEndpoint, error) {
	var endpoints []GithubEndpoint
	err := s.conn.Find(&endpoints).Error
	if err != nil {
		return nil, errors.Wrap(err, "fetching github endpoints")
	}

	var ret []params.GithubEndpoint
	for _, ep := range endpoints {
		commonEp, err := s.sqlToCommonGithubEndpoint(ep)
		if err != nil {
			return nil, errors.Wrap(err, "converting github endpoint")
		}
		ret = append(ret, commonEp)
	}
	return ret, nil
}

func (s *sqlDatabase) UpdateGithubEndpoint(_ context.Context, name string, param params.UpdateGithubEndpointParams) (ghEndpoint params.GithubEndpoint, err error) {
	if name == defaultGithubEndpoint {
		return params.GithubEndpoint{}, errors.Wrap(runnerErrors.ErrBadRequest, "cannot update default github endpoint")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.UpdateOperation, ghEndpoint)
		}
	}()
	var endpoint GithubEndpoint
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("name = ?", name).First(&endpoint).Error; err != nil {
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
		return params.GithubEndpoint{}, errors.Wrap(err, "updating github endpoint")
	}
	ghEndpoint, err = s.sqlToCommonGithubEndpoint(endpoint)
	if err != nil {
		return params.GithubEndpoint{}, errors.Wrap(err, "converting github endpoint")
	}
	return ghEndpoint, nil
}

func (s *sqlDatabase) GetGithubEndpoint(_ context.Context, name string) (params.GithubEndpoint, error) {
	var endpoint GithubEndpoint

	err := s.conn.Where("name = ?", name).First(&endpoint).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.GithubEndpoint{}, errors.Wrap(runnerErrors.ErrNotFound, "github endpoint not found")
		}
		return params.GithubEndpoint{}, errors.Wrap(err, "fetching github endpoint")
	}

	return s.sqlToCommonGithubEndpoint(endpoint)
}

func (s *sqlDatabase) DeleteGithubEndpoint(_ context.Context, name string) (err error) {
	if name == defaultGithubEndpoint {
		return errors.Wrap(runnerErrors.ErrBadRequest, "cannot delete default github endpoint")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.GithubEndpointEntityType, common.DeleteOperation, params.GithubEndpoint{Name: name})
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var endpoint GithubEndpoint
		if err := tx.Where("name = ?", name).First(&endpoint).Error; err != nil {
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
			return errors.New("cannot delete endpoint with associated entities")
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

func (s *sqlDatabase) CreateGithubCredentials(ctx context.Context, param params.CreateGithubCredentialsParams) (ghCreds params.GithubCredentials, err error) {
	userID, err := getUIDFromContext(ctx)
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "creating github credentials")
	}
	if param.Endpoint == "" {
		return params.GithubCredentials{}, errors.Wrap(runnerErrors.ErrBadRequest, "endpoint name is required")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.GithubCredentialsEntityType, common.CreateOperation, ghCreds)
		}
	}()
	var creds GithubCredentials
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var endpoint GithubEndpoint
		if err := tx.Where("name = ?", param.Endpoint).First(&endpoint).Error; err != nil {
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
		case params.GithubAuthTypePAT:
			data, err = s.marshalAndSeal(param.PAT)
		case params.GithubAuthTypeApp:
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
		return params.GithubCredentials{}, errors.Wrap(err, "creating github credentials")
	}
	ghCreds, err = s.sqlToCommonGithubCredentials(creds)
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "converting github credentials")
	}
	return ghCreds, nil
}

func (s *sqlDatabase) getGithubCredentialsByName(ctx context.Context, tx *gorm.DB, name string, detailed bool) (GithubCredentials, error) {
	var creds GithubCredentials
	q := tx.Preload("Endpoint")

	if detailed {
		q = q.
			Preload("Repositories").
			Preload("Organizations").
			Preload("Enterprises")
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

func (s *sqlDatabase) GetGithubCredentialsByName(ctx context.Context, name string, detailed bool) (params.GithubCredentials, error) {
	creds, err := s.getGithubCredentialsByName(ctx, s.conn, name, detailed)
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "fetching github credentials")
	}

	return s.sqlToCommonGithubCredentials(creds)
}

func (s *sqlDatabase) GetGithubCredentials(ctx context.Context, id uint, detailed bool) (params.GithubCredentials, error) {
	var creds GithubCredentials
	q := s.conn.Preload("Endpoint")

	if detailed {
		q = q.
			Preload("Repositories").
			Preload("Organizations").
			Preload("Enterprises")
	}

	if !auth.IsAdmin(ctx) {
		userID, err := getUIDFromContext(ctx)
		if err != nil {
			return params.GithubCredentials{}, errors.Wrap(err, "fetching github credentials")
		}
		q = q.Where("user_id = ?", userID)
	}

	err := q.Where("id = ?", id).First(&creds).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return params.GithubCredentials{}, errors.Wrap(runnerErrors.ErrNotFound, "github credentials not found")
		}
		return params.GithubCredentials{}, errors.Wrap(err, "fetching github credentials")
	}

	return s.sqlToCommonGithubCredentials(creds)
}

func (s *sqlDatabase) ListGithubCredentials(ctx context.Context) ([]params.GithubCredentials, error) {
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

	var ret []params.GithubCredentials
	for _, c := range creds {
		commonCreds, err := s.sqlToCommonGithubCredentials(c)
		if err != nil {
			return nil, errors.Wrap(err, "converting github credentials")
		}
		ret = append(ret, commonCreds)
	}
	return ret, nil
}

func (s *sqlDatabase) UpdateGithubCredentials(ctx context.Context, id uint, param params.UpdateGithubCredentialsParams) (ghCreds params.GithubCredentials, err error) {
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
		case params.GithubAuthTypePAT:
			if param.PAT != nil {
				data, err = s.marshalAndSeal(param.PAT)
			}

			if param.App != nil {
				return errors.Wrap(runnerErrors.ErrBadRequest, "cannot update app credentials for PAT")
			}
		case params.GithubAuthTypeApp:
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
		return params.GithubCredentials{}, errors.Wrap(err, "updating github credentials")
	}

	ghCreds, err = s.sqlToCommonGithubCredentials(creds)
	if err != nil {
		return params.GithubCredentials{}, errors.Wrap(err, "converting github credentials")
	}
	return ghCreds, nil
}

func (s *sqlDatabase) DeleteGithubCredentials(ctx context.Context, id uint) (err error) {
	var name string
	defer func() {
		if err == nil {
			s.sendNotify(common.GithubCredentialsEntityType, common.DeleteOperation, params.GithubCredentials{ID: id, Name: name})
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
