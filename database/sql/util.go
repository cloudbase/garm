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
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/auth"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/appdefaults"
)

func (s *sqlDatabase) sqlToParamsInstance(instance Instance) (params.Instance, error) {
	var id string
	if instance.ProviderID != nil {
		id = *instance.ProviderID
	}

	var labels []string
	if len(instance.AditionalLabels) > 0 {
		if err := json.Unmarshal(instance.AditionalLabels, &labels); err != nil {
			return params.Instance{}, fmt.Errorf("error unmarshalling labels: %w", err)
		}
	}

	var jitConfig map[string]string
	if len(instance.JitConfiguration) > 0 {
		if err := s.unsealAndUnmarshal(instance.JitConfiguration, &jitConfig); err != nil {
			return params.Instance{}, fmt.Errorf("error unmarshalling jit configuration: %w", err)
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
		Heartbeat:         instance.Heartbeat,
	}

	if len(instance.Capabilities) > 0 {
		var caps params.AgentCapabilities
		if err := json.Unmarshal(instance.Capabilities, &caps); err == nil {
			ret.Capabilities = caps
		} else {
			slog.ErrorContext(s.ctx, "failed to unmarshal capabilities", "instance_name", instance.Name, "error", err)
		}
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
			return params.Instance{}, fmt.Errorf("error converting job: %w", err)
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
		return params.Organization{}, fmt.Errorf("error decrypting secret: %w", err)
	}

	endpoint, err := s.sqlToCommonGithubEndpoint(org.Endpoint)
	if err != nil {
		return params.Organization{}, fmt.Errorf("error converting endpoint: %w", err)
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
		AgentMode:        org.AgentMode,
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
		return params.Organization{}, fmt.Errorf("error converting credentials: %w", err)
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
			return params.Organization{}, fmt.Errorf("error converting pool: %w", err)
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
		return params.Enterprise{}, fmt.Errorf("error decrypting secret: %w", err)
	}

	endpoint, err := s.sqlToCommonGithubEndpoint(enterprise.Endpoint)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error converting endpoint: %w", err)
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
		AgentMode:        enterprise.AgentMode,
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
			return params.Enterprise{}, fmt.Errorf("error converting credentials: %w", err)
		}
		ret.Credentials = creds
	}

	if ret.PoolBalancerType == "" {
		ret.PoolBalancerType = params.PoolBalancerTypeRoundRobin
	}

	for idx, pool := range enterprise.Pools {
		ret.Pools[idx], err = s.sqlToCommonPool(pool)
		if err != nil {
			return params.Enterprise{}, fmt.Errorf("error converting pool: %w", err)
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
		EnableShell:            pool.EnableShell,
	}

	if pool.TemplateID != nil && *pool.TemplateID != 0 {
		ret.TemplateID = *pool.TemplateID
		ret.TemplateName = pool.Template.Name
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
		return params.Pool{}, fmt.Errorf("error converting endpoint: %w", err)
	}
	ret.Endpoint = endpoint

	for idx, val := range pool.Tags {
		ret.Tags[idx] = s.sqlToCommonTags(*val)
	}

	for idx, inst := range pool.Instances {
		ret.Instances[idx], err = s.sqlToParamsInstance(inst)
		if err != nil {
			return params.Pool{}, fmt.Errorf("error converting instance: %w", err)
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
		EnableShell:            scaleSet.EnableShell,
	}

	if scaleSet.TemplateID != nil && *scaleSet.TemplateID != 0 {
		ret.TemplateID = *scaleSet.TemplateID
		ret.TemplateName = scaleSet.Template.Name
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
		return params.ScaleSet{}, fmt.Errorf("error converting endpoint: %w", err)
	}
	ret.Endpoint = endpoint

	for idx, inst := range scaleSet.Instances {
		ret.Instances[idx], err = s.sqlToParamsInstance(inst)
		if err != nil {
			return params.ScaleSet{}, fmt.Errorf("error converting instance: %w", err)
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
		return params.Repository{}, fmt.Errorf("error decrypting secret: %w", err)
	}
	endpoint, err := s.sqlToCommonGithubEndpoint(repo.Endpoint)
	if err != nil {
		return params.Repository{}, fmt.Errorf("error converting endpoint: %w", err)
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
		AgentMode:        repo.AgentMode,
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
		return params.Repository{}, fmt.Errorf("error converting credentials: %w", err)
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
			return params.Repository{}, fmt.Errorf("error converting pool: %w", err)
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
		return Tag{}, fmt.Errorf("error fetching tag from database: %w", q.Error)
	}
	newTag := Tag{
		Name: tagName,
	}

	if err := tx.Create(&newTag).Error; err != nil {
		return Tag{}, fmt.Errorf("error creating tag: %w", err)
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

	if param.EnableShell != nil {
		pool.EnableShell = *param.EnableShell
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

	if param.TemplateID != nil {
		pool.TemplateID = param.TemplateID
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
		return params.Pool{}, fmt.Errorf("error saving database entry: %w", q.Error)
	}

	tags := []Tag{}
	if len(param.Tags) > 0 {
		for _, val := range param.Tags {
			t, err := s.getOrCreateTag(tx, val)
			if err != nil {
				return params.Pool{}, fmt.Errorf("error fetching tag: %w", err)
			}
			tags = append(tags, t)
		}

		if err := tx.Model(&pool).Association("Tags").Replace(&tags); err != nil {
			return params.Pool{}, fmt.Errorf("error replacing tags: %w", err)
		}
	}

	return s.sqlToCommonPool(pool)
}

func (s *sqlDatabase) getPoolByID(tx *gorm.DB, poolID string, preload ...string) (Pool, error) {
	u, err := uuid.Parse(poolID)
	if err != nil {
		return Pool{}, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
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
		return Pool{}, fmt.Errorf("error fetching org from database: %w", q.Error)
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
		return ScaleSet{}, fmt.Errorf("error fetching scale set from database: %w", q.Error)
	}
	return scaleSet, nil
}

func (s *sqlDatabase) hasGithubEntity(tx *gorm.DB, entityType params.ForgeEntityType, entityID string) error {
	u, err := uuid.Parse(entityID)
	if err != nil {
		return fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
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
		return fmt.Errorf("error invalid entity type: %w", runnerErrors.ErrBadRequest)
	}

	var entity interface{}
	if err := q.First(entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("error entity not found: %w", runnerErrors.ErrNotFound)
		}
		return fmt.Errorf("error fetching entity from database: %w", err)
	}
	return nil
}

func (s *sqlDatabase) marshalAndSeal(data interface{}) ([]byte, error) {
	enc, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error marshalling data: %w", err)
	}
	return util.Seal(enc, []byte(s.cfg.Passphrase))
}

func (s *sqlDatabase) unsealAndUnmarshal(data []byte, target interface{}) error {
	decrypted, err := util.Unseal(data, []byte(s.cfg.Passphrase))
	if err != nil {
		return fmt.Errorf("error decrypting data: %w", err)
	}
	if err := json.Unmarshal(decrypted, target); err != nil {
		return fmt.Errorf("error unmarshalling data: %w", err)
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
		return params.ForgeEntity{}, fmt.Errorf("error invalid entity type: %w", runnerErrors.ErrBadRequest)
	}
	if err != nil {
		return params.ForgeEntity{}, fmt.Errorf("error failed to get entity from db: %w", err)
	}

	entity, err := ghEntity.GetEntity()
	if err != nil {
		return params.ForgeEntity{}, fmt.Errorf("error failed to get entity: %w", err)
	}
	return entity, nil
}

func (s *sqlDatabase) addRepositoryEvent(ctx context.Context, repoID string, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error {
	repo, err := s.getRepoByID(ctx, s.conn, repoID)
	if err != nil {
		return fmt.Errorf("error updating instance: %w", err)
	}

	msg := RepositoryEvent{
		Message:    statusMessage,
		EventType:  event,
		EventLevel: eventLevel,
		RepoID:     repo.ID,
	}

	// Use Create instead of Association.Append to avoid loading all existing events
	if err := s.conn.Create(&msg).Error; err != nil {
		return fmt.Errorf("error adding status message: %w", err)
	}

	if maxEvents > 0 {
		var count int64
		if err := s.conn.Model(&RepositoryEvent{}).Where("repo_id = ?", repo.ID).Count(&count).Error; err != nil {
			return fmt.Errorf("error counting events: %w", err)
		}

		if count > int64(maxEvents) {
			// Get the ID of the Nth most recent event
			var cutoffEvent RepositoryEvent
			if err := s.conn.Model(&RepositoryEvent{}).
				Select("id").
				Where("repo_id = ?", repo.ID).
				Order("id desc").
				Offset(maxEvents - 1).
				Limit(1).
				First(&cutoffEvent).Error; err != nil {
				return fmt.Errorf("error finding cutoff event: %w", err)
			}

			// Delete all events older than the cutoff
			if err := s.conn.Where("repo_id = ? and id < ?", repo.ID, cutoffEvent.ID).Unscoped().Delete(&RepositoryEvent{}).Error; err != nil {
				return fmt.Errorf("error deleting old events: %w", err)
			}
		}
	}
	return nil
}

func (s *sqlDatabase) addOrgEvent(ctx context.Context, orgID string, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error {
	org, err := s.getOrgByID(ctx, s.conn, orgID)
	if err != nil {
		return fmt.Errorf("error updating instance: %w", err)
	}

	msg := OrganizationEvent{
		Message:    statusMessage,
		EventType:  event,
		EventLevel: eventLevel,
		OrgID:      org.ID,
	}

	// Use Create instead of Association.Append to avoid loading all existing events
	if err := s.conn.Create(&msg).Error; err != nil {
		return fmt.Errorf("error adding status message: %w", err)
	}

	if maxEvents > 0 {
		var count int64
		if err := s.conn.Model(&OrganizationEvent{}).Where("org_id = ?", org.ID).Count(&count).Error; err != nil {
			return fmt.Errorf("error counting events: %w", err)
		}

		if count > int64(maxEvents) {
			// Get the ID of the Nth most recent event
			var cutoffEvent OrganizationEvent
			if err := s.conn.Model(&OrganizationEvent{}).
				Select("id").
				Where("org_id = ?", org.ID).
				Order("id desc").
				Offset(maxEvents - 1).
				Limit(1).
				First(&cutoffEvent).Error; err != nil {
				return fmt.Errorf("error finding cutoff event: %w", err)
			}

			// Delete all events older than the cutoff
			if err := s.conn.Where("org_id = ? and id < ?", org.ID, cutoffEvent.ID).Unscoped().Delete(&OrganizationEvent{}).Error; err != nil {
				return fmt.Errorf("error deleting old events: %w", err)
			}
		}
	}
	return nil
}

func (s *sqlDatabase) addEnterpriseEvent(ctx context.Context, entID string, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error {
	ent, err := s.getEnterpriseByID(ctx, s.conn, entID)
	if err != nil {
		return fmt.Errorf("error updating instance: %w", err)
	}

	msg := EnterpriseEvent{
		Message:      statusMessage,
		EventType:    event,
		EventLevel:   eventLevel,
		EnterpriseID: ent.ID,
	}

	// Use Create instead of Association.Append to avoid loading all existing events
	if err := s.conn.Create(&msg).Error; err != nil {
		return fmt.Errorf("error adding status message: %w", err)
	}

	if maxEvents > 0 {
		var count int64
		if err := s.conn.Model(&EnterpriseEvent{}).Where("enterprise_id = ?", ent.ID).Count(&count).Error; err != nil {
			return fmt.Errorf("error counting events: %w", err)
		}

		if count > int64(maxEvents) {
			// Get the ID of the Nth most recent event
			var cutoffEvent EnterpriseEvent
			if err := s.conn.Model(&EnterpriseEvent{}).
				Select("id").
				Where("enterprise_id = ?", ent.ID).
				Order("id desc").
				Offset(maxEvents - 1).
				Limit(1).
				First(&cutoffEvent).Error; err != nil {
				return fmt.Errorf("error finding cutoff event: %w", err)
			}

			// Delete all events older than the cutoff
			if err := s.conn.Where("enterprise_id = ? and id < ?", ent.ID, cutoffEvent.ID).Unscoped().Delete(&EnterpriseEvent{}).Error; err != nil {
				return fmt.Errorf("error deleting old events: %w", err)
			}
		}
	}

	return nil
}

func (s *sqlDatabase) AddEntityEvent(ctx context.Context, entity params.ForgeEntity, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error {
	if maxEvents == 0 {
		return fmt.Errorf("max events cannot be 0: %w", runnerErrors.ErrBadRequest)
	}

	switch entity.EntityType {
	case params.ForgeEntityTypeRepository:
		return s.addRepositoryEvent(ctx, entity.ID, event, eventLevel, statusMessage, maxEvents)
	case params.ForgeEntityTypeOrganization:
		return s.addOrgEvent(ctx, entity.ID, event, eventLevel, statusMessage, maxEvents)
	case params.ForgeEntityTypeEnterprise:
		return s.addEnterpriseEvent(ctx, entity.ID, event, eventLevel, statusMessage, maxEvents)
	default:
		return fmt.Errorf("invalid entity type: %w", runnerErrors.ErrBadRequest)
	}
}

func (s *sqlDatabase) sqlToCommonForgeCredentials(creds GithubCredentials) (params.ForgeCredentials, error) {
	if len(creds.Payload) == 0 {
		return params.ForgeCredentials{}, errors.New("empty credentials payload")
	}
	data, err := util.Unseal(creds.Payload, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error unsealing credentials: %w", err)
	}

	ep, err := s.sqlToCommonGithubEndpoint(creds.Endpoint)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error converting github endpoint: %w", err)
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
			return params.ForgeCredentials{}, fmt.Errorf("error converting github repository: %w", err)
		}
		commonCreds.Repositories = append(commonCreds.Repositories, commonRepo)
	}

	for _, org := range creds.Organizations {
		commonOrg, err := s.sqlToCommonOrganization(org, false)
		if err != nil {
			return params.ForgeCredentials{}, fmt.Errorf("error converting github organization: %w", err)
		}
		commonCreds.Organizations = append(commonCreds.Organizations, commonOrg)
	}

	for _, ent := range creds.Enterprises {
		commonEnt, err := s.sqlToCommonEnterprise(ent, false)
		if err != nil {
			return params.ForgeCredentials{}, fmt.Errorf("error converting github enterprise %s: %w", ent.Name, err)
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
		return params.ForgeCredentials{}, fmt.Errorf("error unsealing credentials: %w", err)
	}

	ep, err := s.sqlToCommonGithubEndpoint(creds.Endpoint)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("error converting github endpoint: %w", err)
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
			return params.ForgeCredentials{}, fmt.Errorf("error converting github repository: %w", err)
		}
		commonCreds.Repositories = append(commonCreds.Repositories, commonRepo)
	}

	for _, org := range creds.Organizations {
		commonOrg, err := s.sqlToCommonOrganization(org, false)
		if err != nil {
			return params.ForgeCredentials{}, fmt.Errorf("error converting github organization: %w", err)
		}
		commonCreds.Organizations = append(commonCreds.Organizations, commonOrg)
	}

	return commonCreds, nil
}

func (s *sqlDatabase) sqlToCommonGithubEndpoint(ep GithubEndpoint) (params.ForgeEndpoint, error) {
	ret := params.ForgeEndpoint{
		Name:          ep.Name,
		Description:   ep.Description,
		APIBaseURL:    ep.APIBaseURL,
		BaseURL:       ep.BaseURL,
		UploadBaseURL: ep.UploadBaseURL,
		CACertBundle:  ep.CACertBundle,
		CreatedAt:     ep.CreatedAt,
		EndpointType:  ep.EndpointType,
		UpdatedAt:     ep.UpdatedAt,
	}
	if ep.EndpointType == params.GiteaEndpointType {
		ret.UseInternalToolsMetadata = &ep.UseInternalToolsMetadata
		if ep.ToolsMetadataURL == "" {
			ret.ToolsMetadataURL = appdefaults.GiteaRunnerReleasesURL
		} else {
			ret.ToolsMetadataURL = ep.ToolsMetadataURL
		}
	}
	return ret, nil
}

func getUIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID := auth.UserID(ctx)
	if userID == "" {
		return uuid.Nil, fmt.Errorf("error getting UID from context: %w", runnerErrors.ErrUnauthorized)
	}

	asUUID, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("error parsing UID from context: %w", runnerErrors.ErrUnauthorized)
	}
	return asUUID, nil
}

func (s *sqlDatabase) sqlToParamTemplate(template Template) (params.Template, error) {
	var data []byte
	if len(template.Data) > 0 {
		if err := s.unsealAndUnmarshal(template.Data, &data); err != nil {
			return params.Template{}, fmt.Errorf("error unsealing template: %w", err)
		}
	}

	owner := params.SystemUser
	if template.UserID != nil {
		owner = template.User.Username
	}
	return params.Template{
		ID:          template.ID,
		CreatedAt:   template.CreatedAt,
		UpdatedAt:   template.UpdatedAt,
		Name:        template.Name,
		Description: template.Description,
		Data:        data,
		ForgeType:   template.ForgeType,
		Owner:       owner,
		OSType:      template.OSType,
	}, nil
}
