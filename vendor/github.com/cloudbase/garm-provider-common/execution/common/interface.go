// Copyright 2023 Cloudbase Solutions SRL
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

	"github.com/cloudbase/garm-provider-common/params"
)

// ExternalProvider defines a common interface that external providers need to implement.
// This is very similar to the common.Provider interface, and was redefined here to
// decouple it, in case it may diverge from native providers.
type ExternalProvider interface {
	// CreateInstance creates a new compute instance in the provider.
	CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.ProviderInstance, error)
	// Delete instance will delete the instance in a provider.
	DeleteInstance(ctx context.Context, instance string) error
	// GetInstance will return details about one instance.
	GetInstance(ctx context.Context, instance string) (params.ProviderInstance, error)
	// ListInstances will list all instances for a provider.
	ListInstances(ctx context.Context, poolID string) ([]params.ProviderInstance, error)
	// RemoveAllInstances will remove all instances created by this provider.
	RemoveAllInstances(ctx context.Context) error
	// Stop shuts down the instance.
	Stop(ctx context.Context, instance string, force bool) error
	// Start boots up an instance.
	Start(ctx context.Context, instance string) error
	// GetVersion returns the version of the provider.
	GetVersion(ctx context.Context) string
}
