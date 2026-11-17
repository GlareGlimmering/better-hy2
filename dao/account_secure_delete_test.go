package dao

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"h-ui/model/constant"
	"h-ui/model/entity"
)

func TestAccountDeletionErasesCredentialFromLiveSQLite(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "SQLite ? data #")
	t.Setenv("HUI_DATA", dataDir)
	if err := InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseSqliteDB() })
	db, err := sqliteDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	first, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.Conn(context.Background())
	if err != nil {
		first.Close()
		t.Fatal(err)
	}
	for _, conn := range []*sql.Conn{first, second} {
		t.Cleanup(func() { _ = conn.Close() })
		var state int
		if err := conn.QueryRowContext(context.Background(), "PRAGMA secure_delete").Scan(&state); err != nil || state != 1 {
			t.Fatalf("secure_delete=%d error=%v", state, err)
		}
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	username, credential := "audit-user-private-marker", "audit-connection-secret-marker"
	id, err := SaveAccount(entity.Account{Username: &username, ConPass: &credential})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dataDir, constant.SqliteDBPath)
	before, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(before, []byte(credential)) {
		t.Fatalf("test marker was never written: %v", err)
	}
	if err := DeleteAccount([]int64{id}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte(credential)) || bytes.Contains(after, []byte(username)) {
		t.Fatal("deleted account or credential remains in live SQLite bytes")
	}
}
