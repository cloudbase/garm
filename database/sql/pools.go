package sql

import (
	"context"

	"garm/params"

	"github.com/pkg/errors"
)

func (s *sqlDatabase) ListAllPools(ctx context.Context) ([]params.Pool, error) {
	var pools []Pool

	q := s.conn.Model(&Pool{}).
		Preload("Tags").
		Preload("Organization").
		Preload("Repository").
		Find(&pools)
	if q.Error != nil {
		return nil, errors.Wrap(q.Error, "fetching all pools")
	}

	ret := make([]params.Pool, len(pools))
	for idx, val := range pools {
		ret[idx] = s.sqlToCommonPool(val)
	}
	return ret, nil
}

func (s *sqlDatabase) GetPoolByID(ctx context.Context, poolID string) (params.Pool, error) {
	pool, err := s.getPoolByID(ctx, poolID, "Tags", "Instances", "Organization", "Repository")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool by ID")
	}
	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) DeletePoolByID(ctx context.Context, poolID string) error {
	pool, err := s.getPoolByID(ctx, poolID)
	if err != nil {
		return errors.Wrap(err, "fetching pool by ID")
	}

	if q := s.conn.Unscoped().Delete(&pool); q.Error != nil {
		return errors.Wrap(q.Error, "removing pool")
	}

	return nil
}
