// Copyright 2026 Cloudbase Solutions SRL
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

package migrations

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cloudbase/garm/params"
)

// Minimal model copies for the migration. These are intentionally decoupled
// from the main models so that future model changes don't break this migration.

type forgeInstance0003 struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	GiteaCredentialsID *uint   `gorm:"index"`
	EndpointName       *string `gorm:"uniqueIndex:idx_forgeinstance_endpoint_nocase,expression:LOWER(endpoint_name)"`

	PoolManagerRunning       bool
	PoolManagerFailureReason string

	WebhookSecret    []byte
	PoolBalancerType params.PoolBalancerType `gorm:"type:varchar(64)"`
	AgentMode        bool                    `gorm:"index:forgeinstance_agent_idx"`
}

func (forgeInstance0003) TableName() string { return "forge_instances" }

type forgeInstanceEvent0003 struct {
	gorm.Model

	EventType  params.EventType
	EventLevel params.EventLevel
	Message    string `gorm:"type:text"`

	ForgeInstanceID uuid.UUID `gorm:"index:idx_forgeinstance_event"`
}

func (forgeInstanceEvent0003) TableName() string { return "forge_instance_events" }

// pool0003 and workflowJob0003 are minimal stubs that only declare the new
// column. AutoMigrate will add the column without touching existing ones.

type pool0003 struct {
	ForgeInstanceID *uuid.UUID `gorm:"index"`
}

func (pool0003) TableName() string { return "pools" }

type workflowJob0003 struct {
	ForgeInstanceID *uuid.UUID `gorm:"index"`
}

func (workflowJob0003) TableName() string { return "workflow_jobs" }

func init() {
	Register(&gormigrate.Migration{
		ID: "0003_forge_instances",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&forgeInstance0003{},
				&forgeInstanceEvent0003{},
				&pool0003{},
				&workflowJob0003{},
			)
		},
	})
}
