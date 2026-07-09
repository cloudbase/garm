package migrations

import (
	"testing"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Verify 0006 applies cleanly on an existing database that already has
// pools and scale_sets tables without the proxy_id column. Fresh databases
// take the initSchema path, so this is the only coverage for the
// gormigrate upgrade path.
func TestProxyMigrationOnExistingDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.Exec("CREATE TABLE pools (id text PRIMARY KEY, provider_name text)").Error; err != nil {
		t.Fatalf("failed to create pools: %v", err)
	}
	if err := db.Exec("CREATE TABLE scale_sets (id integer PRIMARY KEY, name text)").Error; err != nil {
		t.Fatalf("failed to create scale_sets: %v", err)
	}

	var target []*gormigrate.Migration
	for _, m := range All() {
		if m.ID == "0006_proxies" {
			target = append(target, m)
		}
	}
	if len(target) != 1 {
		t.Fatalf("expected to find 0006_proxies migration, got %d", len(target))
	}

	m := gormigrate.New(db, gormigrate.DefaultOptions, target)
	if err := m.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	if !db.Migrator().HasTable("proxies") {
		t.Fatal("proxies table was not created")
	}
	for _, col := range []string{"name", "http_proxy", "https_proxy", "no_proxy", "credentials"} {
		if !db.Migrator().HasColumn(&proxy0006{}, col) {
			t.Fatalf("proxies table is missing column %q", col)
		}
	}
	if !db.Migrator().HasColumn(&pool0006{}, "proxy_id") {
		t.Fatal("pools table is missing proxy_id")
	}
	if !db.Migrator().HasColumn(&scaleSet0006{}, "proxy_id") {
		t.Fatal("scale_sets table is missing proxy_id")
	}

	// unique name index must reject duplicate names case-insensitively
	if err := db.Exec("INSERT INTO proxies (name, http_proxy) VALUES ('airgap', 'http://proxy:3128')").Error; err != nil {
		t.Fatalf("failed to insert proxy: %v", err)
	}
	if err := db.Exec("INSERT INTO proxies (name, http_proxy) VALUES ('AIRGAP', 'http://proxy:3128')").Error; err == nil {
		t.Fatal("duplicate proxy name was not rejected")
	}
}
