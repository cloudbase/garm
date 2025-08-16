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

package sql

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (b *Base) BeforeCreate(_ *gorm.DB) error {
	emptyID := uuid.UUID{}
	if b.ID != emptyID {
		return nil
	}
	newID, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("error generating id: %w", err)
	}
	b.ID = newID
	return nil
}

type ControllerInfo struct {
	Base

	ControllerID uuid.UUID

	CallbackURL    string
	MetadataURL    string
	WebhookBaseURL string
	// MinimumJobAgeBackoff is the minimum time that a job must be in the queue
	// before GARM will attempt to allocate a runner to service it. This backoff
	// is useful if you have idle runners in various pools that could potentially
	// pick up the job. GARM would allow this amount of time for runners to react
	// before spinning up a new one and potentially having to scale down later.
	MinimumJobAgeBackoff uint
}

type Tag struct {
	Base

	Name  string  `gorm:"type:varchar(64);uniqueIndex"`
	Pools []*Pool `gorm:"many2many:pool_tags;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
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
	OSType                 commonParams.OSType
	OSArch                 commonParams.OSArch
	Tags                   []*Tag `gorm:"many2many:pool_tags;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
	Enabled                bool
	// ExtraSpecs is an opaque json that gets sent to the provider
	// as part of the bootstrap params for instances. It can contain
	// any kind of data needed by providers.
	ExtraSpecs        datatypes.JSON
	GitHubRunnerGroup string

	RepoID     *uuid.UUID `gorm:"index"`
	Repository Repository `gorm:"foreignKey:RepoID;"`

	OrgID        *uuid.UUID   `gorm:"index"`
	Organization Organization `gorm:"foreignKey:OrgID"`

	EnterpriseID *uuid.UUID `gorm:"index"`
	Enterprise   Enterprise `gorm:"foreignKey:EnterpriseID"`

	Instances []Instance `gorm:"foreignKey:PoolID"`
	Priority  uint       `gorm:"index:idx_pool_priority"`
}

// ScaleSet represents a github scale set. Scale sets are almost identical to pools with a few
// notable exceptions:
//   - Labels are no longer relevant
//   - Workflows will use the scaleset name to target runners.
//   - A scale set is a stand alone unit. If a workflow targets a scale set, no other runner will pick up that job.
type ScaleSet struct {
	gorm.Model

	// ScaleSetID is the github ID of the scale set. This field may not be set if
	// the scale set was ceated in GARM but has not yet been created in GitHub.
	// The scale set ID is also not globally unique. It is only unique within the context
	// of an entity.
	ScaleSetID        int    `gorm:"index:idx_scale_set"`
	Name              string `gorm:"unique_index:idx_name"`
	GitHubRunnerGroup string `gorm:"unique_index:idx_name"`
	DisableUpdate     bool

	// State stores the provisioning state of the scale set in GitHub
	State params.ScaleSetState
	// ExtendedState stores a more detailed message regarding the State.
	// If an error occurs, the reason for the error will be stored here.
	ExtendedState string

	ProviderName           string
	RunnerPrefix           string
	MaxRunners             uint
	MinIdleRunners         uint
	RunnerBootstrapTimeout uint
	Image                  string
	Flavor                 string
	OSType                 commonParams.OSType
	OSArch                 commonParams.OSArch
	Enabled                bool
	LastMessageID          int64
	DesiredRunnerCount     int
	// ExtraSpecs is an opaque json that gets sent to the provider
	// as part of the bootstrap params for instances. It can contain
	// any kind of data needed by providers.
	ExtraSpecs datatypes.JSON

	RepoID     *uuid.UUID `gorm:"index"`
	Repository Repository `gorm:"foreignKey:RepoID;"`

	OrgID        *uuid.UUID   `gorm:"index"`
	Organization Organization `gorm:"foreignKey:OrgID"`

	EnterpriseID *uuid.UUID `gorm:"index"`
	Enterprise   Enterprise `gorm:"foreignKey:EnterpriseID"`

	Instances []Instance `gorm:"foreignKey:ScaleSetFkID"`
}

type RepositoryEvent struct {
	gorm.Model

	EventType  params.EventType
	EventLevel params.EventLevel
	Message    string `gorm:"type:text"`

	RepoID uuid.UUID  `gorm:"index:idx_repo_event"`
	Repo   Repository `gorm:"foreignKey:RepoID"`
}

