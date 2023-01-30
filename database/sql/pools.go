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

	"garm/params"

	"github.com/pkg/errors"
)

func (s *sqlDatabase) ListAllPools(ctx context.Context) ([]params.Pool, error) {
	var pools []Pool

	q := s.conn.Model(&Pool{}).
		Preload("Tags").
		Preload("Organization").
		Preload("Repository").
		Preload("Enterprise").
		Omit("extra_specs").
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
	pool, err := s.getPoolByID(ctx, poolID, "Tags", "Instances", "Enterprise", "Organization", "Repository")
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
