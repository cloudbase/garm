// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package provider

import (
	"fmt"

	"github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

type providerHelper interface {
	SetInstanceStatus(instanceName string, status commonParams.InstanceStatus, providerFault []byte) error
	InstanceTokenGetter() auth.InstanceTokenGetter
	updateArgsFromProviderInstance(instanceName string, providerInstance commonParams.ProviderInstance) (params.Instance, error)
	GetControllerInfo() (params.ControllerInfo, error)
	GetGithubEntity(entity params.ForgeEntity) (params.ForgeEntity, error)
}

func (p *Provider) updateArgsFromProviderInstance(instanceName string, providerInstance commonParams.ProviderInstance) (params.Instance, error) {
	updateParams := params.UpdateInstanceParams{
		ProviderID:    providerInstance.ProviderID,
		OSName:        providerInstance.OSName,
		OSVersion:     providerInstance.OSVersion,
		Addresses:     providerInstance.Addresses,
		Status:        providerInstance.Status,
		ProviderFault: providerInstance.ProviderFault,
	}

	updated, err := p.store.UpdateInstance(p.ctx, instanceName, updateParams)
	if err != nil {
		return params.Instance{}, fmt.Errorf("updating instance %s: %w", instanceName, err)
	}
	return updated, nil
}

func (p *Provider) GetControllerInfo() (params.ControllerInfo, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	info, err := p.store.ControllerInfo()
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("getting controller info: %w", err)
	}

	return info, nil
}

func (p *Provider) SetInstanceStatus(instanceName string, status commonParams.InstanceStatus, providerFault []byte) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	if _, ok := p.runners[instanceName]; !ok {
		return errors.ErrNotFound
	}

	updateParams := params.UpdateInstanceParams{
		Status:        status,
		ProviderFault: providerFault,
	}

	_, err := p.store.UpdateInstance(p.ctx, instanceName, updateParams)
	if err != nil {
		return fmt.Errorf("updating instance %s: %w", instanceName, err)
	}

	return nil
}

func (p *Provider) InstanceTokenGetter() auth.InstanceTokenGetter {
	return p.tokenGetter
}

func (p *Provider) GetGithubEntity(entity params.ForgeEntity) (params.ForgeEntity, error) {
	ghEntity, err := p.store.GetForgeEntity(p.ctx, entity.EntityType, entity.ID)
	if err != nil {
		return params.ForgeEntity{}, fmt.Errorf("getting github entity: %w", err)
	}

	return ghEntity, nil
}
