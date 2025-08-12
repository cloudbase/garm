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
	"fmt"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/auth"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) sqlToParamsInstance(instance Instance) (params.Instance, error) {
	var id string
	if instance.ProviderID != nil {
		id = *instance.ProviderID
	}

	var labels []string
	if len(instance.AditionalLabels) > 0 {
		if err := json.Unmarshal(instance.AditionalLabels, &labels); err != nil {
			return params.Instance{}, errors.Wrap(err, "unmarshalling labels")
		}
	}

	var jitConfig map[string]string
	if len(instance.JitConfiguration) > 0 {
		if err := s.unsealAndUnmarshal(instance.JitConfiguration, &jitConfig); err != nil {
			return params.Instance{}, errors.Wrap(err, "unmarshalling jit configuration")
		}
	}
	ret := params.Instance{
		ID:                instance.ID.String(),
		ProviderID:        id,
		AgentID:           instance.AgentID,
		Name:              instance.Name,
		OSType:            instance.OSType,
		OSName:            instance.OSName,
		OSVersion:         instance.OSVersion,
		OSArch:            instance.OSArch,
		Status:            instance.Status,
		RunnerStatus:      instance.RunnerStatus,
		CallbackURL:       instance.CallbackURL,
		MetadataURL:       instance.MetadataURL,
		StatusMessages:    []params.StatusMessage{},
		CreateAttempt:     instance.CreateAttempt,
		CreatedAt:         instance.CreatedAt,
		UpdatedAt:         instance.UpdatedAt,
		TokenFetched:      instance.TokenFetched,
		JitConfiguration:  jitConfig,
		GitHubRunnerGroup: instance.GitHubRunnerGroup,
		AditionalLabels:   labels,
	}

	if instance.ScaleSetFkID != nil {
		ret.ScaleSetID = *instance.ScaleSetFkID
		ret.ProviderName = instance.ScaleSet.ProviderName
	}

	if instance.PoolID != nil {
		ret.PoolID = instance.PoolID.String()
		ret.ProviderName = instance.Pool.ProviderName
	}

	if ret.ScaleSetID == 0 && ret.PoolID == "" {
		return params.Instance{}, errors.New("missing pool or scale set id")
	}

	if ret.ScaleSetID != 0 && ret.PoolID != "" {
		return params.Instance{}, errors.New("both pool and scale set ids are set")
	}

	if instance.Job != nil {
		paramJob, err := sqlWorkflowJobToParamsJob(*instance.Job)
		if err != nil {
			return params.Instance{}, errors.Wrap(err, "converting job")
		}
		ret.Job = &paramJob
	}

	if len(instance.ProviderFault) > 0 {
		ret.ProviderFault = instance.ProviderFault
	}

	for _, addr := range instance.Addresses {
		ret.Addresses = append(ret.Addresses, s.sqlAddressToParamsAddress(addr))
	}

	for _, msg := range instance.StatusMessages {
		ret.StatusMessages = append(ret.StatusMessages, params.StatusMessage{
			CreatedAt:  msg.CreatedAt,
			Message:    msg.Message,
			EventType:  msg.EventType,
			EventLevel: msg.EventLevel,
		})
	}
	return ret, nil
}

func (s *sqlDatabase) sqlAddressToParamsAddress(addr Address) commonParams.Address {
	return commonParams.Address{
		Address: addr.Address,
		Type:    commonParams.AddressType(addr.Type),
	}
}

