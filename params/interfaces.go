// Copyright 2025 Cloudbase Solutions SRL
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

package params

import (
	"context"
	"io"
	"time"
)

// EntityGetter is implemented by all github entities (repositories, organizations and enterprises).
// It defines the GetEntity() function which returns a github entity.
type EntityGetter interface {
	GetEntity() (ForgeEntity, error)
}

type IDGetter interface {
	GetID() uint
}

type CreationDateGetter interface {
	GetCreatedAt() time.Time
}

type ForgeCredentialsGetter interface {
	GetForgeCredentials() ForgeCredentials
}

type GARMToolsManager interface {
	ListAllGARMTools(ctx context.Context) ([]GARMAgentTool, error)
	CreateGARMTool(ctx context.Context, param CreateGARMToolParams, reader io.Reader) (FileObject, error)
	DeleteGarmTool(ctx context.Context, osType, osArch string) error
}
