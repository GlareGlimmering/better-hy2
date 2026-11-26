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
	"strings"
)

type AccountUpdateResult struct {
	OutboundChanged  bool   `json:"outboundChanged"`
	Hysteria2Running bool   `json:"hysteria2Running"`
	BandwidthStatus  string `json:"bandwidthStatus,omitempty"`
	RuntimeApplied   bool   `json:"runtimeApplied"`
}

var accountHysteria2IsRunning = Hysteria2IsRunning
var accountApplyHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
	return applyHysteria2Config(config, defaultHysteria2ConfigApplyOps())
}

func PageAccount(accountPageDto dto.AccountPageDto) ([]entity.Account, int64, error) {
	return dao.PageAccount(accountPageDto)
}

func SaveAccount(account entity.Account) error {
	_, err := SaveAccountWithResult(account, nil)
	return err
}

func SaveAccountWithResult(account entity.Account, existingBindings map[string]string) (AccountUpdateResult, error) {
	result := AccountUpdateResult{}
	hysteria2ConfigApplyMutex.Lock()
	defer hysteria2ConfigApplyMutex.Unlock()

	outbound, err := CanonicalAccountOutbound(AccountOutbound(account))
	if err != nil {
		return result, err
	}
	account.Outbound = &outbound
	if account.Role == nil {
		role := "user"
		account.Role = &role
	}
	if len(existingBindings) > 0 && (outbound == "" || *account.Role != "user" || (account.Deleted != nil && *account.Deleted != 0)) {
		return result, errors.New("creating with existing bindings requires an enabled proxy account with an outbound")
	}
	if err := validateBandwidth(account); err != nil {
		return result, err
	}
	before, err := dao.ListAccount("role = ? and deleted = 0", "user")
	if err != nil {
		return result, err
	}
	config, err := GetHysteria2Config()
	if err != nil {
		return result, err
	}
	changes, previous, err := planAccountOutboundBindings(config, before, existingBindings, &account)
	if err != nil {
		return result, err
	}
	id, err := dao.CreateAccountWithOutbounds(account, changes)
	if err != nil {
		return result, err
	}
	result, err = applyAccountRoutingMutation(before)
	if err == nil {
		if account.UploadMbps != nil || account.DownloadMbps != nil {
			applyAccountBandwidth(id, &result)
		}
		return result, nil
	}
	if restoreErr := dao.RestoreAccountCreation(id, previous); restoreErr != nil {
		return result, fmt.Errorf("%w; restore account creation failed: %v", err, restoreErr)
	}
	if len(previous) > 0 {
		return result, fmt.Errorf("%w; new account removed and previous account bindings restored", err)
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
	updates := map[string]interface{}{}
	if account.Username != nil && *account.Username != "" {
		updates["username"] = *account.Username
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
	hasBandwidth := account.UploadMbps != nil || account.DownloadMbps != nil
	if account.UploadMbps != nil {
		updates["upload_mbps"] = *account.UploadMbps
	}
	if account.DownloadMbps != nil {
		updates["download_mbps"] = *account.DownloadMbps
	}
	if account.Outbound == nil && account.Username == nil && account.Deleted == nil && !hasBandwidth {
		return result, dao.UpdateAccount([]int64{*account.Id}, updates)
	}

	hysteria2ConfigApplyMutex.Lock()
	defer hysteria2ConfigApplyMutex.Unlock()

	if account.Outbound != nil {
		outbound, err := CanonicalAccountOutbound(*account.Outbound)
		if err != nil {
			return result, err
		}
		account.Outbound = &outbound
		updates["outbound"] = outbound
	}
	if err := validateBandwidth(account); err != nil {
		return result, err
	}
	current, err := GetAccount(*account.Id)
	if err != nil {
		return result, err
	}
	if hasBandwidth && *current.Role != "user" {
		return result, errors.New("bandwidth can only be configured for proxy accounts")
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
		if hasBandwidth {
			applyAccountBandwidth(*account.Id, &result)
			if result.BandwidthStatus == "unconfirmed" {
				return result, fmt.Errorf("apply Hysteria2 user outbound: %w; account restored but runtime bandwidth could not be confirmed", err)
			}
		}
		return result, fmt.Errorf("apply Hysteria2 user outbound: %w; previous account binding restored", err)
	}
	if hasBandwidth {
		applyAccountBandwidth(*account.Id, &result)
	}
	return result, nil
}

// BindUnassignedAccountOutbounds lets existing installations enter strict
// routing without choosing an implicit outbound for any account.
func BindUnassignedAccountOutbounds(requested map[string]string) (AccountUpdateResult, error) {
	result := AccountUpdateResult{}
	hysteria2ConfigApplyMutex.Lock()
	defer hysteria2ConfigApplyMutex.Unlock()

	before, err := dao.ListAccount("role = ? and deleted = 0", "user")
	if err != nil {
		return result, err
	}
	config, err := GetHysteria2Config()
	if err != nil {
		return result, err
	}
	if len(requested) == 0 {
		return result, errors.New("select outbounds for the unbound accounts")
	}
	changes, previous, err := planAccountOutboundBindings(config, before, requested, nil)
	if err != nil {
		return result, err
	}
	if err := dao.UpdateAccountOutbounds(changes); err != nil {
		return result, err
	}
	result, err = applyAccountRoutingMutation(before)
	if err == nil {
		return result, nil
	}
	if restoreErr := dao.UpdateAccountOutbounds(previous); restoreErr != nil {
		return result, fmt.Errorf("%w; restore account bindings failed: %v", err, restoreErr)
	}
	return result, fmt.Errorf("%w; previous account bindings restored", err)
}

func planAccountOutboundBindings(config bo.Hysteria2ServerConfig, before []entity.Account, requested map[string]string, newAccount *entity.Account) (changes, previous map[int64]string, err error) {
	after := append([]entity.Account(nil), before...)
	previous = make(map[int64]string, len(requested))
	if len(requested) > 0 {
		for i := range after {
			account := &after[i]
			if strings.TrimSpace(AccountOutbound(*account)) != "" {
				continue
			}
			username := *account.Username
			name := strings.TrimSpace(requested[username])
			if name == "" {
				return nil, nil, fmt.Errorf("select an outbound for account %q", username)
			}
			previous[*account.Id] = AccountOutbound(*account)
			account.Outbound = &name
		}
		if len(previous) != len(requested) {
			return nil, nil, errors.New("the list of unbound accounts has changed; reload and try again")
		}
	}
	if newAccount != nil {
		after = append(after, *newAccount)
	}
	bindings, err := accountRoutingBindings(config, after)
	if err != nil {
		return nil, nil, err
	}
	changes = make(map[int64]string, len(previous))
	for _, account := range after[:len(before)] {
		if _, changed := previous[*account.Id]; changed {
			changes[*account.Id] = bindings[*account.Username]
		}
	}
	return changes, previous, nil
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
		case "con_pass":
			values[key] = *account.ConPass
		case "upload_mbps":
			values[key] = accountBandwidth(account).UploadMbps
		case "download_mbps":
			values[key] = accountBandwidth(account).DownloadMbps
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

func AccountOutbound(account entity.Account) string {
	if account.Outbound == nil {
		return ""
	}
	return *account.Outbound
}

func ReleaseKickAccount(id int64) error {
	return dao.UpdateAccount([]int64{id}, map[string]interface{}{"kick_util_time": 0})
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
