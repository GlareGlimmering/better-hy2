package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/entity"
)

const accountRoutingTestConfig = `listen: :443
acme:
  domains:
    - tls.example.test
outbounds:
  - name: DIRECT
    type: direct
    direct:
      mode: auto
  - name: TEST_VLESS
    type: socks5
    socks5:
      addr: 127.0.0.1:10808
  - name: ISP_SOCKS5_1
    type: socks5
    socks5:
      addr: 192.0.2.10:1080
  - name: US_HTTP_1
    type: socks5
    socks5:
      addr: 127.0.0.1:10809
trafficStats:
  listen: 127.0.0.1:9999
`

func TestLoginAllowsAdministratorsOnly(t *testing.T) {
	_, _ = setupSubscriptionTestDatabase(t)

	if _, err := Login("subscription-test", "unused"); err == nil {
		t.Fatal("regular user unexpectedly logged in")
	}
	if _, err := Login("sysadmin", "02f382b76ca1ab7aa06ab03345c7712fd5b971fb0c0f2aef98bac9cd"); err != nil {
		t.Fatalf("administrator login failed: %v", err)
	}
}

func TestLegacyAccountImportExportCompatibility(t *testing.T) {
	_, _ = setupSubscriptionTestDatabase(t)
	username, loginPassword, connectionPassword := "legacy-user", "legacy-password-hash", "legacy-user.connection-secret"
	quota, download, upload := int64(4096), int64(100), int64(200)
	expire, kick, devices, deleted := time.Now().Add(24*time.Hour).UnixMilli(), int64(0), int64(2), int64(0)
	role, remark := "user", "legacy import"
	if err := UpsertAccount([]entity.Account{{
		Username: &username, Pass: &loginPassword, ConPass: &connectionPassword,
		Quota: &quota, Download: &download, Upload: &upload, ExpireTime: &expire,
		KickUtilTime: &kick, DeviceNo: &devices, Role: &role, Deleted: &deleted, Remark: &remark,
	}}); err != nil {
		t.Fatalf("UpsertAccount() error = %v", err)
	}
	exported, err := ListExportAccount()
	if err != nil {
		t.Fatalf("ListExportAccount() error = %v", err)
	}
	for _, account := range exported {
		if account.Username == username {
			if account.Pass != loginPassword || account.ConPass != connectionPassword {
				t.Fatalf("legacy credentials changed during import/export: %#v", account)
			}
			return
		}
	}
	t.Fatal("legacy account missing from export")
}

func TestConnectionPasswordUpdateDoesNotNeedLoginPassword(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	newConnectionPassword := "new-connection-secret"
	if err := UpdateAccount(entity.Account{
		BaseEntity: entity.BaseEntity{Id: &accountID},
		ConPass:    &newConnectionPassword,
	}); err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	account, err := dao.GetAccount("id = ?", accountID)
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if got := *account.ConPass; got != "subscription-test."+newConnectionPassword {
		t.Fatalf("connection password = %q", got)
	}
	if got := *account.Pass; got != "unused" {
		t.Fatalf("legacy login password changed to %q", got)
	}
}

func TestAccountOutboundMigrationUpdateAndExport(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	account, err := dao.GetAccount("id = ?", accountID)
	if err != nil {
		t.Fatalf("GetAccount() after migration error = %v", err)
	}
	if got := AccountOutbound(account); got != "" {
		t.Fatalf("legacy account outbound = %q, want empty", got)
	}

	outbound := "TEST_VLESS"
	if err := UpdateAccount(entity.Account{
		BaseEntity: entity.BaseEntity{Id: &accountID},
		Outbound:   &outbound,
	}); err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	exported, err := ListExportAccount()
	if err != nil {
		t.Fatalf("ListExportAccount() error = %v", err)
	}
	for _, item := range exported {
		if item.Id == accountID {
			if item.Outbound != outbound {
				t.Fatalf("exported outbound = %q", item.Outbound)
			}
			return
		}
	}
	t.Fatal("updated account missing from export")
}

func TestLegacyImportPreservesExistingOutboundBinding(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	outbound := "DIRECT"
	if err := UpdateAccount(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &outbound}); err != nil {
		t.Fatal(err)
	}

	username := "subscription-test"
	legacyPassword := "legacy"
	if err := UpsertAccount([]entity.Account{{Username: &username, Pass: &legacyPassword}}); err != nil {
		t.Fatalf("UpsertAccount() error = %v", err)
	}
	account, err := dao.GetAccount("id = ?", accountID)
	if err != nil {
		t.Fatal(err)
	}
	if got := AccountOutbound(account); got != outbound {
		t.Fatalf("outbound after legacy import = %q, want %q", got, outbound)
	}
}

