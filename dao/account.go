package dao

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"h-ui/model/constant"
	"h-ui/model/dto"
	"h-ui/model/entity"
	"time"
)

type AccountTrafficRow struct {
	Username string
	Upload   int64
	Download int64
}

func AccountTrafficSummary(limit int) (int64, int64, []AccountTrafficRow, error) {
	var totals struct {
		Upload   int64
		Download int64
	}
	if tx := sqliteDB.Model(&entity.Account{}).
		Select("COALESCE(SUM(upload), 0) AS upload, COALESCE(SUM(download), 0) AS download").
		Where("role = ?", "user").
		Scan(&totals); tx.Error != nil {
		logrus.Errorf("%v", tx.Error)
		return 0, 0, nil, errors.New(constant.SysError)
	}

	rows := make([]AccountTrafficRow, 0)
	query := sqliteDB.Model(&entity.Account{}).
		Select("username, upload, download").
		Where("role = ?", "user").
		Order("(upload + download) DESC, username ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if tx := query.Scan(&rows); tx.Error != nil {
		logrus.Errorf("%v", tx.Error)
		return 0, 0, nil, errors.New(constant.SysError)
	}
	return totals.Upload, totals.Download, rows, nil
}

func SaveAccount(account entity.Account) (int64, error) {
	if tx := sqliteDB.Save(&account); tx.Error != nil {
		logrus.Errorf("%v", tx.Error)
		return 0, errors.New(constant.SysError)
	}
	return *account.Id, nil
}

func DeleteAccount(ids []int64) error {
	if tx := sqliteDB.Where("id in ?", ids).Delete(&entity.Account{}); tx.Error != nil {
		logrus.Errorf("%v", tx.Error)
		return errors.New(constant.SysError)
	}
	return nil
}

func UpdateAccount(ids []int64, updates map[string]interface{}) error {
	if len(updates) > 0 {
		updates["update_time"] = time.Now().Format("2006-01-02 15:04:05")
		if tx := sqliteDB.Model(&entity.Account{}).
			Where("id in ?", ids).
			Updates(updates); tx.Error != nil {
			logrus.Errorf("%v", tx.Error)
			return errors.New(constant.SysError)
		}
	}
	return nil
}

// CreateAccountWithOutbounds commits the new account and its explicitly requested
// changes to existing bindings together.
func CreateAccountWithOutbounds(account entity.Account, bindings map[int64]string) (int64, error) {
	err := sqliteDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&account).Error; err != nil {
			return errors.New("could not create proxy account")
		}
		return updateAccountOutbounds(tx, bindings)
	})
	if err != nil {
		return 0, err
	}
	return *account.Id, nil
}

func RestoreAccountCreation(id int64, previous map[int64]string) error {
	return sqliteDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&entity.Account{}, id).Error; err != nil {
			return errors.New("could not remove the new proxy account")
		}
		return updateAccountOutbounds(tx, previous)
	})
}

// UpdateAccountOutbounds changes a set of active proxy bindings in one transaction.
func UpdateAccountOutbounds(bindings map[int64]string) error {
	return sqliteDB.Transaction(func(tx *gorm.DB) error {
		return updateAccountOutbounds(tx, bindings)
	})
}

func updateAccountOutbounds(tx *gorm.DB, bindings map[int64]string) error {
	for id, outbound := range bindings {
		update := tx.Model(&entity.Account{}).
			Where("id = ? and role = ? and deleted = 0", id, "user").
			Updates(map[string]interface{}{
				"outbound":    outbound,
				"update_time": time.Now().Format("2006-01-02 15:04:05"),
			})
		if update.Error != nil || update.RowsAffected != 1 {
			return errors.New("could not update account outbound bindings")
		}
	}
	return nil
}

func UpdateAccountTraffic(username string, download int64, upload int64) error {
	if upload != 0 || download != 0 {
		updates := map[string]interface{}{}
		if download != 0 {
			updates["download"] = gorm.Expr("download + ?", download)
		}
		if upload != 0 {
			updates["upload"] = gorm.Expr("upload + ?", upload)
		}
		updates["update_time"] = time.Now().Format("2006-01-02 15:04:05")
		if tx := sqliteDB.Model(&entity.Account{}).
			Where("username = ?", username).
			Updates(updates); tx.Error != nil {
			logrus.Errorf("%v", tx.Error)
			return errors.New(constant.SysError)
		}
	}
	return nil
}

func GetAccount(query interface{}, args ...interface{}) (entity.Account, error) {
	var account entity.Account
	if tx := sqliteDB.Model(&entity.Account{}).
		Where(query, args...).First(&account); tx.Error != nil {
		if tx.Error == gorm.ErrRecordNotFound {
			return account, errors.New(constant.WrongPassword)
		}
		logrus.Errorf("%v", tx.Error)
		return account, errors.New(constant.SysError)
	}
	return account, nil
}

// FindHysteria2AuthAccount distinguishes an unknown credential from a database
// failure without logging the credential or combining account policy predicates.
func FindHysteria2AuthAccount(password string) (entity.Account, bool, error) {
	var account entity.Account
	err := sqliteDB.Where("con_pass = ? and role = 'user'", password).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return account, false, nil
	}
	if err != nil {
		return account, false, errors.New(constant.SysError)
	}
	return account, true, nil
}

func PageAccount(accountPageDto dto.AccountPageDto) ([]entity.Account, int64, error) {
	var accounts []entity.Account
	var total int64
	tx := sqliteDB.Model(&entity.Account{})
	if accountPageDto.Username != nil && *accountPageDto.Username != "" {
		tx.Where("username like ?", fmt.Sprintf("%%%s%%", *accountPageDto.Username))
	}
	if accountPageDto.Deleted != nil {
		tx.Where("deleted = ?", *accountPageDto.Deleted)
	}
	if accountPageDto.Remark != nil && *accountPageDto.Remark != "" {
		tx.Where("remark like ?", fmt.Sprintf("%%%s%%", *accountPageDto.Remark))
	}
	tx.Count(&total)
	if tx.Scopes(Paginate(accountPageDto.PageNum, accountPageDto.PageSize)).
		Order("role,create_time desc").
		Find(&accounts); tx.Error != nil {
		logrus.Errorf("%v", tx.Error)
		return accounts, 0, errors.New(constant.SysError)
	}
	return accounts, total, nil
}

func ListAccount(query interface{}, args ...interface{}) ([]entity.Account, error) {
	var accounts []entity.Account
	if tx := sqliteDB.Model(&entity.Account{}).
		Where(query, args...).Order("role,create_time desc").Find(&accounts); tx.Error != nil {
		logrus.Errorf("%v", tx.Error)
		return accounts, errors.New(constant.SysError)
	}
	return accounts, nil
}
