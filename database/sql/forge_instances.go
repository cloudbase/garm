// Copyright 2026 Cloudbase Solutions SRL
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
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateForgeInstance(ctx context.Context, endpointName string, credentials params.ForgeCredentials, webhookSecret string, poolBalancerType params.PoolBalancerType, agentMode bool) (paramFI params.ForgeInstance, err error) {
	if webhookSecret == "" {
		return params.ForgeInstance{}, errors.New("creating forge instance: missing secret")
	}
	if credentials.ForgeType != params.GiteaEndpointType {
		return params.ForgeInstance{}, fmt.Errorf("forge instances are only supported for gitea endpoints: %w", runnerErrors.ErrBadRequest)
	}
	if credentials.Endpoint.Name != endpointName {
		return params.ForgeInstance{}, fmt.Errorf("credentials endpoint %q does not match requested endpoint %q: %w", credentials.Endpoint.Name, endpointName, runnerErrors.ErrBadRequest)
	}

	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error encoding secret: %w", err)
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.ForgeInstanceEntityType, common.CreateOperation, paramFI)
		}
	}()
	newForgeInstance := ForgeInstance{
		WebhookSecret:    secret,
		PoolBalancerType: poolBalancerType,
		AgentMode:        agentMode,
	}
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		newForgeInstance.GiteaCredentialsID = &credentials.ID
		newForgeInstance.EndpointName = &endpointName

		q := tx.Create(&newForgeInstance)
		if q.Error != nil {
			return fmt.Errorf("error creating forge instance: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error creating forge instance: %w", err)
	}

	ret, err := s.GetForgeInstanceByID(ctx, newForgeInstance.ID.String())
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error creating forge instance: %w", err)
	}

	return ret, nil
}

func (s *sqlDatabase) GetForgeInstance(ctx context.Context, endpointName string) (params.ForgeInstance, error) {
	fi, err := s.getForgeInstanceByName(ctx, s.conn, endpointName, "GiteaCredentials", "GiteaCredentials.Endpoint", "Endpoint")
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error fetching forge instance: %w", err)
	}

	param, err := s.sqlToCommonForgeInstance(fi, true)
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error fetching forge instance: %w", err)
	}
	return param, nil
}

func (s *sqlDatabase) GetForgeInstanceByID(ctx context.Context, forgeInstanceID string) (params.ForgeInstance, error) {
	preloadList := []string{
		"Pools",
		"GiteaCredentials",
		"GiteaCredentials.Endpoint",
		"Endpoint",
		"Events",
	}
	fi, err := s.getForgeInstanceByID(ctx, s.conn, forgeInstanceID, preloadList...)
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error fetching forge instance: %w", err)
	}

	param, err := s.sqlToCommonForgeInstance(fi, true)
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error fetching forge instance: %w", err)
	}
	return param, nil
}

