// Copyright 2026 Cloudbase Solutions SRL
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
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

// proxyCredentials is the shape of the sealed credentials blob stored in
// the proxies table.
type proxyCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *sqlDatabase) sqlToParamProxy(proxy Proxy) (params.Proxy, error) {
	ret := params.Proxy{
		ID:          proxy.ID,
		CreatedAt:   proxy.CreatedAt,
		UpdatedAt:   proxy.UpdatedAt,
		Name:        proxy.Name,
		Description: proxy.Description,
		HTTPProxy:   proxy.HTTPProxy,
		HTTPSProxy:  proxy.HTTPSProxy,
		NoProxy:     proxy.NoProxy,
	}

	if len(proxy.Credentials) > 0 {
		var creds proxyCredentials
		if err := s.unsealAndUnmarshal(proxy.Credentials, &creds); err != nil {
			return params.Proxy{}, fmt.Errorf("failed to unseal proxy credentials: %w", err)
		}
		ret.Username = creds.Username
		ret.Password = creds.Password
	}

	return ret, nil
}

// sealProxyCredentials seals the given username and password. If the
// username is empty, credentials are considered unset and nil is returned.
func (s *sqlDatabase) sealProxyCredentials(username, password string) ([]byte, error) {
	if username == "" {
		return nil, nil
	}
	sealed, err := s.marshalAndSeal(proxyCredentials{
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to seal proxy credentials: %w", err)
	}
	return sealed, nil
}

// hasProxy checks that a proxy with the given ID exists. It is used when
// pools or scale sets are created or updated with a proxy reference.
func (s *sqlDatabase) hasProxy(tx *gorm.DB, id uint) error {
	var count int64
	if err := tx.Model(&Proxy{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("error checking proxy existence: %w", err)
	}
	if count == 0 {
		return runnerErrors.NewBadRequestError("proxy with ID %d does not exist", id)
	}
	return nil
}

func (s *sqlDatabase) ListProxies(_ context.Context) ([]params.Proxy, error) {
	var proxies []Proxy
	q := s.conn.Model(&Proxy{}).Find(&proxies)
	if q.Error != nil {
		return nil, fmt.Errorf("failed to list proxies: %w", q.Error)
	}

	ret := make([]params.Proxy, len(proxies))
	for idx, proxy := range proxies {
		converted, err := s.sqlToParamProxy(proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to convert proxy: %w", err)
		}
		ret[idx] = converted
	}
	return ret, nil
}

func (s *sqlDatabase) getProxy(tx *gorm.DB, id uint, preload ...string) (Proxy, error) {
	var proxy Proxy
	q := tx.Model(&Proxy{}).Where("id = ?", id)
	for _, item := range preload {
		q = q.Preload(item)
	}

	q = q.First(&proxy)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Proxy{}, runnerErrors.ErrNotFound
		}
		return Proxy{}, fmt.Errorf("failed to get proxy: %w", q.Error)
	}
	return proxy, nil
}

func (s *sqlDatabase) GetProxy(_ context.Context, id uint) (params.Proxy, error) {
	proxy, err := s.getProxy(s.conn, id)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to get proxy: %w", err)
	}

	ret, err := s.sqlToParamProxy(proxy)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to convert proxy: %w", err)
	}
	return ret, nil
}

func (s *sqlDatabase) GetProxyByName(_ context.Context, name string) (params.Proxy, error) {
	var proxy Proxy
	q := s.conn.Model(&Proxy{}).Where("LOWER(name) = LOWER(?)", name).First(&proxy)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.Proxy{}, runnerErrors.ErrNotFound
		}
		return params.Proxy{}, fmt.Errorf("failed to get proxy: %w", q.Error)
	}

	ret, err := s.sqlToParamProxy(proxy)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to convert proxy: %w", err)
	}
	return ret, nil
}

func (s *sqlDatabase) CreateProxy(_ context.Context, param params.CreateProxyParams) (proxy params.Proxy, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.ProxyEntityType, common.CreateOperation, proxy)
		}
	}()
	if err := param.Validate(); err != nil {
		return params.Proxy{}, fmt.Errorf("failed to validate create params: %w", err)
	}

	sealed, err := s.sealProxyCredentials(param.Username, param.Password)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("error sealing proxy credentials: %w", err)
	}

	newProxy := Proxy{
		Name:        param.Name,
		Description: param.Description,
		HTTPProxy:   param.HTTPProxy,
		HTTPSProxy:  param.HTTPSProxy,
		NoProxy:     param.NoProxy,
		Credentials: sealed,
	}

	if err := s.conn.Create(&newProxy).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return params.Proxy{}, runnerErrors.NewConflictError("a proxy already exists with the specified name")
		}
		return params.Proxy{}, fmt.Errorf("error creating proxy: %w", err)
	}

	proxy, err = s.sqlToParamProxy(newProxy)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to convert proxy: %w", err)
	}
	return proxy, nil
}

