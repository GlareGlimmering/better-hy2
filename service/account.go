package service

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/dto"
	"h-ui/model/entity"
	"h-ui/model/vo"
	"maps"
)

type AccountUpdateResult struct {
	OutboundChanged  bool `json:"outboundChanged"`
	Hysteria2Running bool `json:"hysteria2Running"`
	RuntimeApplied   bool `json:"runtimeApplied"`
}

var accountHysteria2IsRunning = Hysteria2IsRunning
var accountApplyHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
	return applyHysteria2Config(config, defaultHysteria2ConfigApplyOps())
}

func Login(username string, pass string) (string, error) {
	account, err := dao.GetAccount("username = ? and pass = ? and role = 'admin' and deleted = 0", username, pass)
	if err != nil {
		return "", err
	}
	accountBo := bo.AccountBo{
		Id:       *account.Id,
		Username: *account.Username,
		Roles:    []string{*account.Role},
		Deleted:  *account.Deleted,
	}
	return GenToken(accountBo)
}

func PageAccount(accountPageDto dto.AccountPageDto) ([]entity.Account, int64, error) {
	return dao.PageAccount(accountPageDto)
}

func SaveAccount(account entity.Account) error {
	_, err := SaveAccountWithResult(account)
	return err
}

func SaveAccountWithResult(account entity.Account) (AccountUpdateResult, error) {
	result := AccountUpdateResult{}
	outbound, err := CanonicalAccountOutbound(AccountOutbound(account))
	if err != nil {
		return result, err
	}
	account.Outbound = &outbound

	hysteria2ConfigApplyMutex.Lock()
	defer hysteria2ConfigApplyMutex.Unlock()
	before, err := dao.ListAccount("role = ? and deleted = 0", "user")
	if err != nil {
		return result, err
	}
	id, err := dao.SaveAccount(account)
	if err != nil {
		return result, err
	}
	result, err = applyAccountRoutingMutation(before)
	if err == nil {
		return result, nil
	}
	if restoreErr := dao.DeleteAccount([]int64{id}); restoreErr != nil {
		return result, fmt.Errorf("%v; remove new account failed: %w", err, restoreErr)
	}
	return result, fmt.Errorf("%w; new account removed", err)
}

func DeleteAccount(ids []int64) error {
	_, err := DeleteAccountWithResult(ids)
	return err
}

func DeleteAccountWithResult(ids []int64) (AccountUpdateResult, error) {
	result := AccountUpdateResult{}
	hysteria2ConfigApplyMutex.Lock()
	defer hysteria2ConfigApplyMutex.Unlock()
	before, err := dao.ListAccount("role = ? and deleted = 0", "user")
	if err != nil {
		return result, err
	}
	removed := make([]entity.Account, 0, len(ids))
	for _, id := range ids {
		account, getErr := GetAccount(id)
		if getErr != nil {
			return result, getErr
		}
		removed = append(removed, account)
	}
	if err := dao.DeleteAccount(ids); err != nil {
		return result, err
	}
	result, err = applyAccountRoutingMutation(before)
	if err == nil {
		return result, nil
	}
	for _, account := range removed {
		if _, restoreErr := dao.SaveAccount(account); restoreErr != nil {
			return result, fmt.Errorf("%v; restore deleted account %q failed: %w", err, *account.Username, restoreErr)
		}
	}
	return result, fmt.Errorf("%w; deleted account restored", err)
}

func UpdateAccount(account entity.Account) error {
	_, err := UpdateAccountWithResult(account)
	return err
}

