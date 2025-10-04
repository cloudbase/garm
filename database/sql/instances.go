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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateInstance(ctx context.Context, poolID string, param params.CreateInstanceParams) (instance params.Instance, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.InstanceEntityType, common.CreateOperation, instance)
		}
	}()

	err = s.conn.Transaction(func(tx *gorm.DB) error {
		pool, err := s.getPoolByID(tx, poolID)
		if err != nil {
			return fmt.Errorf("error fetching pool: %w", err)
		}
		var cnt int64
		q := s.conn.Model(&Instance{}).Where("pool_id = ?", pool.ID).Count(&cnt)
		if q.Error != nil {
			return fmt.Errorf("error fetching instance count: %w", q.Error)
		}
		var maxRunners int64
		if pool.MaxRunners > math.MaxInt64 {
			maxRunners = math.MaxInt64
		} else {
			maxRunners = int64(pool.MaxRunners)
		}
		if cnt >= maxRunners {
			return runnerErrors.NewConflictError("max runners reached for pool %s", pool.ID)
		}

		var labels datatypes.JSON
		if len(param.AditionalLabels) > 0 {
			labels, err = json.Marshal(param.AditionalLabels)
			if err != nil {
				return fmt.Errorf("error marshalling labels: %w", err)
			}
		}

		var secret []byte
		if len(param.JitConfiguration) > 0 {
			secret, err = s.marshalAndSeal(param.JitConfiguration)
			if err != nil {
				return fmt.Errorf("error marshalling jit config: %w", err)
			}
		}

		newInstance := Instance{
			Pool:              pool,
			Name:              param.Name,
			Status:            param.Status,
			RunnerStatus:      param.RunnerStatus,
			OSType:            param.OSType,
			OSArch:            param.OSArch,
			CallbackURL:       param.CallbackURL,
			MetadataURL:       param.MetadataURL,
			GitHubRunnerGroup: param.GitHubRunnerGroup,
			JitConfiguration:  secret,
			AditionalLabels:   labels,
			AgentID:           param.AgentID,
		}
		q = tx.Create(&newInstance)
		if q.Error != nil {
			return fmt.Errorf("error creating instance: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return params.Instance{}, fmt.Errorf("error creating instance: %w", err)
	}

	return s.GetInstance(ctx, param.Name)
}

func (s *sqlDatabase) getPoolInstanceByName(poolID string, instanceName string) (Instance, error) {
	pool, err := s.getPoolByID(s.conn, poolID)
	if err != nil {
		return Instance{}, fmt.Errorf("error fetching pool: %w", err)
	}

	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("name = ? and pool_id = ?", instanceName, pool.ID).
		First(&instance)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Instance{}, fmt.Errorf("error fetching pool instance by name: %w", runnerErrors.ErrNotFound)
		}
		return Instance{}, fmt.Errorf("error fetching pool instance by name: %w", q.Error)
	}

	instance.Pool = pool
	return instance, nil
}

func (s *sqlDatabase) getInstance(_ context.Context, instanceNameOrID string, preload ...string) (Instance, error) {
	var instance Instance

	var whereArg any = instanceNameOrID
	whereClause := "name = ?"
	id, err := uuid.Parse(instanceNameOrID)
	if err == nil {
		whereArg = id
		whereClause = "id = ?"
	}
	q := s.conn

	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	q = q.Model(&Instance{}).
		Preload(clause.Associations).
		Where(whereClause, whereArg).
		First(&instance)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Instance{}, fmt.Errorf("error fetching instance by name: %w", runnerErrors.ErrNotFound)
		}
		return Instance{}, fmt.Errorf("error fetching instance by name: %w", q.Error)
	}
	return instance, nil
}

func (s *sqlDatabase) GetInstance(ctx context.Context, instanceName string) (params.Instance, error) {
	instance, err := s.getInstance(ctx, instanceName, "StatusMessages", "Pool", "ScaleSet")
	if err != nil {
		return params.Instance{}, fmt.Errorf("error fetching instance: %w", err)
	}

	return s.sqlToParamsInstance(instance)
}

