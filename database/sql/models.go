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
	"time"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/providers/common"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (b *Base) BeforeCreate(tx *gorm.DB) error {
	emptyId := uuid.UUID{}
	if b.ID != emptyId {
		return nil
	}
	newID, err := uuid.NewUUID()
	if err != nil {
		return errors.Wrap(err, "generating id")
	}
	b.ID = newID
	return nil
}

type Tag struct {
	Base

	Name  string  `gorm:"type:varchar(64);uniqueIndex"`
	Pools []*Pool `gorm:"many2many:pool_tags;"`
}

type Pool struct {
	Base

	ProviderName           string `gorm:"index:idx_pool_type"`
	RunnerPrefix           string
	MaxRunners             uint
	MinIdleRunners         uint
	RunnerBootstrapTimeout uint
	Image                  string `gorm:"index:idx_pool_type"`
	Flavor                 string `gorm:"index:idx_pool_type"`
	OSType                 params.OSType
	OSArch                 params.OSArch
	Tags                   []*Tag `gorm:"many2many:pool_tags;"`
	Enabled                bool
	// ExtraSpecs is an opaque json that gets sent to the provider
	// as part of the bootstrap params for instances. It can contain
	// any kind of data needed by providers.
	ExtraSpecs        datatypes.JSON
	GitHubRunnerGroup string

	RepoID     uuid.UUID  `gorm:"index"`
	Repository Repository `gorm:"foreignKey:RepoID"`

	OrgID        uuid.UUID    `gorm:"index"`
	Organization Organization `gorm:"foreignKey:OrgID"`

	EnterpriseID uuid.UUID  `gorm:"index"`
	Enterprise   Enterprise `gorm:"foreignKey:EnterpriseID"`

	Instances []Instance `gorm:"foreignKey:PoolID"`
}

type Repository struct {
	Base

	CredentialsName string
	Owner           string `gorm:"index:idx_owner_nocase,unique,collate:nocase"`
	Name            string `gorm:"index:idx_owner_nocase,unique,collate:nocase"`
	WebhookSecret   []byte
	Pools           []Pool `gorm:"foreignKey:RepoID"`
}

type Organization struct {
	Base

	CredentialsName string
	Name            string `gorm:"index:idx_org_name_nocase,collate:nocase"`
	WebhookSecret   []byte
	Pools           []Pool `gorm:"foreignKey:OrgID"`
}

type Enterprise struct {
	Base

	CredentialsName string
	Name            string `gorm:"index:idx_ent_name_nocase,collate:nocase"`
	WebhookSecret   []byte
	Pools           []Pool `gorm:"foreignKey:EnterpriseID"`
}

type Address struct {
	Base

	Address string
	Type    string

	InstanceID uuid.UUID
	Instance   Instance `gorm:"foreignKey:InstanceID"`
}

type InstanceStatusUpdate struct {
	Base

	EventType  params.EventType `gorm:"index:eventType"`
	EventLevel params.EventLevel
	Message    string `gorm:"type:text"`

	InstanceID uuid.UUID
	Instance   Instance `gorm:"foreignKey:InstanceID"`
}

type Instance struct {
	Base

	ProviderID        *string `gorm:"uniqueIndex"`
	Name              string  `gorm:"uniqueIndex"`
	AgentID           int64
	OSType            params.OSType
	OSArch            params.OSArch
	OSName            string
	OSVersion         string
	Addresses         []Address `gorm:"foreignKey:InstanceID"`
	Status            common.InstanceStatus
	RunnerStatus      common.RunnerStatus
	CallbackURL       string
	MetadataURL       string
	ProviderFault     []byte `gorm:"type:longblob"`
	CreateAttempt     int
	TokenFetched      bool
	GitHubRunnerGroup string
	AditionalLabels   datatypes.JSON

	PoolID uuid.UUID
	Pool   Pool `gorm:"foreignKey:PoolID"`

	StatusMessages []InstanceStatusUpdate `gorm:"foreignKey:InstanceID"`
}

type User struct {
	Base

	Username string `gorm:"uniqueIndex;varchar(64)"`
	FullName string `gorm:"type:varchar(254)"`
	Email    string `gorm:"type:varchar(254);unique;index:idx_email"`
	Password string `gorm:"type:varchar(60)"`
	IsAdmin  bool
	Enabled  bool
}

type ControllerInfo struct {
	Base

	ControllerID uuid.UUID
}

type WorkflowJob struct {
	// ID is the ID of the job.
	ID int64 `gorm:"index"`
	// RunID is the ID of the workflow run. A run may have multiple jobs.
	RunID int64
	// Action is the specific activity that triggered the event.
	Action string `gorm:"type:varchar(254);index"`
	// Conclusion is the outcome of the job.
	// Possible values: "success", "failure", "neutral", "cancelled", "skipped",
	// "timed_out", "action_required"
	Conclusion string
	// Status is the phase of the lifecycle that the job is currently in.
	// "queued", "in_progress" and "completed".
	Status string
	// Name is the name if the job that was triggered.
	Name string

	StartedAt   time.Time
	CompletedAt time.Time

	GithubRunnerID  int64
	RunnerName      string
	RunnerGroupID   int64
	RunnerGroupName string

	// repository in which the job was triggered.
	RepositoryName  string
	RepositoryOwner string

	Labels datatypes.JSON

	// The entity that received the hook.
	//
	// Webhooks may be configured on the repo, the org and/or the enterprise.
	// If we only configure a repo to use garm, we'll only ever receive a
	// webhook from the repo. But if we configure the parent org of the repo and
	// the parent enterprise of the org to use garm, a webhook will be sent for each
	// entity type, in response to one workflow event. Thus, we will get 3 webhooks
	// with the same run_id and job id. Record all involved entities in the same job
	// if we have them configured in garm.
	RepoID     uuid.UUID  `gorm:"index"`
	Repository Repository `gorm:"foreignKey:RepoID"`

	OrgID        uuid.UUID    `gorm:"index"`
	Organization Organization `gorm:"foreignKey:OrgID"`

	EnterpriseID uuid.UUID  `gorm:"index"`
	Enterprise   Enterprise `gorm:"foreignKey:EnterpriseID"`

	LockedBy uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
