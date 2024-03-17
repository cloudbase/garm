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
var _ poolHelper = &enterprise{}

func NewEnterprisePoolManager(ctx context.Context, cfg params.Enterprise, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ctx = util.WithContext(ctx, slog.Any("pool_mgr", cfg.Name), slog.Any("pool_type", params.GithubEntityTypeEnterprise))
	entity := params.GithubEntity{
		Owner:         cfg.Name,
		ID:            cfg.ID,
		WebhookSecret: cfg.WebhookSecret,
		EntityType:    params.GithubEntityTypeEnterprise,
	}
	ghc, err := util.GithubClient(ctx, entity, cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	wg := &sync.WaitGroup{}
	keyMuxes := &keyMutex{}

	helper := &enterprise{
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

type enterprise struct {
	ctx   context.Context
	id    string
	store dbCommon.Store
}

func (e *enterprise) FetchDbInstances() ([]params.Instance, error) {
	return e.store.ListEnterpriseInstances(e.ctx, e.id)
}

func (e *enterprise) ListPools() ([]params.Pool, error) {
	pools, err := e.store.ListEnterprisePools(e.ctx, e.id)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (e *enterprise) GetPoolByID(poolID string) (params.Pool, error) {
	pool, err := e.store.GetEnterprisePool(e.ctx, e.id, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}
