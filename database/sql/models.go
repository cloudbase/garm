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

	// CallbackURL is the URL where userdata scripts call back into, to send status updates
	// and installation progress.
	CallbackURL string
	// MetadataURL is the base URL from which runners can get their installation metadata.
	MetadataURL string
	// WebhookBaseURL is the base URL used to construct the controller webhook URL.
	WebhookBaseURL string
	// AgentURL is the websocket enabled URL where garm agents connect to.
	AgentURL string
	// GARMAgentReleasesURL is the URL from which GARM can sync garm-agent binaries. Alternatively
	// the user can manually upload binaries.
	GARMAgentReleasesURL string
	// SyncGARMAgentTools enables or disables automatic sync of garm-agent tools.
	SyncGARMAgentTools bool
	// MinimumJobAgeBackoff is the minimum time that a job must be in the queue
	// before GARM will attempt to allocate a runner to service it. This backoff
	// is useful if you have idle runners in various pools that could potentially
	// pick up the job. GARM would allow this amount of time for runners to react
	// before spinning up a new one and potentially having to scale down later.
	MinimumJobAgeBackoff uint
	// CachedGARMAgentRelease stores the cached JSON response from GARMAgentReleasesURL
	CachedGARMAgentRelease datatypes.JSON
	// CachedGARMAgentReleaseFetchedAt is the timestamp when the release data was last fetched
	CachedGARMAgentReleaseFetchedAt *time.Time
}

