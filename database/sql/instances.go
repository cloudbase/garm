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

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) marshalAndSeal(data interface{}) ([]byte, error) {
	enc, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling data")
	}
	return util.Seal(enc, []byte(s.cfg.Passphrase))
}

func (s *sqlDatabase) unsealAndUnmarshal(data []byte, target interface{}) error {
	decrypted, err := util.Unseal(data, []byte(s.cfg.Passphrase))
	if err != nil {
		return errors.Wrap(err, "decrypting data")
	}
	if err := json.Unmarshal(decrypted, target); err != nil {
		return errors.Wrap(err, "unmarshalling data")
	}
	return nil
}

func (s *sqlDatabase) CreateInstance(ctx context.Context, poolID string, param params.CreateInstanceParams) (params.Instance, error) {
	pool, err := s.getPoolByID(ctx, poolID)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching pool")
	}

	var labels datatypes.JSON
	if len(param.AditionalLabels) > 0 {
		labels, err = json.Marshal(param.AditionalLabels)
		if err != nil {
			return params.Instance{}, errors.Wrap(err, "marshalling labels")
		}
	}

	var secret []byte
	if len(param.JitConfiguration) > 0 {
		secret, err = s.marshalAndSeal(param.JitConfiguration)
		if err != nil {
			return params.Instance{}, errors.Wrap(err, "marshalling jit config")
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
	q := s.conn.Create(&newInstance)
	if q.Error != nil {
		return params.Instance{}, errors.Wrap(q.Error, "creating instance")
	}

	return s.sqlToParamsInstance(newInstance)
}

func (s *sqlDatabase) getInstanceByID(_ context.Context, instanceID string) (Instance, error) {
	u, err := uuid.Parse(instanceID)
	if err != nil {
		return Instance{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("id = ?", u).
		First(&instance)
	if q.Error != nil {
		return Instance{}, errors.Wrap(q.Error, "fetching instance")
	}
	return instance, nil
}

func (s *sqlDatabase) getPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (Instance, error) {
	pool, err := s.getPoolByID(ctx, poolID)
	if err != nil {
		return Instance{}, errors.Wrap(err, "fetching pool")
	}

	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("name = ? and pool_id = ?", instanceName, pool.ID).
		First(&instance)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Instance{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching pool instance by name")
		}
		return Instance{}, errors.Wrap(q.Error, "fetching pool instance by name")
	}
	return instance, nil
}

func (s *sqlDatabase) getInstanceByName(_ context.Context, instanceName string, preload ...string) (Instance, error) {
	var instance Instance

	q := s.conn

	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	q = q.Model(&Instance{}).
		Preload(clause.Associations).
		Where("name = ?", instanceName).
		First(&instance)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Instance{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching instance by name")
		}
		return Instance{}, errors.Wrap(q.Error, "fetching instance by name")
	}
	return instance, nil
}

func (s *sqlDatabase) GetPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (params.Instance, error) {
	instance, err := s.getPoolInstanceByName(ctx, poolID, instanceName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	return s.sqlToParamsInstance(instance)
}

func (s *sqlDatabase) GetInstanceByName(ctx context.Context, instanceName string) (params.Instance, error) {
	instance, err := s.getInstanceByName(ctx, instanceName, "StatusMessages")
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	return s.sqlToParamsInstance(instance)
}

func (s *sqlDatabase) DeleteInstance(ctx context.Context, poolID string, instanceName string) error {
	instance, err := s.getPoolInstanceByName(ctx, poolID, instanceName)
	if err != nil {
		return errors.Wrap(err, "deleting instance")
	}
	if q := s.conn.Unscoped().Delete(&instance); q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "deleting instance")
	}
	return nil
}

func (s *sqlDatabase) ListInstanceEvents(_ context.Context, instanceID string, eventType params.EventType, eventLevel params.EventLevel) ([]params.StatusMessage, error) {
	var events []InstanceStatusUpdate
	query := s.conn.Model(&InstanceStatusUpdate{}).Where("instance_id = ?", instanceID)
	if eventLevel != "" {
		query = query.Where("event_level = ?", eventLevel)
	}

	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	if result := query.Find(&events); result.Error != nil {
		return nil, errors.Wrap(result.Error, "fetching events")
	}

	eventParams := make([]params.StatusMessage, len(events))
	for idx, val := range events {
		eventParams[idx] = params.StatusMessage{
			Message:    val.Message,
			EventType:  val.EventType,
			EventLevel: val.EventLevel,
		}
	}
	return eventParams, nil
}

func (s *sqlDatabase) AddInstanceEvent(ctx context.Context, instanceID string, event params.EventType, eventLevel params.EventLevel, statusMessage string) error {
	instance, err := s.getInstanceByID(ctx, instanceID)
	if err != nil {
		return errors.Wrap(err, "updating instance")
	}

	msg := InstanceStatusUpdate{
		Message:    statusMessage,
		EventType:  event,
		EventLevel: eventLevel,
	}

	if err := s.conn.Model(&instance).Association("StatusMessages").Append(&msg); err != nil {
		return errors.Wrap(err, "adding status message")
	}
	return nil
}

func (s *sqlDatabase) UpdateInstance(ctx context.Context, instanceID string, param params.UpdateInstanceParams) (params.Instance, error) {
	instance, err := s.getInstanceByID(ctx, instanceID)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "updating instance")
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
			return params.Instance{}, errors.Wrap(err, "marshalling jit config")
		}
		instance.JitConfiguration = secret
	}

	instance.ProviderFault = param.ProviderFault

	q := s.conn.Save(&instance)
	if q.Error != nil {
		return params.Instance{}, errors.Wrap(q.Error, "updating instance")
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
			return params.Instance{}, errors.Wrap(err, "updating addresses")
		}
	}

	return s.sqlToParamsInstance(instance)
}

func (s *sqlDatabase) ListPoolInstances(_ context.Context, poolID string) ([]params.Instance, error) {
	u, err := uuid.Parse(poolID)
	if err != nil {
		return nil, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}

	var instances []Instance
	query := s.conn.Model(&Instance{}).Where("pool_id = ?", u)

	if err := query.Find(&instances); err.Error != nil {
		return nil, errors.Wrap(err.Error, "fetching instances")
	}

	ret := make([]params.Instance, len(instances))
	for idx, inst := range instances {
		ret[idx], err = s.sqlToParamsInstance(inst)
		if err != nil {
			return nil, errors.Wrap(err, "converting instance")
		}
	}
	return ret, nil
}

func (s *sqlDatabase) ListAllInstances(_ context.Context) ([]params.Instance, error) {
	var instances []Instance

	q := s.conn.Model(&Instance{}).Find(&instances)
	if q.Error != nil {
		return nil, errors.Wrap(q.Error, "fetching instances")
	}
	ret := make([]params.Instance, len(instances))
	var err error
	for idx, instance := range instances {
		ret[idx], err = s.sqlToParamsInstance(instance)
		if err != nil {
			return nil, errors.Wrap(err, "converting instance")
		}
	}
	return ret, nil
}

func (s *sqlDatabase) PoolInstanceCount(ctx context.Context, poolID string) (int64, error) {
	pool, err := s.getPoolByID(ctx, poolID)
	if err != nil {
		return 0, errors.Wrap(err, "fetching pool")
	}

	var cnt int64
	q := s.conn.Model(&Instance{}).Where("pool_id = ?", pool.ID).Count(&cnt)
	if q.Error != nil {
		return 0, errors.Wrap(q.Error, "fetching instance count")
	}
	return cnt, nil
}