func (s *sqlDatabase) sqlToCommonOrganization(org Organization, detailed bool) (params.Organization, error) {
	if len(org.WebhookSecret) == 0 {
		return params.Organization{}, errors.New("missing secret")
	}
	secret, err := util.Unseal(org.WebhookSecret, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "decrypting secret")
	}

	endpoint, err := s.sqlToCommonGithubEndpoint(org.Endpoint)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "converting endpoint")
	}
	ret := params.Organization{
		ID:               org.ID.String(),
		Name:             org.Name,
		CredentialsName:  org.Credentials.Name,
		Pools:            make([]params.Pool, len(org.Pools)),
		WebhookSecret:    string(secret),
		PoolBalancerType: org.PoolBalancerType,
		Endpoint:         endpoint,
		CreatedAt:        org.CreatedAt,
		UpdatedAt:        org.UpdatedAt,
	}

	var forgeCreds params.ForgeCredentials
	if org.CredentialsID != nil {
		ret.CredentialsID = *org.CredentialsID
		forgeCreds, err = s.sqlToCommonForgeCredentials(org.Credentials)
	}

	if org.GiteaCredentialsID != nil {
		ret.CredentialsID = *org.GiteaCredentialsID
		forgeCreds, err = s.sqlGiteaToCommonForgeCredentials(org.GiteaCredentials)
	}

	if err != nil {
		return params.Organization{}, errors.Wrap(err, "converting credentials")
	}

	if len(org.Events) > 0 {
		ret.Events = make([]params.EntityEvent, len(org.Events))
		for idx, event := range org.Events {
			ret.Events[idx] = params.EntityEvent{
				ID:         event.ID,
				Message:    event.Message,
				EventType:  event.EventType,
				EventLevel: event.EventLevel,
				CreatedAt:  event.CreatedAt,
			}
		}
	}

	if detailed {
		ret.Credentials = forgeCreds
		ret.CredentialsName = forgeCreds.Name
	}

	if ret.PoolBalancerType == "" {
		ret.PoolBalancerType = params.PoolBalancerTypeRoundRobin
	}

	for idx, pool := range org.Pools {
		ret.Pools[idx], err = s.sqlToCommonPool(pool)
		if err != nil {
			return params.Organization{}, errors.Wrap(err, "converting pool")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) sqlToCommonEnterprise(enterprise Enterprise, detailed bool) (params.Enterprise, error) {
	if len(enterprise.WebhookSecret) == 0 {
		return params.Enterprise{}, errors.New("missing secret")
	}
	secret, err := util.Unseal(enterprise.WebhookSecret, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "decrypting secret")
	}

	endpoint, err := s.sqlToCommonGithubEndpoint(enterprise.Endpoint)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "converting endpoint")
	}
	ret := params.Enterprise{
		ID:               enterprise.ID.String(),
		Name:             enterprise.Name,
		CredentialsName:  enterprise.Credentials.Name,
		Pools:            make([]params.Pool, len(enterprise.Pools)),
		WebhookSecret:    string(secret),
		PoolBalancerType: enterprise.PoolBalancerType,
		CreatedAt:        enterprise.CreatedAt,
		UpdatedAt:        enterprise.UpdatedAt,
		Endpoint:         endpoint,
	}

	if enterprise.CredentialsID != nil {
		ret.CredentialsID = *enterprise.CredentialsID
	}

	if len(enterprise.Events) > 0 {
		ret.Events = make([]params.EntityEvent, len(enterprise.Events))
		for idx, event := range enterprise.Events {
			ret.Events[idx] = params.EntityEvent{
				ID:         event.ID,
				Message:    event.Message,
				EventType:  event.EventType,
				EventLevel: event.EventLevel,
				CreatedAt:  event.CreatedAt,
			}
		}
	}

	if detailed {
		creds, err := s.sqlToCommonForgeCredentials(enterprise.Credentials)
		if err != nil {
			return params.Enterprise{}, errors.Wrap(err, "converting credentials")
		}
		ret.Credentials = creds
	}

	if ret.PoolBalancerType == "" {
		ret.PoolBalancerType = params.PoolBalancerTypeRoundRobin
	}

	for idx, pool := range enterprise.Pools {
		ret.Pools[idx], err = s.sqlToCommonPool(pool)
		if err != nil {
			return params.Enterprise{}, errors.Wrap(err, "converting pool")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) sqlToCommonPool(pool Pool) (params.Pool, error) {
	ret := params.Pool{
		ID:             pool.ID.String(),
		ProviderName:   pool.ProviderName,
		MaxRunners:     pool.MaxRunners,
		MinIdleRunners: pool.MinIdleRunners,
		RunnerPrefix: params.RunnerPrefix{
			Prefix: pool.RunnerPrefix,
		},
		Image:                  pool.Image,
		Flavor:                 pool.Flavor,
		OSArch:                 pool.OSArch,
		OSType:                 pool.OSType,
		Enabled:                pool.Enabled,
		Tags:                   make([]params.Tag, len(pool.Tags)),
		Instances:              make([]params.Instance, len(pool.Instances)),
		RunnerBootstrapTimeout: pool.RunnerBootstrapTimeout,
		ExtraSpecs:             json.RawMessage(pool.ExtraSpecs),
		GitHubRunnerGroup:      pool.GitHubRunnerGroup,
		Priority:               pool.Priority,
		CreatedAt:              pool.CreatedAt,
		UpdatedAt:              pool.UpdatedAt,
	}

	var ep GithubEndpoint
	if pool.RepoID != nil {
		ret.RepoID = pool.RepoID.String()
		if pool.Repository.Owner != "" && pool.Repository.Name != "" {
			ret.RepoName = fmt.Sprintf("%s/%s", pool.Repository.Owner, pool.Repository.Name)
		}
		ep = pool.Repository.Endpoint
	}

	if pool.OrgID != nil && pool.Organization.Name != "" {
		ret.OrgID = pool.OrgID.String()
		ret.OrgName = pool.Organization.Name
		ep = pool.Organization.Endpoint
	}

	if pool.EnterpriseID != nil && pool.Enterprise.Name != "" {
		ret.EnterpriseID = pool.EnterpriseID.String()
		ret.EnterpriseName = pool.Enterprise.Name
		ep = pool.Enterprise.Endpoint
	}

	endpoint, err := s.sqlToCommonGithubEndpoint(ep)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "converting endpoint")
	}
	ret.Endpoint = endpoint

	for idx, val := range pool.Tags {
		ret.Tags[idx] = s.sqlToCommonTags(*val)
	}

	for idx, inst := range pool.Instances {
		ret.Instances[idx], err = s.sqlToParamsInstance(inst)
		if err != nil {
			return params.Pool{}, errors.Wrap(err, "converting instance")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) sqlToCommonScaleSet(scaleSet ScaleSet) (params.ScaleSet, error) {
	ret := params.ScaleSet{
		ID:            scaleSet.ID,
		CreatedAt:     scaleSet.CreatedAt,
		UpdatedAt:     scaleSet.UpdatedAt,
		ScaleSetID:    scaleSet.ScaleSetID,
		Name:          scaleSet.Name,
		DisableUpdate: scaleSet.DisableUpdate,

		ProviderName:   scaleSet.ProviderName,
		MaxRunners:     scaleSet.MaxRunners,
		MinIdleRunners: scaleSet.MinIdleRunners,
		RunnerPrefix: params.RunnerPrefix{
			Prefix: scaleSet.RunnerPrefix,
		},
		Image:                  scaleSet.Image,
		Flavor:                 scaleSet.Flavor,
		OSArch:                 scaleSet.OSArch,
		OSType:                 scaleSet.OSType,
		Enabled:                scaleSet.Enabled,
		Instances:              make([]params.Instance, len(scaleSet.Instances)),
		RunnerBootstrapTimeout: scaleSet.RunnerBootstrapTimeout,
		ExtraSpecs:             json.RawMessage(scaleSet.ExtraSpecs),
		GitHubRunnerGroup:      scaleSet.GitHubRunnerGroup,
		State:                  scaleSet.State,
		ExtendedState:          scaleSet.ExtendedState,
		LastMessageID:          scaleSet.LastMessageID,
		DesiredRunnerCount:     scaleSet.DesiredRunnerCount,
	}

	var ep GithubEndpoint
	if scaleSet.RepoID != nil {
		ret.RepoID = scaleSet.RepoID.String()
		if scaleSet.Repository.Owner != "" && scaleSet.Repository.Name != "" {
			ret.RepoName = fmt.Sprintf("%s/%s", scaleSet.Repository.Owner, scaleSet.Repository.Name)
		}
		ep = scaleSet.Repository.Endpoint
	}

	if scaleSet.OrgID != nil {
		ret.OrgID = scaleSet.OrgID.String()
		ret.OrgName = scaleSet.Organization.Name
		ep = scaleSet.Organization.Endpoint
	}

	if scaleSet.EnterpriseID != nil {
		ret.EnterpriseID = scaleSet.EnterpriseID.String()
		ret.EnterpriseName = scaleSet.Enterprise.Name
		ep = scaleSet.Enterprise.Endpoint
	}

	endpoint, err := s.sqlToCommonGithubEndpoint(ep)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "converting endpoint")
	}
	ret.Endpoint = endpoint

	for idx, inst := range scaleSet.Instances {
		ret.Instances[idx], err = s.sqlToParamsInstance(inst)
		if err != nil {
			return params.ScaleSet{}, errors.Wrap(err, "converting instance")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) sqlToCommonTags(tag Tag) params.Tag {
	return params.Tag{
		ID:   tag.ID.String(),
		Name: tag.Name,
	}
}

func (s *sqlDatabase) sqlToCommonRepository(repo Repository, detailed bool) (params.Repository, error) {
	if len(repo.WebhookSecret) == 0 {
		return params.Repository{}, errors.New("missing secret")
	}
	secret, err := util.Unseal(repo.WebhookSecret, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "decrypting secret")
	}
	endpoint, err := s.sqlToCommonGithubEndpoint(repo.Endpoint)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "converting endpoint")
	}
	ret := params.Repository{
		ID:               repo.ID.String(),
		Name:             repo.Name,
		Owner:            repo.Owner,
		CredentialsName:  repo.Credentials.Name,
		Pools:            make([]params.Pool, len(repo.Pools)),
		WebhookSecret:    string(secret),
		PoolBalancerType: repo.PoolBalancerType,
		CreatedAt:        repo.CreatedAt,
		UpdatedAt:        repo.UpdatedAt,
		Endpoint:         endpoint,
	}

	if repo.CredentialsID != nil && repo.GiteaCredentialsID != nil {
		return params.Repository{}, runnerErrors.NewConflictError("both gitea and github credentials are set for repo %s", repo.Name)
	}

	var forgeCreds params.ForgeCredentials
	if repo.CredentialsID != nil {
		ret.CredentialsID = *repo.CredentialsID
		forgeCreds, err = s.sqlToCommonForgeCredentials(repo.Credentials)
	}

	if repo.GiteaCredentialsID != nil {
		ret.CredentialsID = *repo.GiteaCredentialsID
		forgeCreds, err = s.sqlGiteaToCommonForgeCredentials(repo.GiteaCredentials)
	}

	if err != nil {
		return params.Repository{}, errors.Wrap(err, "converting credentials")
	}

	if len(repo.Events) > 0 {
		ret.Events = make([]params.EntityEvent, len(repo.Events))
		for idx, event := range repo.Events {
			ret.Events[idx] = params.EntityEvent{
				ID:         event.ID,
				Message:    event.Message,
				EventType:  event.EventType,
				EventLevel: event.EventLevel,
				CreatedAt:  event.CreatedAt,
			}
		}
	}

	if detailed {
		ret.Credentials = forgeCreds
		ret.CredentialsName = forgeCreds.Name
	}

	if ret.PoolBalancerType == "" {
		ret.PoolBalancerType = params.PoolBalancerTypeRoundRobin
	}

	for idx, pool := range repo.Pools {
		ret.Pools[idx], err = s.sqlToCommonPool(pool)
		if err != nil {
			return params.Repository{}, errors.Wrap(err, "converting pool")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) sqlToParamsUser(user User) params.User {
	return params.User{
		ID:         user.ID.String(),
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
		Email:      user.Email,
		Username:   user.Username,
		FullName:   user.FullName,
		Password:   user.Password,
		Enabled:    user.Enabled,
		IsAdmin:    user.IsAdmin,
		Generation: user.Generation,
	}
}

func (s *sqlDatabase) getOrCreateTag(tx *gorm.DB, tagName string) (Tag, error) {
	var tag Tag
	q := tx.Where("name = ? COLLATE NOCASE", tagName).First(&tag)
	if q.Error == nil {
		return tag, nil
	}
	if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return Tag{}, errors.Wrap(q.Error, "fetching tag from database")
	}
	newTag := Tag{
		Name: tagName,
	}

	if err := tx.Create(&newTag).Error; err != nil {
		return Tag{}, errors.Wrap(err, "creating tag")
	}
	return newTag, nil
}

