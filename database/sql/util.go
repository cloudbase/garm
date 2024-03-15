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
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
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
		PoolID:            instance.PoolID.String(),
		CallbackURL:       instance.CallbackURL,
		MetadataURL:       instance.MetadataURL,
		StatusMessages:    []params.StatusMessage{},
		CreateAttempt:     instance.CreateAttempt,
		UpdatedAt:         instance.UpdatedAt,
		TokenFetched:      instance.TokenFetched,
		JitConfiguration:  jitConfig,
		GitHubRunnerGroup: instance.GitHubRunnerGroup,
		AditionalLabels:   labels,
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

func (s *sqlDatabase) sqlToCommonOrganization(org Organization) (params.Organization, error) {
	if len(org.WebhookSecret) == 0 {
		return params.Organization{}, errors.New("missing secret")
	}
	secret, err := util.Unseal(org.WebhookSecret, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "decrypting secret")
	}

	ret := params.Organization{
		ID:               org.ID.String(),
		Name:             org.Name,
		CredentialsName:  org.CredentialsName,
		Pools:            make([]params.Pool, len(org.Pools)),
		WebhookSecret:    string(secret),
		PoolBalancerType: org.PoolBalancerType,
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

func (s *sqlDatabase) sqlToCommonEnterprise(enterprise Enterprise) (params.Enterprise, error) {
	if len(enterprise.WebhookSecret) == 0 {
		return params.Enterprise{}, errors.New("missing secret")
	}
	secret, err := util.Unseal(enterprise.WebhookSecret, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "decrypting secret")
	}

	ret := params.Enterprise{
		ID:               enterprise.ID.String(),
		Name:             enterprise.Name,
		CredentialsName:  enterprise.CredentialsName,
		Pools:            make([]params.Pool, len(enterprise.Pools)),
		WebhookSecret:    string(secret),
		PoolBalancerType: enterprise.PoolBalancerType,
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
	}

	if pool.RepoID != nil {
		ret.RepoID = pool.RepoID.String()
		if pool.Repository.Owner != "" && pool.Repository.Name != "" {
			ret.RepoName = fmt.Sprintf("%s/%s", pool.Repository.Owner, pool.Repository.Name)
		}
	}

	if pool.OrgID != nil && pool.Organization.Name != "" {
		ret.OrgID = pool.OrgID.String()
		ret.OrgName = pool.Organization.Name
	}

	if pool.EnterpriseID != nil && pool.Enterprise.Name != "" {
		ret.EnterpriseID = pool.EnterpriseID.String()
		ret.EnterpriseName = pool.Enterprise.Name
	}

	for idx, val := range pool.Tags {
		ret.Tags[idx] = s.sqlToCommonTags(*val)
	}

	var err error
	for idx, inst := range pool.Instances {
		ret.Instances[idx], err = s.sqlToParamsInstance(inst)
		if err != nil {
			return params.Pool{}, errors.Wrap(err, "converting instance")
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

func (s *sqlDatabase) sqlToCommonRepository(repo Repository) (params.Repository, error) {
	if len(repo.WebhookSecret) == 0 {
		return params.Repository{}, errors.New("missing secret")
	}
	secret, err := util.Unseal(repo.WebhookSecret, []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "decrypting secret")
	}

	ret := params.Repository{
		ID:               repo.ID.String(),
		Name:             repo.Name,
		Owner:            repo.Owner,
		CredentialsName:  repo.CredentialsName,
		Pools:            make([]params.Pool, len(repo.Pools)),
		WebhookSecret:    string(secret),
		PoolBalancerType: repo.PoolBalancerType,
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
		ID:        user.ID.String(),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		Password:  user.Password,
		Enabled:   user.Enabled,
		IsAdmin:   user.IsAdmin,
	}
}

func (s *sqlDatabase) getOrCreateTag(tagName string) (Tag, error) {
	var tag Tag
	q := s.conn.Where("name = ?", tagName).First(&tag)
	if q.Error == nil {
		return tag, nil
	}
	if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return Tag{}, errors.Wrap(q.Error, "fetching tag from database")
	}
	newTag := Tag{
		Name: tagName,
	}

	q = s.conn.Create(&newTag)
	if q.Error != nil {
		return Tag{}, errors.Wrap(q.Error, "creating tag")
	}
	return newTag, nil
}

func (s *sqlDatabase) updatePool(pool Pool, param params.UpdatePoolParams) (params.Pool, error) {
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

	if q := s.conn.Save(&pool); q.Error != nil {
		return params.Pool{}, errors.Wrap(q.Error, "saving database entry")
	}

	tags := []Tag{}
	if param.Tags != nil && len(param.Tags) > 0 {
		for _, val := range param.Tags {
			t, err := s.getOrCreateTag(val)
			if err != nil {
				return params.Pool{}, errors.Wrap(err, "fetching tag")
			}
			tags = append(tags, t)
		}

		if err := s.conn.Model(&pool).Association("Tags").Replace(&tags); err != nil {
			return params.Pool{}, errors.Wrap(err, "replacing tags")
		}
	}

	return s.sqlToCommonPool(pool)
}
