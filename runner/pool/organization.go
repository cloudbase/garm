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

package pool

import (
	"context"
	"log/slog"
	"sync"

	"github.com/pkg/errors"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util"
)

// test that we implement PoolManager
var _ poolHelper = &organization{}

func NewOrganizationPoolManager(ctx context.Context, cfg params.Organization, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ctx = util.WithContext(ctx, slog.Any("pool_mgr", cfg.Name), slog.Any("pool_type", params.GithubEntityTypeOrganization))
	entity := params.GithubEntity{
		Owner:         cfg.Name,
		ID:            cfg.ID,
		WebhookSecret: cfg.WebhookSecret,
		EntityType:    params.GithubEntityTypeOrganization,
	}
	ghc, err := util.GithubClient(ctx, entity, cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	wg := &sync.WaitGroup{}
	keyMuxes := &keyMutex{}

	helper := &organization{
		ctx:   ctx,
		id:    cfg.ID,
		store: store,
	}

	repo := &basePoolManager{
		ctx:          ctx,
		cfgInternal:  cfgInternal,
		ghcli:        ghc,
		entity:       entity,
		store:        store,
		providers:    providers,
		controllerID: cfgInternal.ControllerID,
		urls: urls{
			webhookURL:           cfgInternal.BaseWebhookURL,
			callbackURL:          cfgInternal.InstanceCallbackURL,
			metadataURL:          cfgInternal.InstanceMetadataURL,
			controllerWebhookURL: cfgInternal.ControllerWebhookURL,
		},
		quit:         make(chan struct{}),
		helper:       helper,
		credsDetails: cfgInternal.GithubCredentialsDetails,
		wg:           wg,
		keyMux:       keyMuxes,
	}
	return repo, nil
}

type organization struct {
	ctx   context.Context
	id    string
	store dbCommon.Store
}

func (o *organization) FetchDbInstances() ([]params.Instance, error) {
	return o.store.ListOrgInstances(o.ctx, o.id)
}

func (o *organization) ListPools() ([]params.Pool, error) {
	pools, err := o.store.ListOrgPools(o.ctx, o.id)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (o *organization) GetPoolByID(poolID string) (params.Pool, error) {
	pool, err := o.store.GetOrganizationPool(o.ctx, o.id, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}