type Repository struct {
	Base

	CredentialsID *uint             `gorm:"index"`
	Credentials   GithubCredentials `gorm:"foreignKey:CredentialsID;constraint:OnDelete:SET NULL"`

	GiteaCredentialsID *uint            `gorm:"index"`
	GiteaCredentials   GiteaCredentials `gorm:"foreignKey:GiteaCredentialsID;constraint:OnDelete:SET NULL"`

	Owner            string `gorm:"index:idx_owner_nocase,unique,collate:nocase"`
	Name             string `gorm:"index:idx_owner_nocase,unique,collate:nocase"`
	WebhookSecret    []byte
	Pools            []Pool                  `gorm:"foreignKey:RepoID"`
	ScaleSets        []ScaleSet              `gorm:"foreignKey:RepoID"`
	Jobs             []WorkflowJob           `gorm:"foreignKey:RepoID;constraint:OnDelete:SET NULL"`
	PoolBalancerType params.PoolBalancerType `gorm:"type:varchar(64)"`

	EndpointName *string        `gorm:"index:idx_owner_nocase,unique,collate:nocase"`
	Endpoint     GithubEndpoint `gorm:"foreignKey:EndpointName;constraint:OnDelete:SET NULL"`

	Events []RepositoryEvent `gorm:"foreignKey:RepoID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
}

type OrganizationEvent struct {
	gorm.Model

	EventType  params.EventType
	EventLevel params.EventLevel
	Message    string `gorm:"type:text"`

	OrgID uuid.UUID    `gorm:"index:idx_org_event"`
	Org   Organization `gorm:"foreignKey:OrgID"`
}
type Organization struct {
	Base

	CredentialsID *uint             `gorm:"index"`
	Credentials   GithubCredentials `gorm:"foreignKey:CredentialsID;constraint:OnDelete:SET NULL"`

	GiteaCredentialsID *uint            `gorm:"index"`
	GiteaCredentials   GiteaCredentials `gorm:"foreignKey:GiteaCredentialsID;constraint:OnDelete:SET NULL"`

	Name             string `gorm:"index:idx_org_name_nocase,collate:nocase"`
	WebhookSecret    []byte
	Pools            []Pool                  `gorm:"foreignKey:OrgID"`
	ScaleSet         []ScaleSet              `gorm:"foreignKey:OrgID"`
	Jobs             []WorkflowJob           `gorm:"foreignKey:OrgID;constraint:OnDelete:SET NULL"`
	PoolBalancerType params.PoolBalancerType `gorm:"type:varchar(64)"`

	EndpointName *string        `gorm:"index:idx_org_name_nocase,collate:nocase"`
	Endpoint     GithubEndpoint `gorm:"foreignKey:EndpointName;constraint:OnDelete:SET NULL"`

	Events []OrganizationEvent `gorm:"foreignKey:OrgID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
}

type EnterpriseEvent struct {
	gorm.Model

	EventType  params.EventType
	EventLevel params.EventLevel
	Message    string `gorm:"type:text"`

	EnterpriseID uuid.UUID  `gorm:"index:idx_enterprise_event"`
	Enterprise   Enterprise `gorm:"foreignKey:EnterpriseID"`
}

