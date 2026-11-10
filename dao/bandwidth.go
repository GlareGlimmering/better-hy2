package dao

import (
	"gorm.io/gorm"
	"h-ui/model/entity"
	"strings"
)

func initBandwidthSchema() error {
	return sqliteDB.Transaction(func(tx *gorm.DB) error {
		for _, column := range []string{
			"upload_mbps INTEGER NOT NULL DEFAULT 0 CHECK(upload_mbps BETWEEN 0 AND 100000)",
			"download_mbps INTEGER NOT NULL DEFAULT 0 CHECK(download_mbps BETWEEN 0 AND 100000)",
			"bandwidth_version INTEGER NOT NULL DEFAULT 1",
		} {
			name := column[:strings.IndexByte(column, ' ')]
			if !tx.Migrator().HasColumn(&entity.Account{}, name) {
				if err := tx.Exec("ALTER TABLE account ADD COLUMN " + column).Error; err != nil {
					return err
				}
			}
		}
		return tx.Exec(`CREATE TRIGGER IF NOT EXISTS account_bandwidth_version AFTER UPDATE OF upload_mbps,download_mbps ON account
			WHEN OLD.upload_mbps IS NOT NEW.upload_mbps OR OLD.download_mbps IS NOT NEW.download_mbps
			BEGIN UPDATE account SET bandwidth_version=OLD.bandwidth_version+1 WHERE id=NEW.id; END`).Error
	})
}
