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
	return authenticatePanelAdmin(username, password, util.VerifyPanelPassword)
}

func authenticatePanelAdmin(username, password string, verify func(string, string) bool) (string, error) {
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
	if admin.MustChange {
		return "", ErrPanelPasswordChangeRequired
	}
	if admin.Legacy {
		hash, err := util.HashPanelPassword(password)
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
