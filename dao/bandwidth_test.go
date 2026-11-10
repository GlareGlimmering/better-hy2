package dao

import (
	"testing"

	"h-ui/model/entity"
)

func TestBandwidthSchemaPreservesLegacyQuotaAndVersionsUpdates(t *testing.T) {
	t.Setenv("HUI_DATA", t.TempDir())
	if err := InitSqliteDB(); err != nil {
		t.Fatal(err)
	}
	defer CloseSqliteDB()
	if err := sqliteInit(sqlInitStr); err != nil {
		t.Fatal(err)
	}
	if err := sqliteDB.Exec("UPDATE account SET quota=-1,upload=123,download=456 WHERE id=1").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := initBandwidthSchema(); err != nil {
			t.Fatal(err)
		}
	}
	var a entity.Account
	check := func(up, down int64, version uint64) {
		t.Helper()
		var err error
		a, err = GetAccount("id = ?", 1)
		if err != nil {
			t.Fatal(err)
		}
		if *a.Quota != -1 || *a.Upload != 123 || *a.Download != 456 || *a.UploadMbps != up ||
			*a.DownloadMbps != down || a.BandwidthVersion != version {
			t.Fatal("unexpected quota, usage, bandwidth or policy version after migration/update")
		}
	}
	check(0, 0, 1)
	for _, rate := range []int64{5, 5} {
		if err := UpdateAccount([]int64{1}, map[string]interface{}{"upload_mbps": rate}); err != nil {
			t.Fatal(err)
		}
		check(5, 0, 2)
	}
	if err := sqliteDB.Exec("UPDATE account SET download_mbps=-1 WHERE id=1").Error; err == nil {
		t.Fatal("negative bandwidth accepted")
	}
	check(5, 0, 2)
}
