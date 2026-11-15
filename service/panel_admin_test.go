package service

import (
	"testing"

	"h-ui/dao"
	"h-ui/model/constant"
	"h-ui/util"
)

func TestFailedPanelLoginVerifiesPasswordOnce(t *testing.T) {
	setupSubscriptionTestDatabase(t)
	const username, password = "test-admin", "incorrect-password"
	hash, err := util.HashPanelPassword("fixture-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dao.ResetPanelAdmin(username, hash); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		hash    string
		matches bool
	}{
		{"missing-admin", "", true},
		{username, hash, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			token, err := authenticatePanelAdmin(tc.name, password, "", func(encoded, input string) bool {
				calls++
				if encoded != tc.hash || input != password {
					t.Fatal("unexpected verification input")
				}
				return tc.matches
			})
			if calls != 1 || token != "" || err == nil || err.Error() != constant.WrongPassword {
				t.Fatalf("wanted one verification and normal rejection, got %d calls and %v", calls, err)
			}
		})
	}
}