// UpdateAccountWithResult applies an outbound binding change to both the
// account table and the managed Hysteria2 configuration. The account update is
// restored if the running configuration cannot be applied.
func UpdateAccountWithResult(account entity.Account) (AccountUpdateResult, error) {
	result := AccountUpdateResult{}
	if account.Outbound != nil {
		outbound, err := CanonicalAccountOutbound(*account.Outbound)
		if err != nil {
			return result, err
		}
		account.Outbound = &outbound
	}
	updates := map[string]interface{}{}
	if account.Username != nil && *account.Username != "" {
		updates["username"] = *account.Username
	}
	if account.Pass != nil && *account.Pass != "" {
		updates["pass"] = *account.Pass
	}
	if account.ConPass != nil && *account.ConPass != "" {
		username := ""
		if account.Username != nil {
			username = *account.Username
		} else {
			current, err := GetAccount(*account.Id)
			if err != nil {
				return result, err
			}
			username = *current.Username
		}
		updates["con_pass"] = fmt.Sprintf("%s.%s", username, *account.ConPass)
	}
	if account.Quota != nil {
		updates["quota"] = *account.Quota
	}
	if account.ExpireTime != nil {
		updates["expire_time"] = *account.ExpireTime
	}
	if account.Download != nil {
		updates["download"] = *account.Download
	}
	if account.Upload != nil {
		updates["upload"] = *account.Upload
	}
	if account.DeviceNo != nil {
		updates["device_no"] = *account.DeviceNo
	}
	if account.Deleted != nil {
		updates["deleted"] = *account.Deleted
	}
	if account.LoginAt != nil && *account.LoginAt > 0 {
		updates["login_at"] = *account.LoginAt
	}
	if account.ConAt != nil && *account.ConAt > 0 {
		updates["con_at"] = *account.ConAt
	}
	if account.Remark != nil {
		updates["remark"] = *account.Remark
	}
	if account.Outbound != nil {
		updates["outbound"] = *account.Outbound
	}
	if account.Outbound == nil && account.Username == nil && account.Deleted == nil {
		return result, dao.UpdateAccount([]int64{*account.Id}, updates)
	}

	hysteria2ConfigApplyMutex.Lock()
	defer hysteria2ConfigApplyMutex.Unlock()

	current, err := GetAccount(*account.Id)
	if err != nil {
		return result, err
	}
	previous := accountUpdateSnapshot(current, updates)
	if err := dao.UpdateAccount([]int64{*account.Id}, updates); err != nil {
		return result, err
	}
	before := []entity.Account{}
	if *current.Role == "user" && *current.Deleted == 0 {
		before = append(before, current)
	}
	allBefore, listErr := dao.ListAccount("role = ? and deleted = 0 and id != ?", "user", *account.Id)
	if listErr != nil {
		_ = dao.UpdateAccount([]int64{*account.Id}, previous)
		return result, listErr
	}
	before = append(before, allBefore...)
	bindingFieldChanged := account.Outbound != nil && AccountOutbound(current) != *account.Outbound
	result, err = applyAccountRoutingMutation(before)
	result.OutboundChanged = result.OutboundChanged || bindingFieldChanged
	if err != nil {
		if restoreErr := dao.UpdateAccount([]int64{*account.Id}, previous); restoreErr != nil {
			return result, fmt.Errorf("apply Hysteria2 user outbound: %v; restore account failed: %w", err, restoreErr)
		}
		return result, fmt.Errorf("apply Hysteria2 user outbound: %w; previous account binding restored", err)
	}
	return result, nil
}

func applyAccountRoutingMutation(before []entity.Account) (AccountUpdateResult, error) {
	result := AccountUpdateResult{}
	after, err := dao.ListAccount("role = ? and deleted = 0", "user")
	if err != nil {
		return result, err
	}
	config, err := GetHysteria2Config()
	if err != nil {
		return result, err
	}
	beforeBindings, beforeErr := accountRoutingBindings(config, before)
	afterBindings, afterErr := accountRoutingBindings(config, after)
	if beforeErr == nil && afterErr == nil && len(beforeBindings) > 0 && len(after) > 0 && len(afterBindings) == 0 {
		return result, errors.New("account change would disable strict user routing and expose the default outbound")
	}
	if beforeErr == nil && afterErr == nil && maps.Equal(beforeBindings, afterBindings) {
		return result, nil
	}
	result.OutboundChanged = true
	result.Hysteria2Running = accountHysteria2IsRunning()
	if err := accountApplyHysteria2Config(config); err != nil {
		return result, fmt.Errorf("apply Hysteria2 user outbound: %w", err)
	}
	result.RuntimeApplied = result.Hysteria2Running
	return result, nil
}

func accountRoutingBindings(config bo.Hysteria2ServerConfig, accounts []entity.Account) (map[string]string, error) {
	settings, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil {
		return nil, err
	}
	config, err = MergeManagedOutboundNodes(config, settings)
	if err != nil {
		return nil, err
	}
	return BuildUserOutboundBindings(accounts, config.Outbounds)
}

func accountUpdateSnapshot(account entity.Account, updates map[string]interface{}) map[string]interface{} {
	values := map[string]interface{}{}
	for key := range updates {
		switch key {
		case "username":
			values[key] = *account.Username
		case "pass":
			values[key] = *account.Pass
		case "con_pass":
			values[key] = *account.ConPass
		case "quota":
			values[key] = *account.Quota
		case "expire_time":
			values[key] = *account.ExpireTime
		case "download":
			values[key] = *account.Download
		case "upload":
			values[key] = *account.Upload
		case "device_no":
			values[key] = *account.DeviceNo
		case "deleted":
			values[key] = *account.Deleted
		case "login_at":
			values[key] = *account.LoginAt
		case "con_at":
			values[key] = *account.ConAt
		case "remark":
			values[key] = *account.Remark
		case "outbound":
			values[key] = AccountOutbound(account)
		}
	}
	return values
}

