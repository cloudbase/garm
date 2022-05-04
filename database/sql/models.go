package sql

import (
	"garm/config"
	"garm/runner/providers/common"
	"time"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
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
	newID, err := uuid.NewV4()
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

	ProviderName   string `gorm:"index:idx_pool_type,unique"`
	MaxRunners     uint
	MinIdleRunners uint
	Image          string `gorm:"index:idx_pool_type,unique"`
	Flavor         string `gorm:"index:idx_pool_type,unique"`
	OSType         config.OSType
	OSArch         config.OSArch
	Tags           []*Tag `gorm:"many2many:pool_tags;"`
	Enabled        bool

	RepoID     uuid.UUID
	Repository Repository `gorm:"foreignKey:RepoID"`

	OrgID        uuid.UUID
	Organization Organization `gorm:"foreignKey:OrgID"`

	Instances []Instance `gorm:"foreignKey:PoolID"`
}

type Repository struct {
	Base

	CredentialsName string
	Owner           string `gorm:"index:idx_owner,unique"`
	Name            string `gorm:"index:idx_owner,unique"`
	WebhookSecret   []byte
	Pools           []Pool `gorm:"foreignKey:RepoID"`
}

type Organization struct {
	Base

	CredentialsName string
	Name            string `gorm:"uniqueIndex"`
	WebhookSecret   []byte
	Pools           []Pool `gorm:"foreignKey:OrgID"`
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

	Message string `gorm:"type:text"`

	InstanceID uuid.UUID
	Instance   Instance `gorm:"foreignKey:InstanceID"`
}

type Instance struct {
	Base

	ProviderID   *string `gorm:"uniqueIndex"`
	Name         string  `gorm:"uniqueIndex"`
	OSType       config.OSType
	OSArch       config.OSArch
	OSName       string
	OSVersion    string
	Addresses    []Address `gorm:"foreignKey:InstanceID"`
	Status       common.InstanceStatus
	RunnerStatus common.RunnerStatus
	CallbackURL  string

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
