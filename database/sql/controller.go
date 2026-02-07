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
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/appdefaults"
)

func dbControllerToCommonController(dbInfo ControllerInfo) (params.ControllerInfo, error) {
	url, err := url.JoinPath(dbInfo.WebhookBaseURL, dbInfo.ControllerID.String())
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error joining webhook URL: %w", err)
	}

	if dbInfo.GARMAgentReleasesURL == "" {
		dbInfo.GARMAgentReleasesURL = appdefaults.GARMAgentDefaultReleasesURL
	}

	ret := params.ControllerInfo{
		ControllerID:                    dbInfo.ControllerID,
		MetadataURL:                     dbInfo.MetadataURL,
		WebhookURL:                      dbInfo.WebhookBaseURL,
		ControllerWebhookURL:            url,
		CallbackURL:                     dbInfo.CallbackURL,
		AgentURL:                        dbInfo.AgentURL,
		MinimumJobAgeBackoff:            dbInfo.MinimumJobAgeBackoff,
		Version:                         appdefaults.GetVersion(),
		GARMAgentReleasesURL:            dbInfo.GARMAgentReleasesURL,
		SyncGARMAgentTools:              dbInfo.SyncGARMAgentTools,
		CachedGARMAgentReleaseFetchedAt: dbInfo.CachedGARMAgentReleaseFetchedAt,
		CachedGARMAgentRelease:          dbInfo.CachedGARMAgentRelease,
	}

	// Parse cached release data to populate CachedGARMAgentTools
	if len(dbInfo.CachedGARMAgentRelease) > 0 {
		tools, err := garmUtil.ParseToolsFromRelease(dbInfo.CachedGARMAgentRelease)
		if err != nil {
			slog.Warn("failed to parse cached tools during DB conversion", "error", err)
		} else {
			ret.CachedGARMAgentTools = tools
		}
	}

	return ret, nil
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

func (s *sqlDatabase) HasEntitiesWithAgentModeEnabled() (bool, error) {
	var reposCnt int64
	if err := s.conn.Model(&Repository{}).Where("agent_mode = ?", true).Count(&reposCnt).Error; err != nil {
		return false, fmt.Errorf("error fetching repo count: %w", err)
	}

	var orgCount int64
	if err := s.conn.Model(&Organization{}).Where("agent_mode = ?", true).Count(&orgCount).Error; err != nil {
		return false, fmt.Errorf("error fetching repo count: %w", err)
	}

	var enterpriseCount int64
	if err := s.conn.Model(&Enterprise{}).Where("agent_mode = ?", true).Count(&enterpriseCount).Error; err != nil {
		return false, fmt.Errorf("error fetching repo count: %w", err)
	}
	return reposCnt+orgCount+enterpriseCount > 0, nil
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
		GARMAgentReleasesURL: appdefaults.GARMAgentDefaultReleasesURL,
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

		if info.AgentURL != nil {
			dbInfo.AgentURL = *info.AgentURL
		}

		if info.GARMAgentReleasesURL != nil {
			agentToolsURL := *info.GARMAgentReleasesURL
			if agentToolsURL == "" {
				agentToolsURL = appdefaults.GARMAgentDefaultReleasesURL
			}
			dbInfo.GARMAgentReleasesURL = agentToolsURL
		}

		if info.SyncGARMAgentTools != nil {
			dbInfo.SyncGARMAgentTools = *info.SyncGARMAgentTools
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

func (s *sqlDatabase) UpdateCachedGARMAgentRelease(releaseData []byte, fetchedAt time.Time) error {
	var dbInfo ControllerInfo
	err := s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&ControllerInfo{}).First(&dbInfo)
		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching controller info: %w", runnerErrors.ErrNotFound)
			}
			return fmt.Errorf("error fetching controller info: %w", q.Error)
		}

		dbInfo.CachedGARMAgentRelease = releaseData
		dbInfo.CachedGARMAgentReleaseFetchedAt = &fetchedAt

		q = tx.Save(&dbInfo)
		if q.Error != nil {
			return fmt.Errorf("error saving controller info: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error updating cached release: %w", err)
	}

	paramInfo, err := dbControllerToCommonController(dbInfo)
	if err != nil {
		return fmt.Errorf("error converting controller info: %w", err)
	}
	s.sendNotify(common.ControllerEntityType, common.UpdateOperation, paramInfo)

	return nil
}
