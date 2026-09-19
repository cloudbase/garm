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
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Databases migrated through the numbered stubs are missing the foreign key
// constraints that fresh databases get from the full models: the stubs only
// add columns and indexes. This migration brings existing databases to
// constraint parity. The stubs below mirror the relation declarations of
// the real models (both sides, with identical field names) so GORM derives
// identical constraint names. Constraints that already exist are skipped,
// making this a no-op on databases created from the full models.

type proxy0008 struct {
	ID uint `gorm:"primarykey"`

	Pools     []pool0008     `gorm:"foreignKey:ProxyID"`
	ScaleSets []scaleSet0008 `gorm:"foreignKey:ProxyID"`
}

func (proxy0008) TableName() string { return "proxies" }

type githubEndpoint0008 struct {
	Name string `gorm:"type:varchar(64);primary_key;"`
}

func (githubEndpoint0008) TableName() string { return "github_endpoints" }

type giteaCredentials0008 struct {
	ID uint `gorm:"primarykey"`

	ForgeInstances []forgeInstance0008 `gorm:"foreignKey:GiteaCredentialsID"`
}

func (giteaCredentials0008) TableName() string { return "gitea_credentials" }

type forgeInstance0008 struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key;"`

	GiteaCredentialsID *uint                `gorm:"index"`
	GiteaCredentials   giteaCredentials0008 `gorm:"foreignKey:GiteaCredentialsID;constraint:OnDelete:SET NULL"`

	EndpointName *string            `gorm:"uniqueIndex:idx_forgeinstance_endpoint_nocase,expression:LOWER(endpoint_name)"`
	Endpoint     githubEndpoint0008 `gorm:"foreignKey:EndpointName;constraint:OnDelete:SET NULL"`

	Pools  []pool0008               `gorm:"foreignKey:ForgeInstanceID"`
	Jobs   []workflowJob0008        `gorm:"foreignKey:ForgeInstanceID;constraint:OnDelete:SET NULL"`
	Events []forgeInstanceEvent0008 `gorm:"foreignKey:ForgeInstanceID;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;"`
}

func (forgeInstance0008) TableName() string { return "forge_instances" }

type forgeInstanceEvent0008 struct {
	ID uint `gorm:"primarykey"`

	ForgeInstanceID uuid.UUID         `gorm:"index:idx_forgeinstance_event"`
	ForgeInstance   forgeInstance0008 `gorm:"foreignKey:ForgeInstanceID"`
}

func (forgeInstanceEvent0008) TableName() string { return "forge_instance_events" }

type pool0008 struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key;"`

	ProxyID *uint     `gorm:"index"`
	Proxy   proxy0008 `gorm:"foreignKey:ProxyID"`

	ForgeInstanceID *uuid.UUID        `gorm:"index"`
	ForgeInstance   forgeInstance0008 `gorm:"foreignKey:ForgeInstanceID"`
}

func (pool0008) TableName() string { return "pools" }

type scaleSet0008 struct {
	ID uint `gorm:"primarykey"`

	ProxyID *uint     `gorm:"index"`
	Proxy   proxy0008 `gorm:"foreignKey:ProxyID"`
}

func (scaleSet0008) TableName() string { return "scale_sets" }

type workflowJob0008 struct {
	ID int64 `gorm:"index"`

	ForgeInstanceID *uuid.UUID        `gorm:"index"`
	ForgeInstance   forgeInstance0008 `gorm:"foreignKey:ForgeInstanceID"`
}

func (workflowJob0008) TableName() string { return "workflow_jobs" }

type fkViolation struct {
	Table  string `gorm:"column:table"`
	Rowid  int64  `gorm:"column:rowid"`
	Parent string `gorm:"column:parent"`
	Fkid   int64  `gorm:"column:fkid"`
}

// resolveSQLiteOrphans applies each foreign key's own delete semantics to
// rows that violate it. Old GARM versions ran with foreign key enforcement
// disabled in places, so long lived databases can hold orphaned rows (for
// example addresses of instances deleted years ago) that enforcement never
// sees. CASCADE orphans are deleted, SET NULL references are cleared and
// anything else is left alone and reported by the caller.
func resolveSQLiteOrphans(tx *gorm.DB) error {
	var violations []fkViolation
	if err := tx.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil {
		return fmt.Errorf("checking foreign keys: %w", err)
	}

	for _, v := range violations {
		var fks []struct {
			ID       int64  `gorm:"column:id"`
			From     string `gorm:"column:from"`
			OnDelete string `gorm:"column:on_delete"`
		}
		if err := tx.Raw(fmt.Sprintf("PRAGMA foreign_key_list(%q)", v.Table)).Scan(&fks).Error; err != nil {
			return fmt.Errorf("listing foreign keys for %s: %w", v.Table, err)
		}
		for _, fk := range fks {
			if fk.ID != v.Fkid {
				continue
			}
			switch fk.OnDelete {
			case "CASCADE":
				if err := tx.Exec(fmt.Sprintf("DELETE FROM %q WHERE rowid = ?", v.Table), v.Rowid).Error; err != nil {
					return fmt.Errorf("deleting orphaned row %d from %s: %w", v.Rowid, v.Table, err)
				}
				slog.Warn("deleted orphaned row during migration", "table", v.Table, "rowid", v.Rowid, "references", v.Parent)
			case "SET NULL":
				if err := tx.Exec(fmt.Sprintf("UPDATE %q SET %q = NULL WHERE rowid = ?", v.Table, fk.From), v.Rowid).Error; err != nil {
					return fmt.Errorf("clearing orphaned reference in %s row %d: %w", v.Table, v.Rowid, err)
				}
				slog.Warn("cleared orphaned reference during migration", "table", v.Table, "rowid", v.Rowid, "references", v.Parent)
			}
			break
		}
	}
	return nil
}

