package dao

import (
	"encoding/hex"
	"errors"
	"strings"

	"gorm.io/gorm"
	"h-ui/util"
)

type PanelAdmin struct {
	ID           int64
	Username     string
	PasswordHash string
	Legacy       bool
	MustChange   bool
}

func (PanelAdmin) TableName() string { return "panel_admin" }

func initPanelAdmins() error {
	return sqliteDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("CREATE TABLE IF NOT EXISTS panel_admin (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, legacy INTEGER NOT NULL DEFAULT 0, must_change INTEGER NOT NULL DEFAULT 0)").Error; err != nil {
			return err
		}
		var admins []struct {
			ID       int64
			Username string
			Pass     string
			ConPass  string
		}
		if err := tx.Raw("SELECT id, username, pass, con_pass FROM account WHERE role = 'admin' AND deleted = 0").Scan(&admins).Error; err != nil {
			return err
		}
		for _, admin := range admins {
			// A known factory password must never grant access, including to the password change endpoint.
			// Unusable legacy credentials can be recovered with the local reset command.
			decoded, err := hex.DecodeString(admin.Pass)
			if err != nil || len(decoded) != 28 || admin.Pass == util.SHA224String("sysadmin") {
				// Preserve the username as a recovery target, with no usable legacy credential.
				if err := tx.Create(&PanelAdmin{ID: admin.ID, Username: admin.Username, MustChange: true}).Error; err != nil {
					return err
				}
				continue
			}
			// Legacy databases may encode con_pass as username.password; a rename may
			// have left the old username prefix. Check both that form and a bare password.
			_, suffix, _ := strings.Cut(admin.ConPass, ".")
			coupled := util.SHA224String(admin.ConPass) == admin.Pass || (suffix != "" && util.SHA224String(suffix) == admin.Pass)
			hash, err := util.HashPanelPassword(admin.Pass)
			if err != nil {
				return err
			}
			if err := tx.Create(&PanelAdmin{ID: admin.ID, Username: admin.Username, PasswordHash: hash, Legacy: true, MustChange: coupled}).Error; err != nil {
				return err
			}
		}
		return tx.Exec("DELETE FROM account WHERE role = 'admin'").Error
	})
}

func PanelAdminByUsername(username string) (PanelAdmin, error) {
	var admin PanelAdmin
	err := sqliteDB.Where("username = ?", username).First(&admin).Error
	return admin, err
}

func PanelAdminByID(id int64) (PanelAdmin, error) {
	var admin PanelAdmin
	err := sqliteDB.First(&admin, id).Error
	return admin, err
}

func ReplacePanelAdminPassword(admin PanelAdmin, hash string) error {
	result := sqliteDB.Model(&PanelAdmin{}).Where("id = ? AND password_hash = ?", admin.ID, admin.PasswordHash).
		Updates(map[string]any{"password_hash": hash, "legacy": false, "must_change": false})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("administrator credentials changed; try again")
	}
	return nil
}

func ResetPanelAdmin(newUsername, hash string) (string, error) {
	var admin PanelAdmin
	err := sqliteDB.Transaction(func(tx *gorm.DB) error {
		err := tx.Order("id ASC").First(&admin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			admin = PanelAdmin{Username: newUsername, PasswordHash: hash}
			return tx.Create(&admin).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&admin).Updates(map[string]any{"password_hash": hash, "legacy": false, "must_change": false}).Error
	})
	return admin.Username, err
}