func (s *sqlDatabase) ListForgeInstances(_ context.Context, filter params.ForgeInstanceFilter) ([]params.ForgeInstance, error) {
	var forgeInstances []ForgeInstance
	q := s.conn.
		Preload("GiteaCredentials").
		Preload("GiteaCredentials.Endpoint").
		Preload("Endpoint")
	if filter.Endpoint != "" {
		q = q.Where("endpoint_name = ?", filter.Endpoint)
	}
	q = q.Find(&forgeInstances)
	if q.Error != nil {
		return []params.ForgeInstance{}, fmt.Errorf("error fetching forge instances: %w", q.Error)
	}

	ret := make([]params.ForgeInstance, len(forgeInstances))
	for idx, val := range forgeInstances {
		var err error
		ret[idx], err = s.sqlToCommonForgeInstance(val, true)
		if err != nil {
			return nil, fmt.Errorf("error fetching forge instances: %w", err)
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteForgeInstance(ctx context.Context, forgeInstanceID string) error {
	fi, err := s.getForgeInstanceByID(ctx, s.conn, forgeInstanceID, "Endpoint", "GiteaCredentials", "GiteaCredentials.Endpoint")
	if err != nil {
		return fmt.Errorf("error fetching forge instance: %w", err)
	}

	defer func(inst ForgeInstance) {
		if err == nil {
			asParams, innerErr := s.sqlToCommonForgeInstance(inst, true)
			if innerErr == nil {
				s.sendNotify(common.ForgeInstanceEntityType, common.DeleteOperation, asParams)
			} else {
				slog.With(slog.Any("error", innerErr)).ErrorContext(ctx, "error sending delete notification", "forge_instance", forgeInstanceID)
			}
		}
	}(fi)

	q := s.conn.Unscoped().Delete(&fi)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("error deleting forge instance: %w", q.Error)
	}

	return nil
}

func (s *sqlDatabase) UpdateForgeInstance(ctx context.Context, forgeInstanceID string, param params.UpdateEntityParams) (newParams params.ForgeInstance, err error) {
	var rowsAffected int64
	defer func() {
		if err == nil && rowsAffected > 0 {
			s.sendNotify(common.ForgeInstanceEntityType, common.UpdateOperation, newParams)
		}
	}()
	var fi ForgeInstance
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var err error
		fi, err = s.getForgeInstanceByID(ctx, tx.Clauses(clause.Locking{Strength: "UPDATE"}), forgeInstanceID, "Endpoint")
		if err != nil {
			return fmt.Errorf("error fetching forge instance: %w", err)
		}

		if fi.EndpointName == nil {
			return fmt.Errorf("forge instance has no endpoint: %w", runnerErrors.ErrUnprocessable)
		}

		updates := make(map[string]any)

		if err := s.updateEntityCredentials(ctx, tx, &fi, param.CredentialsName, updates); err != nil {
			return err
		}

		if param.WebhookSecret != "" {
			existingSecret, err := util.Unseal(fi.WebhookSecret, []byte(s.cfg.Passphrase))
			if err != nil {
				return fmt.Errorf("failed to decrypt existing webhook secret: %w", err)
			}
			if string(existingSecret) != param.WebhookSecret {
				secret, err := util.Seal([]byte(param.WebhookSecret), []byte(s.cfg.Passphrase))
				if err != nil {
					return fmt.Errorf("error encoding secret: %w", err)
				}
				updates["webhook_secret"] = secret
			}
		}

		if param.PoolManagerStatus != nil {
			if param.PoolManagerStatus.IsRunning != fi.PoolManagerRunning {
				updates["pool_manager_running"] = param.PoolManagerStatus.IsRunning
			}
			if param.PoolManagerStatus.FailureReason != fi.PoolManagerFailureReason {
				updates["pool_manager_failure_reason"] = param.PoolManagerStatus.FailureReason
			}
		}

		if param.PoolBalancerType != "" && param.PoolBalancerType != fi.PoolBalancerType {
			updates["pool_balancer_type"] = param.PoolBalancerType
		}

		if param.AgentMode != nil && *param.AgentMode != fi.AgentMode {
			updates["agent_mode"] = *param.AgentMode
		}

		if len(updates) > 0 {
			q := tx.Model(&fi).Omit("Endpoint").Updates(updates)
			if q.Error != nil {
				return fmt.Errorf("error saving forge instance: %w", q.Error)
			}
			rowsAffected = q.RowsAffected
		}

		return nil
	})
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error updating forge instance: %w", err)
	}

	fi, err = s.getForgeInstanceByID(ctx, s.conn, forgeInstanceID, "Endpoint", "GiteaCredentials", "GiteaCredentials.Endpoint")
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error updating forge instance: %w", err)
	}
	newParams, err = s.sqlToCommonForgeInstance(fi, true)
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error updating forge instance: %w", err)
	}
	return newParams, nil
}

func (s *sqlDatabase) getForgeInstanceByName(_ context.Context, tx *gorm.DB, name string, preload ...string) (ForgeInstance, error) {
	if name == "" {
		return ForgeInstance{}, fmt.Errorf("missing forge instance name: %w", runnerErrors.ErrBadRequest)
	}
	var fi ForgeInstance

	q := tx
	for _, field := range preload {
		q = q.Preload(field)
	}
	q = q.Where("LOWER(endpoint_name) = LOWER(?)", name).First(&fi)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return ForgeInstance{}, runnerErrors.ErrNotFound
		}
		return ForgeInstance{}, fmt.Errorf("error fetching forge instance from database: %w", q.Error)
	}
	return fi, nil
}

func (s *sqlDatabase) getForgeInstanceByID(_ context.Context, tx *gorm.DB, id string, preload ...string) (ForgeInstance, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return ForgeInstance{}, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
	}
	var fi ForgeInstance

	q := tx
	for _, field := range preload {
		q = q.Preload(field)
	}
	q = q.Where("id = ?", u).First(&fi)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return ForgeInstance{}, runnerErrors.ErrNotFound
		}
		return ForgeInstance{}, fmt.Errorf("error fetching forge instance from database: %w", q.Error)
	}
	return fi, nil
}