type Enterprise struct {
	Base

	CredentialsID *uint             `gorm:"index"`
	Credentials   GithubCredentials `gorm:"foreignKey:CredentialsID;constraint:OnDelete:SET NULL"`

	Name             string `gorm:"index:idx_ent_name_nocase,collate:nocase"`
	WebhookSecret    []byte
	Pools            []Pool                  `gorm:"foreignKey:EnterpriseID"`
	ScaleSet         []ScaleSet              `gorm:"foreignKey:EnterpriseID"`
	Jobs             []WorkflowJob           `gorm:"foreignKey:EnterpriseID;constraint:OnDelete:SET NULL"`
	PoolBalancerType params.PoolBalancerType `gorm:"type:varchar(64)"`

	EndpointName *string        `gorm:"index:idx_ent_name_nocase,collate:nocase"`
	Endpoint     GithubEndpoint `gorm:"foreignKey:EndpointName;constraint:OnDelete:SET NULL"`

	Events []EnterpriseEvent `gorm:"foreignKey:EnterpriseID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
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

	InstanceID uuid.UUID `gorm:"index:idx_instance_status_updates_instance_id"`
	Instance   Instance  `gorm:"foreignKey:InstanceID"`
}

type Instance struct {
	Base

	ProviderID        *string `gorm:"uniqueIndex"`
	Name              string  `gorm:"uniqueIndex"`
	AgentID           int64
	OSType            commonParams.OSType
	OSArch            commonParams.OSArch
	OSName            string
	OSVersion         string
	Addresses         []Address `gorm:"foreignKey:InstanceID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
	Status            commonParams.InstanceStatus
	RunnerStatus      params.RunnerStatus
	CallbackURL       string
	MetadataURL       string
	ProviderFault     []byte `gorm:"type:longblob"`
	CreateAttempt     int
	TokenFetched      bool
	JitConfiguration  []byte `gorm:"type:longblob"`
	GitHubRunnerGroup string
	AditionalLabels   datatypes.JSON

	PoolID *uuid.UUID
	Pool   Pool `gorm:"foreignKey:PoolID"`

	ScaleSetFkID *uint
	ScaleSet     ScaleSet `gorm:"foreignKey:ScaleSetFkID"`

	StatusMessages []InstanceStatusUpdate `gorm:"foreignKey:InstanceID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`

	Job *WorkflowJob `gorm:"foreignKey:InstanceID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
}

type User struct {
	Base

	Username   string `gorm:"uniqueIndex;varchar(64)"`
	FullName   string `gorm:"type:varchar(254)"`
	Email      string `gorm:"type:varchar(254);unique;index:idx_email"`
	Password   string `gorm:"type:varchar(60)"`
	Generation uint
	IsAdmin    bool
	Enabled    bool
}

type WorkflowJob struct {
	// ID is the ID of the job.
	ID int64 `gorm:"index"`

	// WorkflowJobID is the ID of the workflow job.
	WorkflowJobID int64 `gorm:"index:workflow_job_id_idx"`
	// ScaleSetJobID is the job ID for a scaleset job.
	ScaleSetJobID string `gorm:"index:scaleset_job_id_idx"`

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

	GithubRunnerID int64

	InstanceID *uuid.UUID `gorm:"index:idx_instance_job"`
	Instance   Instance   `gorm:"foreignKey:InstanceID"`

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
	RepoID     *uuid.UUID `gorm:"index"`
	Repository Repository `gorm:"foreignKey:RepoID"`

	OrgID        *uuid.UUID   `gorm:"index"`
	Organization Organization `gorm:"foreignKey:OrgID"`

	EnterpriseID *uuid.UUID `gorm:"index"`
	Enterprise   Enterprise `gorm:"foreignKey:EnterpriseID"`

	LockedBy uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type GithubEndpoint struct {
	Name      string `gorm:"type:varchar(64) collate nocase;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	EndpointType params.EndpointType `gorm:"index:idx_endpoint_type"`

	Description   string `gorm:"type:text"`
	APIBaseURL    string `gorm:"type:text collate nocase"`
	UploadBaseURL string `gorm:"type:text collate nocase"`
	BaseURL       string `gorm:"type:text collate nocase"`
	CACertBundle  []byte `gorm:"type:longblob"`
}

type GithubCredentials struct {
	gorm.Model

	Name   string     `gorm:"index:idx_github_credentials,unique;type:varchar(64) collate nocase"`
	UserID *uuid.UUID `gorm:"index:idx_github_credentials,unique"`
	User   User       `gorm:"foreignKey:UserID"`

	Description string               `gorm:"type:text"`
	AuthType    params.ForgeAuthType `gorm:"index"`
	Payload     []byte               `gorm:"type:longblob"`

	Endpoint     GithubEndpoint `gorm:"foreignKey:EndpointName"`
	EndpointName *string        `gorm:"index"`

	Repositories  []Repository   `gorm:"foreignKey:CredentialsID"`
	Organizations []Organization `gorm:"foreignKey:CredentialsID"`
	Enterprises   []Enterprise   `gorm:"foreignKey:CredentialsID"`
}

type GiteaCredentials struct {
	gorm.Model

	Name   string     `gorm:"index:idx_gitea_credentials,unique;type:varchar(64) collate nocase"`
	UserID *uuid.UUID `gorm:"index:idx_gitea_credentials,unique"`
	User   User       `gorm:"foreignKey:UserID"`

	Description string               `gorm:"type:text"`
	AuthType    params.ForgeAuthType `gorm:"index"`
	Payload     []byte               `gorm:"type:longblob"`

	Endpoint     GithubEndpoint `gorm:"foreignKey:EndpointName"`
	EndpointName *string        `gorm:"index"`

	Repositories  []Repository   `gorm:"foreignKey:GiteaCredentialsID"`
	Organizations []Organization `gorm:"foreignKey:GiteaCredentialsID"`
}