type Tag struct {
	Base

	Name  string  `gorm:"type:varchar(64);uniqueIndex"`
	Pools []*Pool `gorm:"many2many:pool_tags;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
}

type Template struct {
	gorm.Model

	Name   string     `gorm:"index:idx_template,unique;type:varchar(128)"`
	UserID *uuid.UUID `gorm:"index:idx_template,unique"`
	User   User       `gorm:"foreignKey:UserID"`

	Description string              `gorm:"type:text"`
	OSType      commonParams.OSType `gorm:"type:varchar(32);index:idx_tpl_os_type"`
	ForgeType   params.EndpointType `gorm:"type:varchar(32);index:idx_tpl_forge_type"`
	Data        []byte              `gorm:"type:longblob"`

	ScaleSets []ScaleSet `gorm:"foreignKey:TemplateID"`
	Pools     []Pool     `gorm:"foreignKey:TemplateID"`
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
	EnableShell       bool

	// Generation holds the numeric generation of the pool. This number
	// will be incremented, every time certain settings of the pool, which
	// may influence how runners are created (flavor, specs, image) are changed.
	// When a runner is created, this generation will be copied to the runners as
	// well. That way if some settings diverge, we can target those runners
	// to be recreated.
	Generation uint64

	RepoID     *uuid.UUID `gorm:"index"`
	Repository Repository `gorm:"foreignKey:RepoID;"`

	OrgID        *uuid.UUID   `gorm:"index"`
	Organization Organization `gorm:"foreignKey:OrgID"`

	EnterpriseID *uuid.UUID `gorm:"index"`
	Enterprise   Enterprise `gorm:"foreignKey:EnterpriseID"`

	TemplateID *uint    `gorm:"index"`
	Template   Template `gorm:"foreignKey:TemplateID"`

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
	ExtraSpecs  datatypes.JSON
	EnableShell bool

	// Generation is the scaleset generation at the time of creating this instance.
	// This field is to track a divergence between when the instance was created
	// and the settings currently set on a scaleset. We can then use this field to know
	// if the instance is out of date with the scaleset, allowing us to remove it if we
	// need to.
	Generation uint64

	RepoID     *uuid.UUID `gorm:"index"`
	Repository Repository `gorm:"foreignKey:RepoID;"`

	OrgID        *uuid.UUID   `gorm:"index"`
	Organization Organization `gorm:"foreignKey:OrgID"`

	EnterpriseID *uuid.UUID `gorm:"index"`
	Enterprise   Enterprise `gorm:"foreignKey:EnterpriseID"`

	TemplateID *uint    `gorm:"index"`
	Template   Template `gorm:"foreignKey:TemplateID"`

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
	AgentMode        bool                    `gorm:"index:repo_agent_idx"`

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
	AgentMode        bool                    `gorm:"index:org_agent_idx"`

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
	AgentMode        bool                    `gorm:"index:enterprise_agent_idx"`

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
	Heartbeat         time.Time
	CallbackURL       string
	MetadataURL       string
	ProviderFault     []byte `gorm:"type:longblob"`
	CreateAttempt     int
	TokenFetched      bool
	JitConfiguration  []byte `gorm:"type:longblob"`
	GitHubRunnerGroup string
	AditionalLabels   datatypes.JSON
	Capabilities      datatypes.JSON
	// Generation is the pool generation at the time of creating this instance.
	// This field is to track a divergence between when the instance was created
	// and the settings currently set on a pool. We can then use this field to know
	// if the instance is out of date with the pool, allowing us to remove it if we
	// need to.
	Generation uint64

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
	Status string `gorm:"index:idx_workflow_jobs_status_instance_id,priority:1"`
	// Name is the name if the job that was triggered.
	Name string

	StartedAt   time.Time
	CompletedAt time.Time

	GithubRunnerID int64

	InstanceID *uuid.UUID `gorm:"index:idx_instance_job;index:idx_workflow_jobs_status_instance_id,priority:2"`
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

	EndpointType             params.EndpointType `gorm:"index:idx_endpoint_type"`
	ToolsMetadataURL         string              `gorm:"type:text collate nocase"`
	UseInternalToolsMetadata bool

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

// FileObject represents the table that holds files. This can be used to store
// GARM agent binaries, runner binary downloads that may be cached, etc.
type FileObject struct {
	gorm.Model
	// Name is the name of the file
	Name string `gorm:"type:text;index:idx_fo_name"`
	// Description is a description for the file
	Description string `gorm:"type:text"`
	// FileType holds the MIME type or file type description
	FileType string `gorm:"type:text"`
	// Size is the file size in bytes
	Size int64 `gorm:"type:integer"`
	// SHA256 is the sha256 checksum (hex encoded)
	SHA256 string `gorm:"type:text;index:idx_fo_chksum"`
	// Tags is a JSON array of tags
	TagsList []FileObjectTag `gorm:"foreignKey:FileObjectID;constraint:OnDelete:CASCADE"`
	// Content is a foreign key to a different table where the blob is actually stored.
	// Updating a field in an sqlite 3 DB will read the entire field, update the column
	// and write it back to a different location. SQLite3 will then mark the old row as "deleted"
	// and allow it to be vaccumed. But if we have a blob column with a huge blob, any
	// update operation will consume a lot of resources and take a long time.
	// Using a dedicated table for the blob (which doesn't change), speeds up updates of
	// metadata fields like name, description, tags, etc.
	Content FileBlob `gorm:"foreignKey:FileObjectID;constraint:OnDelete:CASCADE"`
}

// FileBlob is the immutable blob of bytes that once written will not be changed.
// We leave the SHA256, file type and size in the parent table, because we need to
// we able to get that info easily, without preloading the blob table.
type FileBlob struct {
	gorm.Model
	FileObjectID uint `gorm:"index:idx_fileobject_blob_id"`
	// Content is a BLOB column for storing binary data
	Content []byte `gorm:"type:blob"`
}

// FileObjectTag represents the many-to-many relationship between documents and tags
type FileObjectTag struct {
	ID           uint   `gorm:"primaryKey"`
	FileObjectID uint   `gorm:"index:idx_fileobject_tags_doc_id,priority:1;index:idx_fileobject_tags_tag,priority:1;not null"`
	Tag          string `gorm:"type:TEXT COLLATE NOCASE;index:idx_fileobject_tags_tag,priority:2;not null"`
}
