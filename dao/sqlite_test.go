package dao

import (
	"encoding/json"
	"strings"
	"testing"

	"h-ui/model/constant"
)

func TestLegacyTrafficSecretMigrationRotatesJWTExactlyOnce(t *testing.T) {
	t.Setenv("HUI_DATA", t.TempDir())
	if err := InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	// Recreate the relevant v1.0.0 config-table state.
	if err := sqliteDB.Exec("DELETE FROM config WHERE key = ?", constant.Hysteria2TrafficStatsSecret).Error; err != nil {
		t.Fatal(err)
	}
	const legacy = "previously-shared-key-fixture"
	if err := sqliteDB.Exec("UPDATE config SET value = ? WHERE key = ?", legacy, constant.JwtSecret).Error; err != nil {
		t.Fatal(err)
	}
	var firstJWT, firstStats string
	for i := 0; i < 2; i++ {
		if err := CloseSqliteDB(); err != nil {
			t.Fatal(err)
		}
		if err := InitSqliteSchema(); err != nil {
			t.Fatal(err)
		}
		jwt, err := GetConfig("key = ?", constant.JwtSecret)
		if err != nil {
			t.Fatal(err)
		}
		stats, err := GetConfig("key = ?", constant.Hysteria2TrafficStatsSecret)
		if err != nil {
			t.Fatal(err)
		}
		if *jwt.Value == legacy || *stats.Value == legacy || *jwt.Value == *stats.Value {
			t.Fatal("migration did not replace and separate the previously shared key")
		}
		if i == 0 {
			firstJWT, firstStats = *jwt.Value, *stats.Value
		} else if firstJWT != *jwt.Value || firstStats != *stats.Value {
			t.Fatal("reopening the database unexpectedly changed credentials")
		}
	}
	if err := CloseSqliteDB(); err != nil {
		t.Fatal(err)
	}
}

func TestInitSqlForcesRequestedPanelPort(t *testing.T) {
	t.Setenv("HUI_DATA", t.TempDir())
	if err := InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	if tx := sqliteDB.Exec("UPDATE config SET value = ? WHERE key = ?", "9090", constant.HUIWebPort); tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if err := CloseSqliteDB(); err != nil {
		t.Fatal(err)
	}

	if err := InitSql("8000"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseSqliteDB() })
	var port string
	if tx := sqliteDB.Raw("SELECT value FROM config WHERE key = ?", constant.HUIWebPort).Scan(&port); tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if port != "8000" {
		t.Fatalf("panel port=%q want 8000", port)
	}
}

func TestFreshSchemaContainsNoOutboundOrEnvironmentSeedData(t *testing.T) {
	t.Setenv("HUI_DATA", t.TempDir())
	if err := InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseSqliteDB() })

	var proxyUsers int64
	if tx := sqliteDB.Table("account").Where("role = ?", "user").Count(&proxyUsers); tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if proxyUsers != 0 {
		t.Fatalf("fresh proxy users=%d", proxyUsers)
	}
	var boundUsers int64
	if tx := sqliteDB.Table("account").Where("outbound <> ''").Count(&boundUsers); tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if boundUsers != 0 {
		t.Fatalf("fresh outbound bindings=%d", boundUsers)
	}

	var draft string
	if tx := sqliteDB.Raw("SELECT value FROM config WHERE key = ?", constant.XrayDraftConfig).Scan(&draft); tx.Error != nil {
		t.Fatal(tx.Error)
	}
	var settings struct {
		Version   string            `json:"version"`
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(draft), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Version != constant.XrayPinnedVersion || len(settings.Outbounds) != 0 {
		t.Fatalf("fresh Xray settings=%s", draft)
	}
	for _, forbidden := range []string{"TEST_" + "VLESS", "crosscloud" + "connect"} {
		if strings.Contains(draft, forbidden) {
			t.Fatalf("fresh config contains forbidden seed identifier")
		}
	}
}
