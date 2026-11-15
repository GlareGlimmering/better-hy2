package util

import (
	"strings"
	"testing"
)

func TestVerifyPanelPasswordAlwaysDerivesOneKey(t *testing.T) {
	const password = "fixture-password"
	hash, err := HashPanelPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		encoded string
		valid   bool
	}{
		{"valid", hash, true},
		{"empty", "", false},
		{"malformed", panelPasswordPrefix + strings.Repeat("!", 22) + "$" + strings.Repeat("A", 43), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			matched := verifyPanelPassword(tc.encoded, password, func(input string, salt []byte) []byte {
				calls++
				if input != password || len(salt) != 16 {
					t.Fatal("unexpected key derivation parameters")
				}
				if tc.valid {
					return panelPasswordKey(input, salt)
				}
				return make([]byte, 32)
			})
			if calls != 1 || matched != tc.valid {
				t.Fatalf("wanted one derivation and match %v, got %d calls and match %v", tc.valid, calls, matched)
			}
		})
	}
}
