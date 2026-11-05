package dao

import (
	"encoding/json"
	"strings"
	"testing"

	"h-ui/model/constant"
)

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
