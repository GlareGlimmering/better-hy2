package dao

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"h-ui/model/entity"
)

func TestAccountTrafficSummaryExcludesAdminAndSortsByTotal(t *testing.T) {
	previousDB := sqliteDB
	t.Cleanup(func() { sqliteDB = previousDB })

	var err error
	sqliteDB, err = gorm.Open(sqlite.Open("file:traffic-summary?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := sqliteDB.AutoMigrate(&entity.Account{}); err != nil {
		t.Fatalf("migrate account table: %v", err)
	}

	accounts := []struct {
		username string
		role     string
		upload   int64
		download int64
		deleted  int64
	}{
		{username: "alice", role: "user", upload: 20, download: 80},
		{username: "bob", role: "user", upload: 200, download: 50},
		{username: "carol", role: "user", upload: 40, download: 60, deleted: 1},
		{username: "dave", role: "user", upload: 10, download: 5},
		{username: "sysadmin", role: "admin", upload: 9000, download: 9000},
	}
	for index, item := range accounts {
		id := int64(index + 1)
		username, role := item.username, item.role
		upload, download, deleted := item.upload, item.download, item.deleted
		account := entity.Account{
			Username: &username,
			Role:     &role,
			Upload:   &upload,
			Download: &download,
			Deleted:  &deleted,
			BaseEntity: entity.BaseEntity{
				Id: &id,
			},
		}
		if tx := sqliteDB.Create(&account); tx.Error != nil {
			t.Fatalf("create account %s: %v", item.username, tx.Error)
		}
	}

	upload, download, top, err := AccountTrafficSummary(3)
	if err != nil {
		t.Fatalf("query traffic summary: %v", err)
	}
	if upload != 270 || download != 195 {
		t.Fatalf("unexpected totals: upload=%d download=%d", upload, download)
	}
	if len(top) != 3 {
		t.Fatalf("expected top 3 users, got %d", len(top))
	}
	want := []string{"bob", "alice", "carol"}
	for index, username := range want {
		if top[index].Username != username {
			t.Fatalf("rank %d: got %s want %s (%s)", index+1, top[index].Username, username, fmt.Sprint(top))
		}
	}
}

func TestAccountTrafficSummaryReturnsEmptyList(t *testing.T) {
	previousDB := sqliteDB
	t.Cleanup(func() { sqliteDB = previousDB })

	var err error
	sqliteDB, err = gorm.Open(sqlite.Open("file:empty-traffic-summary?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := sqliteDB.AutoMigrate(&entity.Account{}); err != nil {
		t.Fatalf("migrate account table: %v", err)
	}

	upload, download, top, err := AccountTrafficSummary(10)
	if err != nil {
		t.Fatalf("query empty traffic summary: %v", err)
	}
	if upload != 0 || download != 0 || len(top) != 0 {
		t.Fatalf("unexpected empty summary: upload=%d download=%d top=%v", upload, download, top)
	}
}
