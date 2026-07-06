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
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/appdefaults"
)

// inferAgentURL extracts the base URL (scheme://host) from a given URL.
// This is used to derive the AgentURL from CallbackURL when AgentURL is not explicitly set.
func inferAgentURL(uri string) (string, error) {
	if uri == "" {
		return "", runnerErrors.NewConflictError("URL cannot be empty")
	}
	parsed, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}
	return strings.TrimSuffix(fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), "/"), nil
}

func dbControllerToCommonController(dbInfo ControllerInfo) (params.ControllerInfo, error) {
	webhookURL, err := url.JoinPath(dbInfo.WebhookBaseURL, dbInfo.ControllerID.String())
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error joining webhook URL: %w", err)
	}

	if dbInfo.GARMAgentReleasesURL == "" {
		dbInfo.GARMAgentReleasesURL = appdefaults.GARMAgentDefaultReleasesURL
	}

	// If AgentURL is not set, derive it from CallbackURL
	agentURL := dbInfo.AgentURL
	if agentURL == "" {
		var err error
		agentURL, err = inferAgentURL(dbInfo.CallbackURL)
		if err != nil {
			slog.Warn("failed to infer agent URL; please update controller info with proper URL", "error", err)
		}
	}

	ret := params.ControllerInfo{
		ControllerID:                    dbInfo.ControllerID,
		MetadataURL:                     dbInfo.MetadataURL,
		WebhookURL:                      dbInfo.WebhookBaseURL,
		ControllerWebhookURL:            webhookURL,
		CallbackURL:                     dbInfo.CallbackURL,
		AgentURL:                        agentURL,
		MinimumJobAgeBackoff:            dbInfo.MinimumJobAgeBackoff,
		Version:                         appdefaults.GetVersion(),
		GARMAgentReleasesURL:            dbInfo.GARMAgentReleasesURL,
		SyncGARMAgentTools:              dbInfo.SyncGARMAgentTools,
		GARMAgentVersion:                dbInfo.GARMAgentVersion,
		CachedGARMAgentReleaseFetchedAt: dbInfo.CachedGARMAgentReleaseFetchedAt,
		CachedGARMAgentReleases:         dbInfo.CachedGARMAgentReleases,
		CACertBundle:                    dbInfo.CACertBundle,
	}

	// Populate CachedGARMAgentTools with the tools of the resolved release:
	// the pinned version when one is set, the latest release otherwise. All
	// consumers of the cached tools (metadata, upstream tool listings)
	// automatically serve the desired version this way. A pin that is absent
	// from the index resolves to the latest release; the sync worker owns
	// warning about it.
	if len(dbInfo.CachedGARMAgentReleases) > 0 {
		index, err := garmUtil.ParseReleaseIndex(dbInfo.CachedGARMAgentReleases)
		if err != nil {
			slog.Warn("failed to parse cached release index during DB conversion", "error", err)
		} else if release, _ := garmUtil.ResolveAgentRelease(index.Releases, dbInfo.GARMAgentVersion); release.TagName != "" {
			ret.CachedGARMAgentTools = garmUtil.ToolsFromRelease(release)
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

// controllerUpdates maps the requested controller changes onto the columns
// that actually differ from the stored values.
func controllerUpdates(info params.UpdateControllerParams, dbInfo ControllerInfo) map[string]interface{} {
	updates := make(map[string]interface{})

	if info.MetadataURL != nil && *info.MetadataURL != dbInfo.MetadataURL {
		updates["metadata_url"] = *info.MetadataURL
	}

	if info.CallbackURL != nil && *info.CallbackURL != dbInfo.CallbackURL {
		updates["callback_url"] = *info.CallbackURL
	}

	if info.WebhookURL != nil && *info.WebhookURL != dbInfo.WebhookBaseURL {
		updates["webhook_base_url"] = *info.WebhookURL
	}

	if info.AgentURL != nil && *info.AgentURL != dbInfo.AgentURL {
		updates["agent_url"] = *info.AgentURL
	}

	if info.ClearCACertBundle != nil && *info.ClearCACertBundle {
		updates["ca_cert_bundle"] = nil
	} else if len(info.CACertBundle) > 0 {
		updates["ca_cert_bundle"] = info.CACertBundle
	}

	if info.GARMAgentReleasesURL != nil {
		agentToolsURL := *info.GARMAgentReleasesURL
		if agentToolsURL == "" {
			agentToolsURL = appdefaults.GARMAgentDefaultReleasesURL
		}
		if agentToolsURL != dbInfo.GARMAgentReleasesURL {
			updates["garm_agent_releases_url"] = agentToolsURL
		}
	}

	if info.SyncGARMAgentTools != nil && *info.SyncGARMAgentTools != dbInfo.SyncGARMAgentTools {
		updates["sync_garm_agent_tools"] = *info.SyncGARMAgentTools
	}

	if info.GARMAgentVersion != nil {
		agentVersion := strings.TrimSpace(*info.GARMAgentVersion)
		// The empty string is the canonical form for "track the latest
		// release"; Validate already made sure anything else is a proper
		// semver version.
		if agentVersion == params.GARMAgentLatestVersion {
			agentVersion = ""
		}
		if agentVersion != dbInfo.GARMAgentVersion {
			updates["garm_agent_version"] = agentVersion
		}
	}

	if info.MinimumJobAgeBackoff != nil && *info.MinimumJobAgeBackoff != dbInfo.MinimumJobAgeBackoff {
		updates["minimum_job_age_backoff"] = *info.MinimumJobAgeBackoff
	}

	return updates
}

func (s *sqlDatabase) UpdateController(info params.UpdateControllerParams) (paramInfo params.ControllerInfo, err error) {
	var rowsAffected int64
	defer func() {
		if err == nil && rowsAffected > 0 {
			s.sendNotify(common.ControllerEntityType, common.UpdateOperation, paramInfo)
		}
	}()
	var dbInfo ControllerInfo
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&ControllerInfo{}).Clauses(clause.Locking{Strength: "UPDATE"}).First(&dbInfo)
		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching controller info: %w", runnerErrors.ErrNotFound)
			}
			return fmt.Errorf("error fetching controller info: %w", q.Error)
		}

		if err := info.Validate(); err != nil {
			return fmt.Errorf("error validating controller info: %w", err)
		}

		if updates := controllerUpdates(info, dbInfo); len(updates) > 0 {
			q = tx.Model(&dbInfo).Updates(updates)
			if q.Error != nil {
				return fmt.Errorf("error saving controller info: %w", q.Error)
			}
			rowsAffected = q.RowsAffected
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

// UpdateCachedGARMAgentReleases persists the release index (a marshaled
// util.AgentReleaseIndex) fetched from the controller's releases URL.
func (s *sqlDatabase) UpdateCachedGARMAgentReleases(index []byte, fetchedAt time.Time) error {
	var dbInfo ControllerInfo
	var rowsAffected int64
	err := s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&ControllerInfo{}).Clauses(clause.Locking{Strength: "UPDATE"}).First(&dbInfo)
		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching controller info: %w", runnerErrors.ErrNotFound)
			}
			return fmt.Errorf("error fetching controller info: %w", q.Error)
		}

		updates := map[string]interface{}{
			"cached_garm_agent_releases":           index,
			"cached_garm_agent_release_fetched_at": &fetchedAt,
		}

		q = tx.Model(&dbInfo).Updates(updates)
		if q.Error != nil {
			return fmt.Errorf("error saving controller info: %w", q.Error)
		}
		rowsAffected = q.RowsAffected
		return nil
	})
	if err != nil {
		return fmt.Errorf("error updating cached release index: %w", err)
	}

	if rowsAffected > 0 {
		paramInfo, err := dbControllerToCommonController(dbInfo)
		if err != nil {
			return fmt.Errorf("error converting controller info: %w", err)
		}
		s.sendNotify(common.ControllerEntityType, common.UpdateOperation, paramInfo)
	}

	return nil
}