func init() {
	Register(&gormigrate.Migration{
		ID: "0008_constraint_parity",
		Migrate: func(tx *gorm.DB) error {
			// Rows violating the constraints about to be added would fail
			// the ADD CONSTRAINT on PostgreSQL and the integrity check on
			// SQLite. Resolve them the way the constraints would have.
			cleanups := []string{
				"DELETE FROM forge_instance_events WHERE forge_instance_id NOT IN (SELECT id FROM forge_instances)",
				"UPDATE forge_instances SET endpoint_name = NULL WHERE endpoint_name IS NOT NULL AND endpoint_name NOT IN (SELECT name FROM github_endpoints)",
				"UPDATE forge_instances SET gitea_credentials_id = NULL WHERE gitea_credentials_id IS NOT NULL AND gitea_credentials_id NOT IN (SELECT id FROM gitea_credentials)",
				"UPDATE pools SET forge_instance_id = NULL WHERE forge_instance_id IS NOT NULL AND forge_instance_id NOT IN (SELECT id FROM forge_instances)",
				"UPDATE pools SET proxy_id = NULL WHERE proxy_id IS NOT NULL AND proxy_id NOT IN (SELECT id FROM proxies)",
				"UPDATE scale_sets SET proxy_id = NULL WHERE proxy_id IS NOT NULL AND proxy_id NOT IN (SELECT id FROM proxies)",
				"UPDATE workflow_jobs SET forge_instance_id = NULL WHERE forge_instance_id IS NOT NULL AND forge_instance_id NOT IN (SELECT id FROM forge_instances)",
			}
			for _, stmt := range cleanups {
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("cleaning up orphaned references: %w", err)
				}
			}

			// 0004's DropColumn rebuilds controller_infos on SQLite, which
			// silently drops its deleted_at index. Heal databases that
			// migrated through it.
			if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_controller_infos_deleted_at ON controller_infos(deleted_at)").Error; err != nil {
				return fmt.Errorf("restoring controller_infos index: %w", err)
			}

			// Create only the constraints, through the migrator, instead of
			// a full AutoMigrate: AutoMigrate would also reconcile column
			// types, and the frozen stubs must never fight the live schema
			// (the endpoint name column, for example, changed its collation
			// between the baseline and the current models).
			constraints := []struct {
				model any
				name  string
			}{
				{&forgeInstance0008{}, "fk_forge_instances_events"},
				{&forgeInstance0008{}, "fk_forge_instances_endpoint"},
				{&forgeInstance0008{}, "fk_gitea_credentials_forge_instances"},
				{&forgeInstance0008{}, "fk_forge_instances_pools"},
				{&forgeInstance0008{}, "fk_forge_instances_jobs"},
				{&proxy0008{}, "fk_proxies_pools"},
				{&proxy0008{}, "fk_proxies_scale_sets"},
			}
			createConstraints := func() error {
				for _, c := range constraints {
					if tx.Migrator().HasConstraint(c.model, c.name) {
						continue
					}
					if err := tx.Migrator().CreateConstraint(c.model, c.name); err != nil {
						return fmt.Errorf("creating constraint %s: %w", c.name, err)
					}
				}
				return nil
			}

			if tx.Name() != "sqlite" {
				return createConstraints()
			}

			// Long lived databases can hold orphaned rows from times when
			// foreign key enforcement was not active. Resolve them using
			// each constraint's own delete semantics before adding more
			// constraints on top.
			if err := resolveSQLiteOrphans(tx); err != nil {
				return err
			}

			// SQLite cannot add a constraint in place; GORM rebuilds the
			// table (create temp, copy, drop, rename). With foreign keys
			// enforced, dropping a table that has child rows fails, so
			// wrap the rebuilds in SQLite's documented ALTER procedure.
			// Safe outside a transaction with the single pooled connection.
			//
			// The rebuild also drops the table's standalone indexes without
			// recreating them, so capture their DDL first and restore any
			// that went missing.
			var indexes []struct {
				Name string
				SQL  string
			}
			err := tx.Raw(`SELECT name, sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL AND tbl_name IN ('forge_instance_events', 'forge_instances', 'pools', 'scale_sets', 'workflow_jobs')`).Scan(&indexes).Error
			if err != nil {
				return fmt.Errorf("capturing index definitions: %w", err)
			}

			if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
				return fmt.Errorf("disabling foreign keys: %w", err)
			}
			migrateErr := createConstraints()
			if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
				return errors.Join(migrateErr, fmt.Errorf("re-enabling foreign keys: %w", err))
			}
			if migrateErr != nil {
				return migrateErr
			}

			for _, index := range indexes {
				var count int64
				if err := tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='index' AND name = ?", index.Name).Scan(&count).Error; err != nil {
					return fmt.Errorf("checking index %s: %w", index.Name, err)
				}
				if count == 0 {
					if err := tx.Exec(index.SQL).Error; err != nil {
						return fmt.Errorf("restoring index %s: %w", index.Name, err)
					}
				}
			}

			// Anything still violating at this point has no automatic
			// resolution (a foreign key without a delete action). Such
			// rows predate this migration and never stopped GARM from
			// working, so report them instead of failing the upgrade.
			var violations []fkViolation
			if err := tx.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil {
				return fmt.Errorf("checking foreign keys: %w", err)
			}
			for _, v := range violations {
				slog.Warn("orphaned row left in place after migration", "table", v.Table, "rowid", v.Rowid, "references", v.Parent)
			}
			return nil
		},
	})
}
