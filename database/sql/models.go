package sql

import (
	"runner-manager/config"
	"time"

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
	b.ID = uuid.NewV4()
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

	RepoID     uuid.UUID
	Repository Repository `gorm:"foreignKey:RepoID"`

	OrgID        uuid.UUID
	Organization Organization `gorm:"foreignKey:OrgID"`
}

type Repository struct {
	Base

	Owner         string `gorm:"index:idx_owner,unique"`
	Name          string `gorm:"index:idx_owner,unique"`
	WebhookSecret []byte
	Pools         []Pool `gorm:"foreignKey:RepoID"`
}

type Organization struct {
	Base

	Name          string `gorm:"uniqueIndex"`
	WebhookSecret []byte
	Pools         []Pool `gorm:"foreignKey:OrgID"`
}

type Address struct {
	Base

	Address string
	Type    string
}

type Instance struct {
	Base

	Name          string `gorm:"uniqueIndex"`
	OSType        config.OSType
	OSArch        config.OSArch
	OSName        string
	OSVersion     string
	Addresses     []Address `gorm:"foreignKey:id"`
	Status        string
	RunnerStatus  string
	CallbackURL   string
	CallbackToken string

	Pool Pool `gorm:"foreignKey:id"`
}