func (s *sqlDatabase) DeleteInstance(_ context.Context, poolID string, instanceName string) (err error) {
	instance, err := s.getPoolInstanceByName(poolID, instanceName)
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("error deleting instance: %w", err)
	}

	defer func() {
		if err == nil {
			var providerID string
			if instance.ProviderID != nil {
				providerID = *instance.ProviderID
			}
			instanceNotif := params.Instance{
				ID:         instance.ID.String(),
				Name:       instance.Name,
				ProviderID: providerID,
				AgentID:    instance.AgentID,
			}
			switch {
			case instance.PoolID != nil:
				instanceNotif.PoolID = instance.PoolID.String()
			case instance.ScaleSetFkID != nil:
				instanceNotif.ScaleSetID = *instance.ScaleSetFkID
			}

			if notifyErr := s.sendNotify(common.InstanceEntityType, common.DeleteOperation, instanceNotif); notifyErr != nil {
				slog.With(slog.Any("error", notifyErr)).Error("failed to send notify")
			}
		}
	}()

	if q := s.conn.Unscoped().Delete(&instance); q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("error deleting instance: %w", q.Error)
	}
	return nil
}

func (s *sqlDatabase) DeleteInstanceByName(ctx context.Context, instanceName string) error {
	instance, err := s.getInstance(ctx, instanceName, "Pool", "ScaleSet")
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("error deleting instance: %w", err)
	}

	defer func() {
		if err == nil {
			var providerID string
			if instance.ProviderID != nil {
				providerID = *instance.ProviderID
			}
			payload := params.Instance{
				ID:         instance.ID.String(),
				Name:       instance.Name,
				ProviderID: providerID,
				AgentID:    instance.AgentID,
			}
			if instance.PoolID != nil {
				payload.PoolID = instance.PoolID.String()
			}
			if instance.ScaleSetFkID != nil {
				payload.ScaleSetID = *instance.ScaleSetFkID
			}
			if notifyErr := s.sendNotify(common.InstanceEntityType, common.DeleteOperation, payload); notifyErr != nil {
				slog.With(slog.Any("error", notifyErr)).Error("failed to send notify")
			}
		}
	}()

	if q := s.conn.Unscoped().Delete(&instance); q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("error deleting instance: %w", q.Error)
	}
	return nil
}

func (s *sqlDatabase) AddInstanceEvent(ctx context.Context, instanceName string, event params.EventType, eventLevel params.EventLevel, statusMessage string) error {
	instance, err := s.getInstance(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("error updating instance: %w", err)
	}

	msg := InstanceStatusUpdate{
		Message:    statusMessage,
		EventType:  event,
		EventLevel: eventLevel,
	}

	if err := s.conn.Model(&instance).Association("StatusMessages").Append(&msg); err != nil {
		return fmt.Errorf("error adding status message: %w", err)
	}
	return nil
}

