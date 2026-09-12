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

// githubCredentials0007 is a minimal stub that only declares the new
// columns: whether a slice of the credential's rate limit budget is
// reserved for critical operations (such as runner deletion), and how
// large that slice is, as a percentage of the hourly limit.

type githubCredentials0007 struct {
	ReserveUsageEnabled    bool
	ReserveUsagePercentage int
}

func (githubCredentials0007) TableName() string { return "github_credentials" }

func init() {
	Register(&gormigrate.Migration{
		ID: "0007_credentials_reserve_usage",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&githubCredentials0007{})
		},
	})
}