func (s *sqlDatabase) updatePool(tx *gorm.DB, pool Pool, param params.UpdatePoolParams) (params.Pool, error) {
	if param.Enabled != nil && pool.Enabled != *param.Enabled {
		pool.Enabled = *param.Enabled
	}

	if param.Flavor != "" {
		pool.Flavor = param.Flavor
	}

	if param.Image != "" {
		pool.Image = param.Image
	}

	if param.Prefix != "" {
		pool.RunnerPrefix = param.Prefix
	}

	if param.MaxRunners != nil {
		pool.MaxRunners = *param.MaxRunners
	}

	if param.MinIdleRunners != nil {
		pool.MinIdleRunners = *param.MinIdleRunners
	}

	if param.OSArch != "" {
		pool.OSArch = param.OSArch
	}

	if param.OSType != "" {
		pool.OSType = param.OSType
	}

	if param.ExtraSpecs != nil {
		pool.ExtraSpecs = datatypes.JSON(param.ExtraSpecs)
	}

	if param.RunnerBootstrapTimeout != nil && *param.RunnerBootstrapTimeout > 0 {
		pool.RunnerBootstrapTimeout = *param.RunnerBootstrapTimeout
	}

	if param.GitHubRunnerGroup != nil {
		pool.GitHubRunnerGroup = *param.GitHubRunnerGroup
	}

	if param.Priority != nil {
		pool.Priority = *param.Priority
	}

	if q := tx.Save(&pool); q.Error != nil {
		return params.Pool{}, errors.Wrap(q.Error, "saving database entry")
	}

	tags := []Tag{}
	if len(param.Tags) > 0 {
		for _, val := range param.Tags {
			t, err := s.getOrCreateTag(tx, val)
			if err != nil {
				return params.Pool{}, errors.Wrap(err, "fetching tag")
			}
			tags = append(tags, t)
		}

		if err := tx.Model(&pool).Association("Tags").Replace(&tags); err != nil {
			return params.Pool{}, errors.Wrap(err, "replacing tags")
		}
	}

	return s.sqlToCommonPool(pool)
}

