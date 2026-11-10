package dao

import (
	"encoding/hex"
	"errors"
	"math"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"h-ui/model/bo"
	"h-ui/model/entity"
)

func initAccountConsistencySchema() error {
	return sqliteDB.Transaction(func(tx *gorm.DB) error {
		if !tx.Migrator().HasColumn(&entity.Account{}, "auth_generation") {
			if err := tx.Exec("ALTER TABLE account ADD COLUMN auth_generation TEXT NOT NULL DEFAULT ''").Error; err != nil {
				return err
			}
		}
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS traffic_batch_receipt (epoch TEXT PRIMARY KEY, sequence INTEGER NOT NULL)`,
			`CREATE TRIGGER IF NOT EXISTS account_auth_generation_insert AFTER INSERT ON account
				BEGIN UPDATE account SET auth_generation=hex(randomblob(16)) WHERE id=NEW.id; END`,
			`CREATE TRIGGER IF NOT EXISTS account_auth_generation_change AFTER UPDATE OF username,pass,role,deleted ON account
				WHEN OLD.username IS NOT NEW.username OR OLD.pass IS NOT NEW.pass OR OLD.role IS NOT NEW.role OR OLD.deleted IS NOT NEW.deleted
				BEGIN UPDATE account SET auth_generation=hex(randomblob(16)) WHERE id=NEW.id; END`,
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SettleTrafficBatch persists the counters and their high-water mark together.
// One receipt per kernel generation suffices because the producer allows only
// one outstanding, sequential batch at a time.
func SettleTrafficBatch(batch bo.Hysteria2TrafficBatch, multiplier float64) error {
	epoch, err := hex.DecodeString(batch.Epoch)
	if err != nil || len(epoch) != 16 || hex.EncodeToString(epoch) != batch.Epoch || batch.Sequence <= 0 || batch.Users == nil ||
		len(batch.Users) > 65536 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier < 0 {
		return errors.New("invalid traffic settlement batch")
	}
	return sqliteDB.Transaction(func(tx *gorm.DB) error {
		// Start with a write, so concurrent transactions do not read stale marks.
		if err := tx.Exec(`INSERT INTO traffic_batch_receipt(epoch,sequence) VALUES (?,0) ON CONFLICT(epoch) DO NOTHING`, batch.Epoch).Error; err != nil {
			return err
		}
		var previous int64
		if err := tx.Raw("SELECT sequence FROM traffic_batch_receipt WHERE epoch=?", batch.Epoch).Scan(&previous).Error; err != nil {
			return err
		}
		if batch.Sequence <= previous {
			return nil
		}
		if batch.Sequence-previous != 1 {
			return errors.New("traffic batch sequence gap")
		}
		for identity, traffic := range batch.Users {
			rawID, ok := strings.CutPrefix(identity, "account:")
			id, err := strconv.ParseInt(rawID, 10, 64)
			if !ok || err != nil || id <= 0 {
				return errors.New("invalid traffic account identity")
			}
			download := float64(traffic.Rx) * multiplier
			upload := float64(traffic.Tx) * multiplier
			if download < 0 || upload < 0 || download >= float64(math.MaxInt64) || upload >= float64(math.MaxInt64) {
				return errors.New("invalid traffic increment")
			}
			// Deleted identities have no remaining ledger. Saturate existing
			// counters instead of letting SQLite overflow to a REAL.
			if err := tx.Exec(`UPDATE account SET
				download=download+min(?,9223372036854775807-download),
				upload=upload+min(?,9223372036854775807-upload) WHERE id=?`,
				int64(download), int64(upload), id).Error; err != nil {
				return err
			}
		}
		return tx.Exec("UPDATE traffic_batch_receipt SET sequence=? WHERE epoch=?", batch.Sequence, batch.Epoch).Error
	})
}