// updateProxyCredentials applies the username/password changes from the
// update params to the sealed credentials blob of the given proxy. Clearing
// the username clears the credentials entirely.
func (s *sqlDatabase) updateProxyCredentials(dbProxy *Proxy, param params.UpdateProxyParams) (bool, error) {
	if param.Username == nil && param.Password == nil {
		return false, nil
	}

	var creds proxyCredentials
	if len(dbProxy.Credentials) > 0 {
		if err := s.unsealAndUnmarshal(dbProxy.Credentials, &creds); err != nil {
			return false, fmt.Errorf("failed to unseal proxy credentials: %w", err)
		}
	}

	if param.Username != nil {
		creds.Username = *param.Username
	}
	if param.Password != nil {
		creds.Password = *param.Password
	}

	if creds.Username == "" && creds.Password != "" {
		return false, runnerErrors.NewBadRequestError("password cannot be set without a username")
	}

	sealed, err := s.sealProxyCredentials(creds.Username, creds.Password)
	if err != nil {
		return false, fmt.Errorf("error sealing proxy credentials: %w", err)
	}
	dbProxy.Credentials = sealed
	return true, nil
}

func (s *sqlDatabase) UpdateProxy(_ context.Context, id uint, param params.UpdateProxyParams) (proxy params.Proxy, err error) {
	var hasChange bool
	defer func() {
		if err == nil && hasChange {
			s.sendNotify(common.ProxyEntityType, common.UpdateOperation, proxy)
		}
	}()
	if err := param.Validate(); err != nil {
		return params.Proxy{}, fmt.Errorf("failed to validate update params: %w", err)
	}

	err = s.conn.Transaction(func(tx *gorm.DB) error {
		dbProxy, err := s.getProxy(tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
		if err != nil {
			return fmt.Errorf("failed to get proxy: %w", err)
		}

		if param.Name != nil && *param.Name != dbProxy.Name {
			hasChange = true
			dbProxy.Name = *param.Name
		}

		if param.Description != nil && *param.Description != dbProxy.Description {
			hasChange = true
			dbProxy.Description = *param.Description
		}

		if param.HTTPProxy != nil && *param.HTTPProxy != dbProxy.HTTPProxy {
			hasChange = true
			dbProxy.HTTPProxy = *param.HTTPProxy
		}

		if param.HTTPSProxy != nil && *param.HTTPSProxy != dbProxy.HTTPSProxy {
			hasChange = true
			dbProxy.HTTPSProxy = *param.HTTPSProxy
		}

		if dbProxy.HTTPProxy == "" && dbProxy.HTTPSProxy == "" {
			return runnerErrors.NewBadRequestError("at least one of http_proxy or https_proxy must be set")
		}

		if param.NoProxy != nil && *param.NoProxy != dbProxy.NoProxy {
			hasChange = true
			dbProxy.NoProxy = *param.NoProxy
		}

		credsChanged, err := s.updateProxyCredentials(&dbProxy, param)
		if err != nil {
			return fmt.Errorf("error updating proxy credentials: %w", err)
		}
		if credsChanged {
			hasChange = true
		}

		if !hasChange {
			proxy, err = s.sqlToParamProxy(dbProxy)
			if err != nil {
				return fmt.Errorf("failed to convert proxy: %w", err)
			}
			return nil
		}

		if q := tx.Save(&dbProxy); q.Error != nil {
			if errors.Is(q.Error, gorm.ErrDuplicatedKey) {
				return runnerErrors.NewConflictError("a proxy already exists with the specified name")
			}
			return fmt.Errorf("failed to save proxy: %w", q.Error)
		}

		proxy, err = s.sqlToParamProxy(dbProxy)
		if err != nil {
			return fmt.Errorf("failed to convert proxy: %w", err)
		}
		return nil
	})
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to update proxy: %w", err)
	}
	return proxy, nil
}

func (s *sqlDatabase) DeleteProxy(_ context.Context, id uint) (err error) {
	var proxy params.Proxy

	defer func() {
		if err == nil {
			s.sendNotify(common.ProxyEntityType, common.DeleteOperation, proxy)
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		dbProxy, err := s.getProxy(tx.Clauses(clause.Locking{Strength: "UPDATE"}), id, "Pools", "ScaleSets")
		if err != nil {
			return fmt.Errorf("failed to get proxy: %w", err)
		}

		if len(dbProxy.Pools) > 0 || len(dbProxy.ScaleSets) > 0 {
			return runnerErrors.NewBadRequestError("cannot delete proxy while in use by pools or scale sets")
		}

		proxy = params.Proxy{
			ID:   dbProxy.ID,
			Name: dbProxy.Name,
		}

		if q := tx.Unscoped().Delete(&dbProxy); q.Error != nil {
			return fmt.Errorf("failed to delete proxy: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to delete proxy: %w", err)
	}
	return nil
}