func (s *sqlDatabase) getPoolByID(tx *gorm.DB, poolID string, preload ...string) (Pool, error) {
	u, err := uuid.Parse(poolID)
	if err != nil {
		return Pool{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var pool Pool
	q := tx.Model(&Pool{})
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	q = q.Where("id = ?", u).First(&pool)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Pool{}, runnerErrors.ErrNotFound
		}
		return Pool{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return pool, nil
}

func (s *sqlDatabase) getScaleSetByID(tx *gorm.DB, scaleSetID uint, preload ...string) (ScaleSet, error) {
	var scaleSet ScaleSet
	q := tx.Model(&ScaleSet{})
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	q = q.Where("id = ?", scaleSetID).First(&scaleSet)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return ScaleSet{}, runnerErrors.ErrNotFound
		}
		return ScaleSet{}, errors.Wrap(q.Error, "fetching scale set from database")
	}
	return scaleSet, nil
}

func (s *sqlDatabase) hasGithubEntity(tx *gorm.DB, entityType params.ForgeEntityType, entityID string) error {
	u, err := uuid.Parse(entityID)
	if err != nil {
		return errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var q *gorm.DB
	switch entityType {
	case params.ForgeEntityTypeRepository:
		q = tx.Model(&Repository{}).Where("id = ?", u)
	case params.ForgeEntityTypeOrganization:
		q = tx.Model(&Organization{}).Where("id = ?", u)
	case params.ForgeEntityTypeEnterprise:
		q = tx.Model(&Enterprise{}).Where("id = ?", u)
	default:
		return errors.Wrap(runnerErrors.ErrBadRequest, "invalid entity type")
	}

	var entity interface{}
	if err := q.First(entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.Wrap(runnerErrors.ErrNotFound, "entity not found")
		}
		return errors.Wrap(err, "fetching entity from database")
	}
	return nil
}

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

