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
	"fmt"
	"log/slog"
	"sync"

	"github.com/pkg/errors"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util"
)

// test that we implement PoolManager
var _ poolHelper = &repository{}

func NewRepositoryPoolManager(ctx context.Context, cfg params.Repository, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ctx = util.WithContext(ctx, slog.Any("pool_mgr", fmt.Sprintf("%s/%s", cfg.Owner, cfg.Name)), slog.Any("pool_type", params.GithubEntityTypeRepository))
	entity := params.GithubEntity{
		Name:          cfg.Name,
		Owner:         cfg.Owner,
		ID:            cfg.ID,
		WebhookSecret: cfg.WebhookSecret,
		EntityType:    params.GithubEntityTypeRepository,
	}
	ghc, err := util.GithubClient(ctx, entity, cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	wg := &sync.WaitGroup{}
	keyMuxes := &keyMutex{}

	helper := &repository{
		ctx:   ctx,
		id:    cfg.ID,
		store: store,
	}

	repo := &basePoolManager{
		ctx:         ctx,
		cfgInternal: cfgInternal,
		entity:      entity,
		ghcli:       ghc,

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

var _ poolHelper = &repository{}

type repository struct {
	ctx   context.Context
	id    string
	store dbCommon.Store
}

func (r *repository) FetchDbInstances() ([]params.Instance, error) {
	return r.store.ListRepoInstances(r.ctx, r.id)
}

func (r *repository) ListPools() ([]params.Pool, error) {
	pools, err := r.store.ListRepoPools(r.ctx, r.id)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *repository) GetPoolByID(poolID string) (params.Pool, error) {
	pool, err := r.store.GetRepositoryPool(r.ctx, r.id, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}
