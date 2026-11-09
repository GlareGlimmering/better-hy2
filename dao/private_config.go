package dao

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"

	"gorm.io/gorm"
	"h-ui/model/constant"
	"h-ui/util"
)

func protectSQLiteFiles(path string) error {
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if err := util.ProtectPrivateFile(path + suffix); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return file.Close()
}

func initIndependentTrafficSecret() error {
	// Create the Traffic Stats key and rotate the previously shared key in
	// one transaction. The existing config table is the migration marker.
	var keys [64]byte
	if _, err := rand.Read(keys[:]); err != nil {
		return err
	}
	err := sqliteDB.Transaction(func(tx *gorm.DB) error {
		created := tx.Exec(`INSERT INTO config (key, value, remark)
			VALUES (?, ?, ?) ON CONFLICT (key) DO NOTHING`,
			constant.Hysteria2TrafficStatsSecret, hex.EncodeToString(keys[:32]), "Private Traffic Stats credential")
		if created.Error != nil || created.RowsAffected == 0 {
			return created.Error
		}
		return tx.Exec("UPDATE config SET value = ? WHERE key = ?", hex.EncodeToString(keys[32:]), constant.JwtSecret).Error
	})
	if err != nil {
		return errors.New("initialize independent Traffic Stats credential")
	}
	return nil
}