func (s *sqlDatabase) sendNotify(entityType dbCommon.DatabaseEntityType, op dbCommon.OperationType, payload interface{}) error {
	if s.producer == nil {
		// no producer was registered. Not sending notifications.
		return nil
	}
	if payload == nil {
		return errors.New("missing payload")
	}
	message := dbCommon.ChangePayload{
		Operation:  op,
		Payload:    payload,
		EntityType: entityType,
	}
	return s.producer.Notify(message)
}

func (s *sqlDatabase) GetForgeEntity(_ context.Context, entityType params.ForgeEntityType, entityID string) (params.ForgeEntity, error) {
	var ghEntity params.EntityGetter
	var err error
	switch entityType {
	case params.ForgeEntityTypeEnterprise:
		ghEntity, err = s.GetEnterpriseByID(s.ctx, entityID)
	case params.ForgeEntityTypeOrganization:
		ghEntity, err = s.GetOrganizationByID(s.ctx, entityID)
	case params.ForgeEntityTypeRepository:
		ghEntity, err = s.GetRepositoryByID(s.ctx, entityID)
	default:
		return params.ForgeEntity{}, errors.Wrap(runnerErrors.ErrBadRequest, "invalid entity type")
	}
	if err != nil {
		return params.ForgeEntity{}, errors.Wrap(err, "failed to get ")
	}

	entity, err := ghEntity.GetEntity()
	if err != nil {
		return params.ForgeEntity{}, errors.Wrap(err, "failed to get entity")
	}
	return entity, nil
}