func TestAccountOutboundUpdateRejectsUnknownName(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	outbound := "MISSING"
	err := UpdateAccount(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &outbound})
	if err == nil || err.Error() != `unknown outbound "MISSING"` {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	account, getErr := dao.GetAccount("id = ?", accountID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if got := AccountOutbound(account); got != "" {
		t.Fatalf("outbound changed after rejected update: %q", got)
	}
}

func TestAccountOutboundUpdateStoresCanonicalName(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	outbound := "test_vless"
	if err := UpdateAccount(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &outbound}); err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	account, err := dao.GetAccount("id = ?", accountID)
	if err != nil {
		t.Fatal(err)
	}
	if got := AccountOutbound(account); got != "TEST_VLESS" {
		t.Fatalf("stored outbound = %q, want canonical name", got)
	}
}

func TestAccountOutboundChangeAppliesRunningHysteria2(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	applyCalls := 0
	accountHysteria2IsRunning = func() bool { return true }
	accountApplyHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
		applyCalls++
		prepared, err := PrepareHysteria2OutboundConfig(config)
		if err != nil {
			return err
		}
		if prepared.UserOutbounds["subscription-test"] != "DIRECT" {
			t.Fatalf("userOutbounds = %#v", prepared.UserOutbounds)
		}
		return nil
	}

	outbound := "DIRECT"
	result, err := UpdateAccountWithResult(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &outbound})
	if err != nil {
		t.Fatalf("UpdateAccountWithResult() error = %v", err)
	}
	if applyCalls != 1 || !result.OutboundChanged || !result.Hysteria2Running || !result.RuntimeApplied {
		t.Fatalf("result = %+v applyCalls = %d", result, applyCalls)
	}

	if _, err := UpdateAccountWithResult(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &outbound}); err != nil {
		t.Fatal(err)
	}
	if applyCalls != 1 {
		t.Fatalf("unchanged outbound triggered apply; calls = %d", applyCalls)
	}
}

func TestAccountOutboundApplyFailureRestoresBinding(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	accountHysteria2IsRunning = func() bool { return true }
	accountApplyHysteria2Config = func(bo.Hysteria2ServerConfig) error {
		return errors.New("start failed")
	}
	outbound := "DIRECT"
	_, err := UpdateAccountWithResult(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &outbound})
	if err == nil || !strings.Contains(err.Error(), "previous account binding restored") {
		t.Fatalf("error = %v", err)
	}
	account, getErr := GetAccount(accountID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if got := AccountOutbound(account); got != "" {
		t.Fatalf("outbound after failed apply = %q", got)
	}
}

func TestAccountOutboundSaveReportsStoppedHysteria2(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	accountHysteria2IsRunning = func() bool { return false }
	accountApplyHysteria2Config = func(bo.Hysteria2ServerConfig) error { return nil }
	outbound := "DIRECT"
	result, err := UpdateAccountWithResult(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &outbound})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OutboundChanged || result.Hysteria2Running || result.RuntimeApplied {
		t.Fatalf("result = %+v", result)
	}
}

func TestAccountCreateAndDeleteAutomaticallyApplyUserOutbounds(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	setAccountRoutingTestConfig(t)
	if err := dao.UpdateAccount([]int64{accountID}, map[string]interface{}{"outbound": "TEST_VLESS"}); err != nil {
		t.Fatal(err)
	}

	var applied []map[string]string
	accountHysteria2IsRunning = func() bool { return true }
	accountApplyHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
		prepared, err := PrepareHysteria2OutboundConfig(config)
		if err != nil {
			return err
		}
		bindings := make(map[string]string, len(prepared.UserOutbounds))
		for username, outbound := range prepared.UserOutbounds {
			bindings[username] = outbound
		}
		applied = append(applied, bindings)
		return nil
	}

	username, outbound := "tousau", "ISP_SOCKS5_1"
	result, err := SaveAccountWithResult(entity.Account{Username: &username, Outbound: &outbound})
	if err != nil {
		t.Fatalf("SaveAccountWithResult() error = %v", err)
	}
	if !result.OutboundChanged || !result.RuntimeApplied || len(applied) != 1 {
		t.Fatalf("save result = %+v applied = %#v", result, applied)
	}
	if applied[0]["subscription-test"] != "TEST_VLESS" || applied[0][username] != outbound {
		t.Fatalf("created user bindings = %#v", applied[0])
	}
	created, err := dao.GetAccount("username = ?", username)
	if err != nil {
		t.Fatal(err)
	}
	result, err = DeleteAccountWithResult([]int64{*created.Id})
	if err != nil {
		t.Fatalf("DeleteAccountWithResult() error = %v", err)
	}
	if !result.OutboundChanged || !result.RuntimeApplied || len(applied) != 2 {
		t.Fatalf("delete result = %+v applied = %#v", result, applied)
	}
	if _, exists := applied[1][username]; exists || applied[1]["subscription-test"] != "TEST_VLESS" {
		t.Fatalf("deleted user bindings = %#v", applied[1])
	}
}

