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

package common

import (
	"context"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

//go:generate go run github.com/vektra/mockery/v2@latest
type Provider interface {
	// CreateInstance creates a new compute instance in the provider.
	CreateInstance(ctx context.Context, bootstrapParams commonParams.BootstrapInstance, createInstanceParams CreateInstanceParams) (commonParams.ProviderInstance, error)
	// Delete instance will delete the instance in a provider.
	DeleteInstance(ctx context.Context, instance string, deleteInstanceParams DeleteInstanceParams) error
	// GetInstance will return details about one instance.
	GetInstance(ctx context.Context, instance string, getInstanceParams GetInstanceParams) (commonParams.ProviderInstance, error)
	// ListInstances will list all instances for a provider.
	ListInstances(ctx context.Context, poolID string, listInstancesParams ListInstancesParams) ([]commonParams.ProviderInstance, error)
	// RemoveAllInstances will remove all instances created by this provider.
	RemoveAllInstances(ctx context.Context, removeAllInstancesParams RemoveAllInstancesParams) error
	// Stop shuts down the instance.
	Stop(ctx context.Context, instance string, stopParams StopParams) error
	// Start boots up an instance.
	Start(ctx context.Context, instance string, startParams StartParams) error
	// DisableJITConfig tells us if the provider explicitly disables JIT configuration and
	// forces runner registration tokens to be used. This may happen if a provider has not yet
	// been updated to support JIT configuration.
	DisableJITConfig() bool

	AsParams() params.Provider
}
