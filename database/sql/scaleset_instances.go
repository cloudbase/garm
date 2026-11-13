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
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateScaleSetInstance(ctx context.Context, scaleSetID uint, param params.CreateInstanceParams) (instance params.Instance, err error) {
	err = s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		scaleSet, err := s.getScaleSetByID(tx, scaleSetID)
		if err != nil {
			return err
		}
		if err := providerLock(tx, scaleSet.ProviderName); err != nil {
			return err
		}
		if _, err := admissionForProvider(tx, scaleSet.ProviderName); err == nil {
			return fmt.Errorf("enrolled provider requires an admission reservation")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		instance, err = s.createScaleSetInstance(tx, scaleSetID, param)
		return err
	})
	if err == nil {
		s.sendNotify(common.InstanceEntityType, common.CreateOperation, instance)
	}
	return instance, err
}

// CreateAdmittedScaleSetInstance binds and inserts the instance in one transaction.
func (s *sqlDatabase) CreateAdmittedScaleSetInstance(ctx context.Context, reservationID string, scaleSetID uint, param params.CreateInstanceParams) (instance params.Instance, err error) {
	if param.Name == "" {
		return params.Instance{}, fmt.Errorf("empty instance name")
	}
	err = s.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		scaleSet, err := s.getScaleSetByID(tx, scaleSetID)
		if err != nil {
			return err
		}
		if err := providerLock(tx, scaleSet.ProviderName); err != nil {
			return err
		}
		d, err := admissionForProvider(tx, scaleSet.ProviderName)
		if err != nil {
			return err
		}
		d, err = lockDomain(tx, d.Domain)
		if err != nil {
			return err
		}
		var r admissionReservation
		if err := tx.First(&r, "id = ?", reservationID).Error; err != nil {
			return err
		}
		if d.Drained || !d.Ready || d.BackendIdentity == "" || d.BackendIdentity != r.BackendIdentity || d.Generation != r.Generation || !d.HealthyUntil.After(time.Now()) {
			return fmt.Errorf("admission unavailable or health stale")
		}
		if scaleSet.Image != d.Image || scaleSet.Flavor != d.Flavor || string(scaleSet.OSArch) != d.OSArch {
			return fmt.Errorf("scale set identity differs from admission health")
		}
		if !scaleSet.Enabled || scaleSet.MinIdleRunners != 0 || r.Domain != d.Domain || r.Owner != fmt.Sprintf("scaleset:%d", scaleSetID) || vector(r) != (params.AdmissionVector{CPU: d.ClassCPU, RAM: d.ClassRAM, Disk: d.ClassDisk, VMs: d.ClassVMs, Jobs: d.ClassJobs}) {
			return fmt.Errorf("reservation does not match enrolled scale set")
		}
		if r.State != admissionReserved && r.State != admissionSending {
			return fmt.Errorf("reservation cannot be bound")
		}
		if r.InstanceName != nil && *r.InstanceName != param.Name {
			return fmt.Errorf("reservation already bound")
		}
		if err := tx.Model(&r).Update("instance_name", param.Name).Error; err != nil {
			return err
		}
		instance, err = s.createScaleSetInstance(tx, scaleSetID, param)
		return err
	})
	if err == nil {
		s.sendNotify(common.InstanceEntityType, common.CreateOperation, instance)
	}
	return instance, err
}

func (s *sqlDatabase) createScaleSetInstance(tx *gorm.DB, scaleSetID uint, param params.CreateInstanceParams) (instance params.Instance, err error) {
	scaleSet, err := s.getScaleSetByID(tx, scaleSetID)
	if err != nil {
		return params.Instance{}, fmt.Errorf("error fetching scale set: %w", err)
	}

	var secret []byte
	if len(param.JitConfiguration) > 0 {
		secret, err = s.marshalAndSeal(param.JitConfiguration)
		if err != nil {
			return params.Instance{}, fmt.Errorf("error marshalling jit config: %w", err)
		}
	}

	// The scale set is set both as the foreign key and as the association.
	// The key is what gets persisted; the association is only here so
	// sqlToParamsInstance can read ProviderName off it. Omit("ScaleSet")
	// below stops gorm upserting the parent row as a side effect of
	// creating a runner.
	newInstance := Instance{
		ScaleSetFkID:      &scaleSet.ID,
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
	q := tx.Omit("ScaleSet").Create(&newInstance)
	if q.Error != nil {
		return params.Instance{}, fmt.Errorf("error creating instance: %w", q.Error)
	}

	return s.sqlToParamsInstance(newInstance)
}

func (s *sqlDatabase) ListScaleSetInstances(ctx context.Context, scalesetID uint, outdatedOnly bool) ([]params.Instance, error) {
	ret, err := s.listInstancesBatched(ctx, func(query *gorm.DB) *gorm.DB {
		q := query.Where("scale_set_fk_id = ?", scalesetID)
		if outdatedOnly {
			q = q.Where("instances.generation < (SELECT scale_sets.generation FROM scale_sets WHERE scale_sets.id = instances.scale_set_fk_id)")
		}
		return q
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list scaleset instances: %w", err)
	}
	return ret, nil
}
