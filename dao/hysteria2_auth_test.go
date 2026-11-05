package dao

import (
	"h-ui/model/entity"
	"testing"
)

func TestFindHysteria2AuthAccountSeparatesCredentialAndPolicy(t *testing.T) {
	t.Setenv("HUI_DATA", t.TempDir())
	if err := InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	username, password, disabled := "auth-diagnostic-user", "test-credential", int64(1)
	if _, err := SaveAccount(entity.Account{Username: &username, ConPass: &password, Deleted: &disabled}); err != nil {
		t.Fatal(err)
	}
	account, found, err := FindHysteria2AuthAccount(password)
	if err != nil || !found || *account.Username != username || *account.Deleted != 1 {
		t.Fatalf("lookup lost disabled account: %v %v", found, err)
	}
	if _, found, err = FindHysteria2AuthAccount("wrong"); err != nil || found {
		t.Fatalf("missing credential: %v %v", found, err)
	}
}