func (s *sqlDatabase) UpdateInstance(ctx context.Context, instanceName string, param params.UpdateInstanceParams) (params.Instance, error) {
	instance, err := s.getInstance(ctx, instanceName, "Pool", "ScaleSet")
	if err != nil {
		return params.Instance{}, fmt.Errorf("error updating instance: %w", err)
	}

	if param.AgentID != 0 {
		instance.AgentID = param.AgentID
	}

	if param.ProviderID != "" {
		instance.ProviderID = &param.ProviderID
	}

	if param.OSName != "" {
		instance.OSName = param.OSName
	}

	if param.OSVersion != "" {
		instance.OSVersion = param.OSVersion
	}

	if string(param.RunnerStatus) != "" {
		instance.RunnerStatus = param.RunnerStatus
	}

	if string(param.Status) != "" {
		instance.Status = param.Status
	}
	if param.CreateAttempt != 0 {
		instance.CreateAttempt = param.CreateAttempt
	}

	if param.TokenFetched != nil {
		instance.TokenFetched = *param.TokenFetched
	}

	if param.JitConfiguration != nil {
		secret, err := s.marshalAndSeal(param.JitConfiguration)
		if err != nil {
			return params.Instance{}, fmt.Errorf("error marshalling jit config: %w", err)
		}
		instance.JitConfiguration = secret
	}

	instance.ProviderFault = param.ProviderFault

	q := s.conn.Save(&instance)
	if q.Error != nil {
		return params.Instance{}, fmt.Errorf("error updating instance: %w", q.Error)
	}

	if len(param.Addresses) > 0 {
		addrs := []Address{}
		for _, addr := range param.Addresses {
			addrs = append(addrs, Address{
				Address: addr.Address,
				Type:    string(addr.Type),
			})
		}
		if err := s.conn.Model(&instance).Association("Addresses").Replace(addrs); err != nil {
			return params.Instance{}, fmt.Errorf("error updating addresses: %w", err)
		}
	}
	inst, err := s.sqlToParamsInstance(instance)
	if err != nil {
		return params.Instance{}, fmt.Errorf("error converting instance: %w", err)
	}
	s.sendNotify(common.InstanceEntityType, common.UpdateOperation, inst)
	return inst, nil
}

// listInstancesBatched is a helper function that retrieves instances in batches
// and converts them to params.Instance. It accepts a query modifier function
// to customize the base query (e.g., add WHERE clauses).
func (s *sqlDatabase) listInstancesBatched(queryModifier func(*gorm.DB) *gorm.DB) ([]params.Instance, error) {
	ret := []params.Instance{}
	err := s.conn.Transaction(func(tx *gorm.DB) error {
		batchSize := 1000
		offset := 0
		for {
			var batch []Instance

			// Start with base query and apply modifier
			query := tx.Limit(batchSize).Offset(offset).
				Preload("Pool").
				Preload("ScaleSet").
				Preload("Job")

			if queryModifier != nil {
				query = queryModifier(query)
			}

			q := query.Find(&batch)
			if q.Error != nil {
				return fmt.Errorf("error fetching instances: %w", q.Error)
			}
			if len(batch) == 0 {
				break
			}

			// Pre-grow slice to avoid multiple small reallocations
			if cap(ret) < len(ret)+len(batch) {
				newCap := max(len(ret)+len(batch), cap(ret)*2)
				newRet := make([]params.Instance, len(ret), newCap)
				copy(newRet, ret)
				ret = newRet
			}

			// Convert directly into result slice
			for _, instance := range batch {
				converted, err := s.sqlToParamsInstance(instance)
				if err != nil {
					return fmt.Errorf("error converting instance: %w", err)
				}
				ret = append(ret, converted)
			}
			offset += len(batch)
		}
		return nil
	})
	return ret, err
}

func (s *sqlDatabase) ListPoolInstances(_ context.Context, poolID string) ([]params.Instance, error) {
	u, err := uuid.Parse(poolID)
	if err != nil {
		return nil, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
	}

	ret, err := s.listInstancesBatched(func(query *gorm.DB) *gorm.DB {
		return query.Where("pool_id = ?", u)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pool instances: %w", err)
	}
	return ret, nil
}

func (s *sqlDatabase) ListAllInstances(_ context.Context) ([]params.Instance, error) {
	ret, err := s.listInstancesBatched(nil) // No query modifier for all instances
	if err != nil {
		return nil, fmt.Errorf("failed to list all instances: %w", err)
	}
	return ret, nil
}

func (s *sqlDatabase) PoolInstanceCount(_ context.Context, poolID string) (int64, error) {
	pool, err := s.getPoolByID(s.conn, poolID)
	if err != nil {
		return 0, fmt.Errorf("error fetching pool: %w", err)
	}

	var cnt int64
	q := s.conn.Model(&Instance{}).Where("pool_id = ?", pool.ID).Count(&cnt)
	if q.Error != nil {
		return 0, fmt.Errorf("error fetching instance count: %w", q.Error)
	}
	return cnt, nil
}
