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

// controllerInfo0005 is a minimal stub that only declares the new column:
// whether deployed garm-agents are configured to connect to GARM over
// plain http/ws (the agent's force_insecure setting).

type controllerInfo0005 struct {
	AllowInsecureGARMAgent bool
}

func (controllerInfo0005) TableName() string { return "controller_infos" }

func init() {
	Register(&gormigrate.Migration{
		ID: "0005_allow_insecure_garm_agent",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&controllerInfo0005{})
		},
	})
}
