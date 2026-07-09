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
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Minimal model copies for the migration. These are intentionally decoupled
// from the main models so that future model changes don't break this migration.

type proxy0006 struct {
	gorm.Model

	Name        string `gorm:"index:idx_proxy_name,unique,expression:LOWER(name);type:varchar(64)"`
	Description string `gorm:"type:text"`

	HTTPProxy  string `gorm:"column:http_proxy;type:text"`
	HTTPSProxy string `gorm:"column:https_proxy;type:text"`
	NoProxy    string `gorm:"column:no_proxy;type:text"`

	Credentials []byte
}

func (proxy0006) TableName() string { return "proxies" }

// pool0006 and scaleSet0006 are minimal stubs that only declare the new
// column. AutoMigrate will add the column without touching existing ones.

type pool0006 struct {
	ProxyID *uint `gorm:"index"`
}

func (pool0006) TableName() string { return "pools" }

type scaleSet0006 struct {
	ProxyID *uint `gorm:"index"`
}

func (scaleSet0006) TableName() string { return "scale_sets" }

func init() {
	Register(&gormigrate.Migration{
		ID: "0006_proxies",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&proxy0006{},
				&pool0006{},
				&scaleSet0006{},
			)
		},
	})
}
