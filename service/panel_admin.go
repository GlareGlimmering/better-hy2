package service

import (
	"errors"

	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/util"
)

var ErrPanelPasswordChangeRequired = errors.New("administrator password change required")

// Bound concurrent Argon2 allocations even when the panel is accessed from many IPs.
var panelAuthSlots = make(chan struct{}, 4)

func Login(username, password string) (string, error) {
	return authenticatePanelAdmin(username, password, "", util.VerifyPanelPassword)
}

func ChangePanelPassword(username, currentPassword, newPassword string) (string, error) {
	if len(newPassword) < 12 || len(newPassword) > 128 || currentPassword == newPassword {
		return "", errors.New("choose a different administrator password of 12 to 128 bytes")
	}
	return authenticatePanelAdmin(username, currentPassword, newPassword, util.VerifyPanelPassword)
}

func authenticatePanelAdmin(username, password, newPassword string, verify func(string, string) bool) (string, error) {
	select {
	case panelAuthSlots <- struct{}{}:
		defer func() { <-panelAuthSlots }()
	default:
		return "", errors.New("administrator login busy; try again")
	}
	admin, err := dao.PanelAdminByUsername(username)
	input := password
	if admin.Legacy {
		input = util.SHA224String(password)
	}
	verified := verify(admin.PasswordHash, input)
	if err != nil || !verified {
		return "", errors.New(constant.WrongPassword)
	}
	if admin.MustChange && newPassword == "" {
		return "", ErrPanelPasswordChangeRequired
	}
	if newPassword != "" || admin.Legacy {
		if newPassword == "" {
			newPassword = password
		}
		hash, err := util.HashPanelPassword(newPassword)
		if err != nil {
			return "", errors.New(constant.SysError)
		}
		if err := dao.ReplacePanelAdminPassword(admin, hash); err != nil {
			return "", errors.New(constant.SysError)
		}
		admin.PasswordHash = hash
	}
	return genToken(bo.AccountBo{Id: admin.ID, Username: admin.Username, Roles: []string{"admin"}}, admin.PasswordHash)
}
