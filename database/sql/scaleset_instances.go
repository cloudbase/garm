// Copyright 2025 Cloudbase Solutions SRL
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
	"fmt"

	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateScaleSetInstance(_ context.Context, scaleSetID uint, param params.CreateInstanceParams) (instance params.Instance, err error) {
	scaleSet, err := s.getScaleSetByID(s.conn, scaleSetID)
	if err != nil {
		return params.Instance{}, fmt.Errorf("error fetching scale set: %w", err)
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.InstanceEntityType, common.CreateOperation, instance)
		}
	}()

	var secret []byte
	if len(param.JitConfiguration) > 0 {
		secret, err = s.marshalAndSeal(param.JitConfiguration)
		if err != nil {
			return params.Instance{}, fmt.Errorf("error marshalling jit config: %w", err)
		}
	}

	newInstance := Instance{
		ScaleSet:          scaleSet,
		Name:              param.Name,
		Status:            param.Status,
		RunnerStatus:      param.RunnerStatus,
		OSType:            param.OSType,
		OSArch:            param.OSArch,
		CallbackURL:       param.CallbackURL,
		MetadataURL:       param.MetadataURL,
		GitHubRunnerGroup: param.GitHubRunnerGroup,
		JitConfiguration:  secret,
		AgentID:           param.AgentID,
	}
	q := s.conn.Create(&newInstance)
	if q.Error != nil {
		return params.Instance{}, fmt.Errorf("error creating instance: %w", q.Error)
	}

	return s.sqlToParamsInstance(newInstance)
}

func (s *sqlDatabase) ListScaleSetInstances(_ context.Context, scalesetID uint) ([]params.Instance, error) {
	var instances []Instance
	query := s.conn.
		Preload("ScaleSet").
		Preload("Job").
		Where("scale_set_fk_id = ?", scalesetID)

	if err := query.Find(&instances); err.Error != nil {
		return nil, fmt.Errorf("error fetching instances: %w", err.Error)
	}

	var err error
	ret := make([]params.Instance, len(instances))
	for idx, inst := range instances {
		ret[idx], err = s.sqlToParamsInstance(inst)
		if err != nil {
			return nil, fmt.Errorf("error converting instance: %w", err)
		}
	}
	return ret, nil
}
