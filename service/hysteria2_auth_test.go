package service

import (
	"errors"
	"h-ui/model/entity"
	"testing"
)

func TestHysteria2AuthPolicies(t *testing.T) {
	for _, reason := range []string{"success", "runtime_unavailable", "connection_password_mismatch", "account_lookup_failed", "account_disabled", "quota_exhausted", "account_expired", "kick_restricted", "device_limit_invalid"} {
		t.Run(reason, func(t *testing.T) {
			id, username, outbound := int64(3), "TESTsss", "DIRECT"
			quota, download, upload, expire, kick, devices, disabled := int64(100), int64(10), int64(20), int64(2000), int64(0), int64(2), int64(0)
			switch reason {
			case "account_disabled":
				disabled = 1
			case "quota_exhausted":
				quota = 30
			case "account_expired":
				expire = 1000
			case "kick_restricted":
				kick = 1000
			case "device_limit_invalid":
				devices = 0
			}
			account := entity.Account{BaseEntity: entity.BaseEntity{Id: &id}, Username: &username, Outbound: &outbound, Quota: &quota, Download: &download, Upload: &upload, ExpireTime: &expire, KickUtilTime: &kick, DeviceNo: &devices, Deleted: &disabled}
			gotID, gotName, limit, err := authenticateHysteria2("secret", func() bool { return reason != "runtime_unavailable" }, func(password string) (entity.Account, bool, error) {
				if password != "secret" {
					t.Fatal("credential changed")
				}
				if reason == "account_lookup_failed" {
					return account, false, errors.New("private upstream detail")
				}
				return account, reason != "connection_password_mismatch", nil
			}, 1000)
			if reason == "success" {
				if err != nil || gotID != id || gotName != username || limit != devices {
					t.Fatalf("DIRECT auth: %d %q %v", gotID, gotName, err)
				}
				return
			}
			failure, ok := err.(*Hysteria2AuthError)
			if !ok || failure.Reason != reason || gotName != "" || gotID != 0 {
				t.Fatalf("%d %q %v", gotID, gotName, err)
			}
		})
	}
}
