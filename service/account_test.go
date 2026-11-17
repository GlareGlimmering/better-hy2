package service

import (
	"database/sql"
	"errors"
	"h-ui/util"
	"path/filepath"
	"strings"
	"testing"

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
	account, err = dao.GetAccount("id = ?", accountID)
	if err != nil || AccountOutbound(account) != outbound {
		t.Fatalf("outbound not persisted: %v", err)
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
	result, err := SaveAccountWithResult(entity.Account{Username: &username, Outbound: &outbound}, nil)
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
	if _, err := SaveAccountWithResult(entity.Account{Username: &username, Outbound: &outbound}, nil); err == nil || !strings.Contains(err.Error(), "new account removed") {
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

func TestPanelAdminMigrationAndPasswordGate(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HUI_DATA", root)
	if err := dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	if err := dao.CloseSqliteDB(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, constant.SqliteDBPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ username, password, connection, role string }{
		{"former-admin", "shared-password", "old-prefix.shared-password", "admin"},
		{"other-admin", "private-password", "different.connection", "admin"},
		{"sysadmin", "sysadmin", "sysadmin.sysadmin", "admin"},
		{"proxy-admin", "private-password", "proxy-admin.connection", "user"},
	} {
		if _, err := db.Exec("INSERT INTO account(username,pass,con_pass,role) VALUES(?,?,?,?)", row.username, util.SHA224String(row.password), row.connection, row.role); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := dao.InitSqliteSchema(); err != nil {
			t.Fatal(err)
		}
		if err := dao.CloseSqliteDB(); err != nil {
			t.Fatal(err)
		}
	}
	if err := dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	defer dao.CloseSqliteDB()
	for _, attempt := range [][2]string{{"sysadmin", "sysadmin"}, {"proxy-admin", "private-password"}} {
		if token, err := Login(attempt[0], attempt[1]); err == nil || token != "" {
			t.Fatal("unexpected panel access")
		}
	}
	if token, err := Login("former-admin", "shared-password"); !errors.Is(err, ErrPanelPasswordChangeRequired) || token != "" {
		t.Fatal("coupled password obtained a session")
	}
	if token, err := ChangePanelPassword("former-admin", "shared-password", "replacement-panel-password"); err != nil || token == "" {
		t.Fatal("coupled password could not be replaced", err)
	}
	if _, found, err := dao.FindHysteria2AuthAccount("old-prefix.shared-password"); err != nil || found {
		t.Fatal("old administrator can authenticate as a proxy user", err)
	}
	if _, found, err := dao.FindHysteria2AuthAccount("proxy-admin.connection"); err != nil || !found {
		t.Fatal("proxy user credential did not survive migration", err)
	}
	if token, err := Login("other-admin", "private-password"); err != nil || token == "" {
		t.Fatal("uncoupled legacy administrator cannot sign in", err)
	}
	admin, err := dao.PanelAdminByUsername("other-admin")
	if err != nil || admin.Legacy || !util.VerifyPanelPassword(admin.PasswordHash, "private-password") {
		t.Fatal("legacy hash was not upgraded to native Argon2id", err)
	}
}

func TestBindUnassignedAccountOutbounds(t *testing.T) {
	accountID, _ := setupSubscriptionTestDatabase(t)
	setAccountRoutingTestConfig(t)
	second := "other-user"
	secondID, err := dao.SaveAccount(entity.Account{Username: &second})
	if err != nil {
		t.Fatal(err)
	}
	existing, existingRoute, disabled := "existing-user", "US_HTTP_1", "disabled-user"
	disabledFlag := int64(1)
	existingID, err := dao.SaveAccount(entity.Account{Username: &existing, Outbound: &existingRoute})
	if err != nil {
		t.Fatal(err)
	}
	disabledID, err := dao.SaveAccount(entity.Account{Username: &disabled, Deleted: &disabledFlag})
	if err != nil {
		t.Fatal(err)
	}
	checkBinding := func(id int64, want string) {
		t.Helper()
		account, err := GetAccount(id)
		if err != nil {
			t.Fatal(err)
		}
		if AccountOutbound(account) != want {
			t.Fatalf("account %d outbound = %q, want %q", id, AccountOutbound(account), want)
		}
	}
	applyCalls, failApply := 0, true
	accountHysteria2IsRunning = func() bool { return true }
	accountApplyHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
		applyCalls++
		prepared, err := PrepareHysteria2OutboundConfig(config)
		if err != nil {
			return err
		}
		bindings := prepared.UserOutbounds
		if len(bindings) != 3 || bindings["subscription-test"] != "ISP_SOCKS5_1" ||
			bindings[second] != "DIRECT" || bindings[existing] != existingRoute {
			t.Fatalf("unexpected applied bindings: %v", bindings)
		}
		if failApply {
			return errors.New("injected apply failure")
		}
		return nil
	}
	for _, request := range []map[string]string{
		{"subscription-test": "ISP_SOCKS5_1"},
		{"subscription-test": "unknown", second: "DIRECT"},
		{"subscription-test": "ISP_SOCKS5_1", second: "DIRECT", existing: "DIRECT"},
		{"subscription-test": "ISP_SOCKS5_1", second: "DIRECT", disabled: "DIRECT"},
	} {
		if _, err := BindUnassignedAccountOutbounds(request); err == nil {
			t.Fatalf("accepted invalid request: %v", request)
		}
	}
	if applyCalls != 0 {
		t.Fatalf("invalid request reached apply: %d calls", applyCalls)
	}
	request := map[string]string{"subscription-test": "isp_socks5_1", second: "DIRECT"}
	if _, err := BindUnassignedAccountOutbounds(request); err == nil || !strings.Contains(err.Error(), "previous account bindings restored") {
		t.Fatalf("unexpected apply failure: %v", err)
	}
	checkBinding(accountID, "")
	checkBinding(secondID, "")
	checkBinding(existingID, existingRoute)

	failApply = false
	result, err := BindUnassignedAccountOutbounds(request)
	if err != nil || !result.RuntimeApplied || applyCalls != 2 {
		t.Fatalf("apply result=%+v calls=%d error=%v", result, applyCalls, err)
	}
	checkBinding(accountID, "ISP_SOCKS5_1")
	checkBinding(secondID, "DIRECT")
	checkBinding(existingID, existingRoute)
	checkBinding(disabledID, "")
	if _, err := BindUnassignedAccountOutbounds(request); err == nil || applyCalls != 2 {
		t.Fatalf("stale request result: calls=%d error=%v", applyCalls, err)
	}
}

func TestCreateSocksAccountWithExistingUnboundAccounts(t *testing.T) {
	oldID, oldPassword := setupSubscriptionTestDatabase(t)
	setAccountRoutingTestConfig(t)
	username, outbound, password := "new-socks", "ISP_SOCKS5_1", "new-socks.Synthetic123!"
	account := entity.Account{Username: &username, Outbound: &outbound, ConPass: &password}
	existing := map[string]string{"subscription-test": "DIRECT"}
	calls, fail := 0, true
	accountHysteria2IsRunning = func() bool { return true }
	accountApplyHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
		calls++
		prepared, err := PrepareHysteria2OutboundConfig(config)
		if err != nil {
			return err
		}
		bindings := prepared.UserOutbounds
		if len(bindings) != 2 || bindings["subscription-test"] != "DIRECT" || bindings[username] != outbound {
			t.Fatalf("applied bindings: %v", bindings)
		}
		if fail {
			return errors.New("injected HY2 startup failure")
		}
		return nil
	}
	checkOld := func(expectedOutbound string) {
		t.Helper()
		old, err := GetAccount(oldID)
		if err != nil {
			t.Fatal(err)
		}
		if AccountOutbound(old) != expectedOutbound || *old.ConPass != oldPassword || *old.Quota != 1000 {
			t.Fatal("unexpected changes to existing account")
		}
	}
	if _, err := SaveAccountWithResult(account, nil); err == nil || !strings.Contains(err.Error(), "has no outbound binding") || calls != 0 {
		t.Fatalf("missing binding should be rejected before apply: calls=%d error=%v", calls, err)
	}
	if _, err := SaveAccountWithResult(account, map[string]string{"subscription-test": "MISSING"}); err == nil || calls != 0 {
		t.Fatalf("unknown binding should be rejected before apply: calls=%d error=%v", calls, err)
	}
	checkOld("")
	if _, err := SaveAccountWithResult(account, existing); err == nil || !strings.Contains(err.Error(), "previous account bindings restored") {
		t.Fatalf("apply failure error = %v", err)
	}
	if _, err := dao.GetAccount("username = ?", username); err == nil {
		t.Fatal("failed create left new account in database")
	}
	checkOld("")
	fail = false
	result, err := SaveAccountWithResult(account, existing)
	if err != nil || !result.RuntimeApplied || calls != 2 {
		t.Fatalf("result=%+v calls=%d error=%v", result, calls, err)
	}
	created, err := dao.GetAccount("username = ?", username)
	if err != nil {
		t.Fatal(err)
	}
	if AccountOutbound(created) != outbound || *created.ConPass != password {
		t.Fatal("new account did not retain its own outbound and credential")
	}
	checkOld("DIRECT")
}
