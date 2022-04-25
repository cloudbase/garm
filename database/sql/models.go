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
	b.ID = uuid.NewV4()
	return nil
}

type Tag struct {
	Base

	Name string `gorm:"type:varchar(64);uniqueIndex"`
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
	Tags           []Tag `gorm:"foreignKey:id"`
}

type Repository struct {
	Base

	Owner         string `gorm:"index:idx_owner,unique"`
	Name          string `gorm:"index:idx_owner,unique"`
	WebhookSecret []byte
	Pools         []Pool `gorm:"foreignKey:id"`
}

type Organization struct {
	Base

	Name          string `gorm:"uniqueIndex"`
	WebhookSecret []byte
	Pools         []Pool `gorm:"foreignKey:id"`
}
