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
	runnerErrors "github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/params"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

func (s *sqlDatabase) ControllerInfo() (params.ControllerInfo, error) {
	var info ControllerInfo
	q := s.conn.Model(&ControllerInfo{}).First(&info)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.ControllerInfo{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching controller info")
		}
		return params.ControllerInfo{}, errors.Wrap(q.Error, "fetching controller info")
	}
	return params.ControllerInfo{
		ControllerID: info.ControllerID,
	}, nil
}

func (s *sqlDatabase) InitController() (params.ControllerInfo, error) {
	if _, err := s.ControllerInfo(); err == nil {
		return params.ControllerInfo{}, runnerErrors.NewConflictError("controller already initialized")
	}

	newID, err := uuid.NewV4()
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