func (s *sqlDatabase) addRepositoryEvent(ctx context.Context, repoID string, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error {
	repo, err := s.getRepoByID(ctx, s.conn, repoID)
	if err != nil {
		return errors.Wrap(err, "updating instance")
	}

	msg := RepositoryEvent{
		Message:    statusMessage,
		EventType:  event,
		EventLevel: eventLevel,
	}

	if err := s.conn.Model(&repo).Association("Events").Append(&msg); err != nil {
		return errors.Wrap(err, "adding status message")
	}

	if maxEvents > 0 {
		var latestEvents []RepositoryEvent
		q := s.conn.Model(&RepositoryEvent{}).
			Limit(maxEvents).Order("id desc").
			Where("repo_id = ?", repo.ID).Find(&latestEvents)
		if q.Error != nil {
			return errors.Wrap(q.Error, "fetching latest events")
		}
		if len(latestEvents) == maxEvents {
			lastInList := latestEvents[len(latestEvents)-1]
			if err := s.conn.Where("repo_id = ? and id < ?", repo.ID, lastInList.ID).Unscoped().Delete(&RepositoryEvent{}).Error; err != nil {
				return errors.Wrap(err, "deleting old events")
			}
		}
	}
	return nil
}

func (s *sqlDatabase) addOrgEvent(ctx context.Context, orgID string, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error {
	org, err := s.getOrgByID(ctx, s.conn, orgID)
	if err != nil {
		return errors.Wrap(err, "updating instance")
	}

	msg := OrganizationEvent{
		Message:    statusMessage,
		EventType:  event,
		EventLevel: eventLevel,
	}

	if err := s.conn.Model(&org).Association("Events").Append(&msg); err != nil {
		return errors.Wrap(err, "adding status message")
	}

	if maxEvents > 0 {
		var latestEvents []OrganizationEvent
		q := s.conn.Model(&OrganizationEvent{}).
			Limit(maxEvents).Order("id desc").
			Where("org_id = ?", org.ID).Find(&latestEvents)
		if q.Error != nil {
			return errors.Wrap(q.Error, "fetching latest events")
		}
		if len(latestEvents) == maxEvents {
			lastInList := latestEvents[len(latestEvents)-1]
			if err := s.conn.Where("org_id = ? and id < ?", org.ID, lastInList.ID).Unscoped().Delete(&OrganizationEvent{}).Error; err != nil {
				return errors.Wrap(err, "deleting old events")
			}
		}
	}
	return nil
}

func (s *sqlDatabase) addEnterpriseEvent(ctx context.Context, entID string, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error {
	ent, err := s.getEnterpriseByID(ctx, s.conn, entID)
	if err != nil {
		return errors.Wrap(err, "updating instance")
	}

	msg := EnterpriseEvent{
		Message:    statusMessage,
		EventType:  event,
		EventLevel: eventLevel,
	}

	if err := s.conn.Model(&ent).Association("Events").Append(&msg); err != nil {
		return errors.Wrap(err, "adding status message")
	}

	if maxEvents > 0 {
		var latestEvents []EnterpriseEvent
		q := s.conn.Model(&EnterpriseEvent{}).
			Limit(maxEvents).Order("id desc").
			Where("enterprise_id = ?", ent.ID).Find(&latestEvents)
		if q.Error != nil {
			return errors.Wrap(q.Error, "fetching latest events")
		}
		if len(latestEvents) == maxEvents {
			lastInList := latestEvents[len(latestEvents)-1]
			if err := s.conn.Where("enterprise_id = ? and id < ?", ent.ID, lastInList.ID).Unscoped().Delete(&EnterpriseEvent{}).Error; err != nil {
				return errors.Wrap(err, "deleting old events")
			}
		}
	}

	return nil
}

func (s *sqlDatabase) AddEntityEvent(ctx context.Context, entity params.ForgeEntity, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error {
	if maxEvents == 0 {
		return errors.Wrap(runnerErrors.ErrBadRequest, "max events cannot be 0")
	}

	switch entity.EntityType {
	case params.ForgeEntityTypeRepository:
		return s.addRepositoryEvent(ctx, entity.ID, event, eventLevel, statusMessage, maxEvents)
	case params.ForgeEntityTypeOrganization:
		return s.addOrgEvent(ctx, entity.ID, event, eventLevel, statusMessage, maxEvents)
	case params.ForgeEntityTypeEnterprise:
		return s.addEnterpriseEvent(ctx, entity.ID, event, eventLevel, statusMessage, maxEvents)
	default:
		return errors.Wrap(runnerErrors.ErrBadRequest, "invalid entity type")
	}
}

func (s *sqlDatabase) sqlToCommonForgeCredentials(creds GithubCredentials) (params.ForgeCredentials, error) {
	if len(creds.Payload) == 0 {
		return params.ForgeCredentials{}, errors.New("empty credentials payload")
	}
	data, err := util.Unseal(creds.Payload, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "unsealing credentials")
	}

	ep, err := s.sqlToCommonGithubEndpoint(creds.Endpoint)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "converting github endpoint")
	}

	commonCreds := params.ForgeCredentials{
		ID:                 creds.ID,
		Name:               creds.Name,
		Description:        creds.Description,
		APIBaseURL:         creds.Endpoint.APIBaseURL,
		BaseURL:            creds.Endpoint.BaseURL,
		UploadBaseURL:      creds.Endpoint.UploadBaseURL,
		CABundle:           creds.Endpoint.CACertBundle,
		AuthType:           creds.AuthType,
		CreatedAt:          creds.CreatedAt,
		UpdatedAt:          creds.UpdatedAt,
		ForgeType:          creds.Endpoint.EndpointType,
		Endpoint:           ep,
		CredentialsPayload: data,
	}

	for _, repo := range creds.Repositories {
		commonRepo, err := s.sqlToCommonRepository(repo, false)
		if err != nil {
			return params.ForgeCredentials{}, errors.Wrap(err, "converting github repository")
		}
		commonCreds.Repositories = append(commonCreds.Repositories, commonRepo)
	}

	for _, org := range creds.Organizations {
		commonOrg, err := s.sqlToCommonOrganization(org, false)
		if err != nil {
			return params.ForgeCredentials{}, errors.Wrap(err, "converting github organization")
		}
		commonCreds.Organizations = append(commonCreds.Organizations, commonOrg)
	}

	for _, ent := range creds.Enterprises {
		commonEnt, err := s.sqlToCommonEnterprise(ent, false)
		if err != nil {
			return params.ForgeCredentials{}, errors.Wrapf(err, "converting github enterprise: %s", ent.Name)
		}
		commonCreds.Enterprises = append(commonCreds.Enterprises, commonEnt)
	}

	return commonCreds, nil
}

func (s *sqlDatabase) sqlGiteaToCommonForgeCredentials(creds GiteaCredentials) (params.ForgeCredentials, error) {
	if len(creds.Payload) == 0 {
		return params.ForgeCredentials{}, errors.New("empty credentials payload")
	}
	data, err := util.Unseal(creds.Payload, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "unsealing credentials")
	}

	ep, err := s.sqlToCommonGithubEndpoint(creds.Endpoint)
	if err != nil {
		return params.ForgeCredentials{}, errors.Wrap(err, "converting github endpoint")
	}

	commonCreds := params.ForgeCredentials{
		ID:                 creds.ID,
		Name:               creds.Name,
		Description:        creds.Description,
		APIBaseURL:         creds.Endpoint.APIBaseURL,
		BaseURL:            creds.Endpoint.BaseURL,
		CABundle:           creds.Endpoint.CACertBundle,
		AuthType:           creds.AuthType,
		CreatedAt:          creds.CreatedAt,
		UpdatedAt:          creds.UpdatedAt,
		ForgeType:          creds.Endpoint.EndpointType,
		Endpoint:           ep,
		CredentialsPayload: data,
	}

	for _, repo := range creds.Repositories {
		commonRepo, err := s.sqlToCommonRepository(repo, false)
		if err != nil {
			return params.ForgeCredentials{}, errors.Wrap(err, "converting github repository")
		}
		commonCreds.Repositories = append(commonCreds.Repositories, commonRepo)
	}

	for _, org := range creds.Organizations {
		commonOrg, err := s.sqlToCommonOrganization(org, false)
		if err != nil {
			return params.ForgeCredentials{}, errors.Wrap(err, "converting github organization")
		}
		commonCreds.Organizations = append(commonCreds.Organizations, commonOrg)
	}

	return commonCreds, nil
}

func (s *sqlDatabase) sqlToCommonGithubEndpoint(ep GithubEndpoint) (params.ForgeEndpoint, error) {
	return params.ForgeEndpoint{
		Name:          ep.Name,
		Description:   ep.Description,
		APIBaseURL:    ep.APIBaseURL,
		BaseURL:       ep.BaseURL,
		UploadBaseURL: ep.UploadBaseURL,
		CACertBundle:  ep.CACertBundle,
		CreatedAt:     ep.CreatedAt,
		EndpointType:  ep.EndpointType,
		UpdatedAt:     ep.UpdatedAt,
	}, nil
}

func getUIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID := auth.UserID(ctx)
	if userID == "" {
		return uuid.Nil, errors.Wrap(runnerErrors.ErrUnauthorized, "getting UID from context")
	}

	asUUID, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, errors.Wrap(runnerErrors.ErrUnauthorized, "parsing UID from context")
	}
	return asUUID, nil
}
