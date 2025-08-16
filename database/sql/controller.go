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
	"errors"
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/appdefaults"
)

func dbControllerToCommonController(dbInfo ControllerInfo) (params.ControllerInfo, error) {
	url, err := url.JoinPath(dbInfo.WebhookBaseURL, dbInfo.ControllerID.String())
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error joining webhook URL: %w", err)
	}

	return params.ControllerInfo{
		ControllerID:         dbInfo.ControllerID,
		MetadataURL:          dbInfo.MetadataURL,
		WebhookURL:           dbInfo.WebhookBaseURL,
		ControllerWebhookURL: url,
		CallbackURL:          dbInfo.CallbackURL,
		MinimumJobAgeBackoff: dbInfo.MinimumJobAgeBackoff,
		Version:              appdefaults.GetVersion(),
	}, nil
}

func (s *sqlDatabase) ControllerInfo() (params.ControllerInfo, error) {
	var info ControllerInfo
	q := s.conn.Model(&ControllerInfo{}).First(&info)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.ControllerInfo{}, fmt.Errorf("error fetching controller info: %w", runnerErrors.ErrNotFound)
		}
		return params.ControllerInfo{}, fmt.Errorf("error fetching controller info: %w", q.Error)
	}

	paramInfo, err := dbControllerToCommonController(info)
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error converting controller info: %w", err)
	}

	return paramInfo, nil
}

func (s *sqlDatabase) InitController() (params.ControllerInfo, error) {
	if _, err := s.ControllerInfo(); err == nil {
		return params.ControllerInfo{}, runnerErrors.NewConflictError("controller already initialized")
	}

	newID, err := uuid.NewRandom()
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error generating UUID: %w", err)
	}

	newInfo := ControllerInfo{
		ControllerID:         newID,
		MinimumJobAgeBackoff: 30,
	}

	q := s.conn.Save(&newInfo)
	if q.Error != nil {
		return params.ControllerInfo{}, fmt.Errorf("error saving controller info: %w", q.Error)
	}

	return params.ControllerInfo{
		ControllerID: newInfo.ControllerID,
	}, nil
}

func (s *sqlDatabase) UpdateController(info params.UpdateControllerParams) (paramInfo params.ControllerInfo, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.ControllerEntityType, common.UpdateOperation, paramInfo)
		}
	}()
	var dbInfo ControllerInfo
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&ControllerInfo{}).First(&dbInfo)
		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching controller info: %w", runnerErrors.ErrNotFound)
			}
			return fmt.Errorf("error fetching controller info: %w", q.Error)
		}

		if err := info.Validate(); err != nil {
			return fmt.Errorf("error validating controller info: %w", err)
		}

		if info.MetadataURL != nil {
			dbInfo.MetadataURL = *info.MetadataURL
		}

		if info.CallbackURL != nil {
			dbInfo.CallbackURL = *info.CallbackURL
		}

		if info.WebhookURL != nil {
			dbInfo.WebhookBaseURL = *info.WebhookURL
		}

		if info.MinimumJobAgeBackoff != nil {
			dbInfo.MinimumJobAgeBackoff = *info.MinimumJobAgeBackoff
		}

		q = tx.Save(&dbInfo)
		if q.Error != nil {
			return fmt.Errorf("error saving controller info: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error updating controller info: %w", err)
	}

	paramInfo, err = dbControllerToCommonController(dbInfo)
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error converting controller info: %w", err)
	}
	return paramInfo, nil
}