func TestAccountOutboundTransitionsApplyWithoutRestartingForNoop(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	setAccountRoutingTestConfig(t)
	if err := dao.UpdateAccount([]int64{accountID}, map[string]interface{}{"outbound": "TEST_VLESS"}); err != nil {
		t.Fatal(err)
	}
	applyCalls := 0
	accountHysteria2IsRunning = func() bool { return true }
	accountApplyHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
		applyCalls++
		prepared, err := PrepareHysteria2OutboundConfig(config)
		if err != nil {
			return err
		}
		account, err := GetAccount(accountID)
		if err != nil {
			return err
		}
		if prepared.UserOutbounds[*account.Username] != AccountOutbound(account) {
			t.Fatalf("applied bindings = %#v account = %+v", prepared.UserOutbounds, account)
		}
		return nil
	}

	for _, outbound := range []string{"ISP_SOCKS5_1", "US_HTTP_1", "DIRECT"} {
		if _, err := UpdateAccountWithResult(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &outbound}); err != nil {
			t.Fatalf("switch to %s: %v", outbound, err)
		}
	}
	if applyCalls != 3 {
		t.Fatalf("apply calls = %d, want 3", applyCalls)
	}
	remark, direct := "remark only", "DIRECT"
	if _, err := UpdateAccountWithResult(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Remark: &remark, Outbound: &direct}); err != nil {
		t.Fatal(err)
	}
	if applyCalls != 3 {
		t.Fatalf("unchanged binding restarted Hysteria2; calls = %d", applyCalls)
	}
}

func TestAccountCreateDeleteAndClearRollbackOnApplyFailure(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	setAccountRoutingTestConfig(t)
	if err := dao.UpdateAccount([]int64{accountID}, map[string]interface{}{"outbound": "TEST_VLESS"}); err != nil {
		t.Fatal(err)
	}
	accountHysteria2IsRunning = func() bool { return true }
	applyCalls := 0
	accountApplyHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
		applyCalls++
		if _, err := PrepareHysteria2OutboundConfig(config); err != nil {
			return err
		}
		return errors.New("injected start failure")
	}

	username, outbound := "rollback-new", "ISP_SOCKS5_1"
	if _, err := SaveAccountWithResult(entity.Account{Username: &username, Outbound: &outbound}); err == nil || !strings.Contains(err.Error(), "new account removed") {
		t.Fatalf("create error = %v", err)
	}
	if _, err := dao.GetAccount("username = ?", username); err == nil {
		t.Fatal("failed create left the account in the database")
	}

	if _, err := DeleteAccountWithResult([]int64{accountID}); err == nil || !strings.Contains(err.Error(), "deleted account restored") {
		t.Fatalf("delete error = %v", err)
	}
	account, err := GetAccount(accountID)
	if err != nil || AccountOutbound(account) != "TEST_VLESS" {
		t.Fatalf("deleted account was not restored: account=%+v err=%v", account, err)
	}

	empty := ""
	if _, err := UpdateAccountWithResult(entity.Account{BaseEntity: entity.BaseEntity{Id: &accountID}, Outbound: &empty}); err == nil || !strings.Contains(err.Error(), "previous account binding restored") {
		t.Fatalf("clear binding error = %v", err)
	}
	account, err = GetAccount(accountID)
	if err != nil || AccountOutbound(account) != "TEST_VLESS" {
		t.Fatalf("clear binding did not fail closed: account=%+v err=%v", account, err)
	}
	if applyCalls != 2 {
		t.Fatalf("clearing the final binding reached the runtime apply path; calls = %d", applyCalls)
	}
}

func setAccountRoutingTestConfig(t *testing.T) {
	t.Helper()
	if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{"value": accountRoutingTestConfig}); err != nil {
		t.Fatalf("store account routing test config: %v", err)
	}
}