func ResetTraffic(id int64) error {
	return dao.UpdateAccount([]int64{id}, map[string]interface{}{"download": 0, "upload": 0})
}

func ExistAccountUsername(username string, id int64) bool {
	var err error
	if id != 0 {
		_, err = dao.GetAccount("username = ? and id != ?", username, id)
	} else {
		_, err = dao.GetAccount("username = ?", username)
	}
	if err != nil {
		if err.Error() == constant.WrongPassword {
			return false
		}
	}
	return true
}

func GetAccount(id int64) (entity.Account, error) {
	return dao.GetAccount("id = ?", id)
}

func ListExportAccount() ([]bo.AccountExport, error) {
	accounts, err := dao.ListAccount(nil, nil)
	if err != nil {
		return nil, errors.New(constant.SysError)
	}
	var accountExports []bo.AccountExport
	for _, item := range accounts {
		accountExport := bo.AccountExport{
			Id:           *item.Id,
			Username:     *item.Username,
			Pass:         *item.Pass,
			ConPass:      *item.ConPass,
			Quota:        *item.Quota,
			Download:     *item.Download,
			Upload:       *item.Upload,
			ExpireTime:   *item.ExpireTime,
			DeviceNo:     *item.DeviceNo,
			KickUtilTime: *item.KickUtilTime,
			Role:         *item.Role,
			Deleted:      *item.Deleted,
			CreateTime:   *item.CreateTime,
			UpdateTime:   *item.UpdateTime,
			LoginAt:      *item.LoginAt,
			ConAt:        *item.ConAt,
			Remark:       *item.Remark,
			Outbound:     AccountOutbound(item),
		}
		accountExports = append(accountExports, accountExport)
	}
	return accountExports, nil
}

func AccountOutbound(account entity.Account) string {
	if account.Outbound == nil {
		return ""
	}
	return *account.Outbound
}

func ReleaseKickAccount(id int64) error {
	return dao.UpdateAccount([]int64{id}, map[string]interface{}{"kick_util_time": 0})
}

func UpsertAccount(accounts []entity.Account) error {
	hysteria2ConfigApplyMutex.Lock()
	defer hysteria2ConfigApplyMutex.Unlock()
	before, err := dao.ListAccount("role = ? and deleted = 0", "user")
	if err != nil {
		return err
	}
	previous := make(map[string]entity.Account, len(accounts))
	for _, account := range accounts {
		if account.Username == nil {
			continue
		}
		if existing, getErr := dao.GetAccount("username = ?", *account.Username); getErr == nil {
			previous[*account.Username] = existing
		}
	}
	for i := range accounts {
		if accounts[i].Outbound != nil || accounts[i].Username == nil {
			continue
		}
		existing, err := dao.GetAccount("username = ?", *accounts[i].Username)
		if err == nil {
			outbound := AccountOutbound(existing)
			accounts[i].Outbound = &outbound
		}
	}
	for i := range accounts {
		outbound, err := CanonicalAccountOutbound(AccountOutbound(accounts[i]))
		if err != nil {
			return err
		}
		accounts[i].Outbound = &outbound
	}
	if err := dao.UpsertAccount(accounts); err != nil {
		return err
	}
	_, err = applyAccountRoutingMutation(before)
	if err == nil {
		return nil
	}
	for _, account := range accounts {
		if account.Username == nil {
			continue
		}
		if old, exists := previous[*account.Username]; exists {
			if _, restoreErr := dao.SaveAccount(old); restoreErr != nil {
				return fmt.Errorf("%v; restore imported account %q failed: %w", err, *account.Username, restoreErr)
			}
			continue
		}
		created, getErr := dao.GetAccount("username = ?", *account.Username)
		if getErr == nil {
			if restoreErr := dao.DeleteAccount([]int64{*created.Id}); restoreErr != nil {
				return fmt.Errorf("%v; remove imported account %q failed: %w", err, *account.Username, restoreErr)
			}
		}
	}
	return fmt.Errorf("%w; imported accounts restored", err)
}

func GetAccountInfo(c *gin.Context) (vo.AccountInfoVo, error) {
	myClaims, err := ParseToken(GetToken(c))
	if err != nil {
		return vo.AccountInfoVo{}, err
	}
	if myClaims.AccountBo.Deleted != 0 {
		return vo.AccountInfoVo{}, errors.New("this account has been disabled")
	}
	return vo.AccountInfoVo{
		Id:       myClaims.AccountBo.Id,
		Username: myClaims.AccountBo.Username,
		Roles:    myClaims.AccountBo.Roles,
	}, nil
}
