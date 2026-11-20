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
	"context"
	"fmt"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

// providerHelper is the instance manager's view of the store. Every method
// that reaches the database takes a context. The caller owns the deadline,
// because it is the caller that is stranded if the query never returns.
type providerHelper interface {
	SetInstanceStatus(ctx context.Context, instanceName string, status commonParams.InstanceStatus, providerFault []byte, force bool) error
	InstanceTokenGetter() auth.InstanceTokenGetter
	persistProviderInstanceState(ctx context.Context, instanceName string, providerInstance commonParams.ProviderInstance) (params.Instance, error)
	GetControllerInfo(ctx context.Context) (params.ControllerInfo, error)
	GetGithubEntity(ctx context.Context, entity params.ForgeEntity) (params.ForgeEntity, error)
}

// persistProviderInstanceState commits the provider-reported state of an
// instance (provider ID, addresses, OS info, status) to the store and returns
// the full post-commit instance as re-read from the database.
func (p *Provider) persistProviderInstanceState(ctx context.Context, instanceName string, providerInstance commonParams.ProviderInstance) (params.Instance, error) {
	updateParams := params.UpdateInstanceParams{
		ProviderID:    providerInstance.ProviderID,
		OSName:        providerInstance.OSName,
		OSVersion:     providerInstance.OSVersion,
		Addresses:     providerInstance.Addresses,
		Status:        providerInstance.Status,
		ProviderFault: providerInstance.ProviderFault,
	}

	updated, err := p.store.UpdateInstance(ctx, instanceName, updateParams)
	if err != nil {
		return params.Instance{}, fmt.Errorf("updating instance %s: %w", instanceName, err)
	}
	return updated, nil
}

func (p *Provider) GetControllerInfo(ctx context.Context) (params.ControllerInfo, error) {
	info, err := p.store.ControllerInfo(ctx)
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("getting controller info: %w", err)
	}

	return info, nil
}

func (p *Provider) SetInstanceStatus(ctx context.Context, instanceName string, status commonParams.InstanceStatus, providerFault []byte, force bool) error {
	updateParams := params.UpdateInstanceParams{
		Status:        status,
		ProviderFault: providerFault,
	}

	var err error
	switch force {
	case true:
		_, err = p.store.ForceUpdateInstance(ctx, instanceName, updateParams)
	default:
		_, err = p.store.UpdateInstance(ctx, instanceName, updateParams)
	}
	if err != nil {
		return fmt.Errorf("updating instance %s: %w", instanceName, err)
	}

	return nil
}

func (p *Provider) InstanceTokenGetter() auth.InstanceTokenGetter {
	return p.tokenGetter
}

func (p *Provider) GetGithubEntity(ctx context.Context, entity params.ForgeEntity) (params.ForgeEntity, error) {
	ghEntity, err := p.store.GetForgeEntity(ctx, entity.EntityType, entity.ID)
	if err != nil {
		return params.ForgeEntity{}, fmt.Errorf("getting github entity: %w", err)
	}

	return ghEntity, nil
}

// beginAdmittedCreate consumes the durable one-shot create authorization for
// enrolled providers. A missing optional interface preserves legacy stores.
func (p *Provider) beginAdmittedCreate(ctx context.Context, provider, owner, name string, shape params.AdmissionCreateShape) (params.AdmissionReservation, bool, error) {
	store, ok := p.store.(interface {
		BeginAdmittedCreate(context.Context, string, string, string, params.AdmissionCreateShape) (params.AdmissionReservation, bool, error)
	})
	if !ok {
		return params.AdmissionReservation{}, false, nil
	}
	return store.BeginAdmittedCreate(ctx, provider, owner, name, shape)
}

func (p *Provider) admissionEnrolled(ctx context.Context, provider string) (bool, error) {
	store, ok := p.store.(interface {
		AdmissionForProvider(context.Context, string) (params.AdmissionConfig, bool, error)
	})
	if !ok {
		return false, nil
	}
	_, enrolled, err := store.AdmissionForProvider(ctx, provider)
	return enrolled, err
}

func (p *Provider) markAdmissionPresent(ctx context.Context, provider, name string) error {
	store, ok := p.store.(interface {
		MarkAdmissionPresent(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("admission store cannot mark provider presence")
	}
	return store.MarkAdmissionPresent(ctx, provider, name)
}

func (p *Provider) beginAdmissionDelete(ctx context.Context, provider, name string) error {
	store, ok := p.store.(interface {
		BeginAdmissionDelete(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("admission store cannot authorize deletion")
	}
	return store.BeginAdmissionDelete(ctx, provider, name)
}

func (p *Provider) confirmAdmissionAbsent(ctx context.Context, provider, name string) error {
	store, ok := p.store.(interface {
		ConfirmAdmissionAbsentByInstance(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("admission store cannot confirm absence")
	}
	return store.ConfirmAdmissionAbsentByInstance(ctx, provider, name)
}

func (p *Provider) admissionPolicy(ctx context.Context, provider string) (params.AdmissionConfig, error) {
	store, ok := p.store.(interface {
		AdmissionForProvider(context.Context, string) (params.AdmissionConfig, bool, error)
	})
	if !ok {
		return params.AdmissionConfig{}, fmt.Errorf("admission policy unavailable")
	}
	policy, enrolled, err := store.AdmissionForProvider(ctx, provider)
	if err != nil {
		return policy, err
	}
	if !enrolled {
		return policy, fmt.Errorf("admission provider is not enrolled")
	}
	return policy, nil
}

func (p *Provider) cancelUnsentAdmission(ctx context.Context, provider, name string) (bool, error) {
	store, ok := p.store.(interface {
		CancelUnsentBoundAdmission(context.Context, string, string) (bool, error)
	})
	if !ok {
		return false, fmt.Errorf("admission store cannot cancel an unsent allocation")
	}
	return store.CancelUnsentBoundAdmission(ctx, provider, name)
}
