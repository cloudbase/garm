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
	"net/url"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

func dbControllerToCommonController(dbInfo ControllerInfo) (params.ControllerInfo, error) {
	url, err := url.JoinPath(dbInfo.WebhookBaseURL, dbInfo.ControllerID.String())
	if err != nil {
		return params.ControllerInfo{}, errors.Wrap(err, "joining webhook URL")
	}

	return params.ControllerInfo{
		ControllerID:         dbInfo.ControllerID,
		MetadataURL:          dbInfo.MetadataURL,
		WebhookURL:           dbInfo.WebhookBaseURL,
		ControllerWebhookURL: url,
		CallbackURL:          dbInfo.CallbackURL,
	}, nil
}

func (s *sqlDatabase) ControllerInfo() (params.ControllerInfo, error) {
	var info ControllerInfo
	q := s.conn.Model(&ControllerInfo{}).First(&info)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.ControllerInfo{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching controller info")
		}
		return params.ControllerInfo{}, errors.Wrap(q.Error, "fetching controller info")
	}

	paramInfo, err := dbControllerToCommonController(info)
	if err != nil {
		return params.ControllerInfo{}, errors.Wrap(err, "converting controller info")
	}

	return paramInfo, nil
}

func (s *sqlDatabase) InitController() (params.ControllerInfo, error) {
	if _, err := s.ControllerInfo(); err == nil {
		return params.ControllerInfo{}, runnerErrors.NewConflictError("controller already initialized")
	}

	newID, err := uuid.NewRandom()
	if err != nil {
		return params.ControllerInfo{}, errors.Wrap(err, "generating UUID")
	}

	newInfo := ControllerInfo{
		ControllerID: newID,
	}

	q := s.conn.Save(&newInfo)
	if q.Error != nil {
		return params.ControllerInfo{}, errors.Wrap(q.Error, "saving controller info")
	}

	return params.ControllerInfo{
		ControllerID: newInfo.ControllerID,
	}, nil
}

func (s *sqlDatabase) UpdateController(info params.UpdateControllerParams) (params.ControllerInfo, error) {
	var dbInfo ControllerInfo
	q := s.conn.Model(&ControllerInfo{}).First(&dbInfo)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.ControllerInfo{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching controller info")
		}
		return params.ControllerInfo{}, errors.Wrap(q.Error, "fetching controller info")
	}

	if err := info.Validate(); err != nil {
		return params.ControllerInfo{}, errors.Wrap(err, "validating controller info")
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

	q = s.conn.Save(&dbInfo)
	if q.Error != nil {
		return params.ControllerInfo{}, errors.Wrap(q.Error, "saving controller info")
	}

	paramInfo, err := dbControllerToCommonController(dbInfo)
	if err != nil {
		return params.ControllerInfo{}, errors.Wrap(err, "converting controller info")
	}
	return paramInfo, nil
}
