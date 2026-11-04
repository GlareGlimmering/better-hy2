package service

import (
	"errors"
	"h-ui/model/entity"
	"testing"
)

func TestHysteria2AuthPolicies(t *testing.T) {
	for _, reason := range []string{"success", "runtime_unavailable", "connection_password_mismatch", "account_lookup_failed", "account_disabled", "quota_exhausted", "account_expired", "kick_restricted", "traffic_stats_unavailable", "device_limit_exceeded"} {
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
			}
			account := entity.Account{BaseEntity: entity.BaseEntity{Id: &id}, Username: &username, Outbound: &outbound, Quota: &quota, Download: &download, Upload: &upload, ExpireTime: &expire, KickUtilTime: &kick, DeviceNo: &devices, Deleted: &disabled}
			onlineCalled := false
			gotID, gotName, err := authenticateHysteria2("secret", func() bool { return reason != "runtime_unavailable" }, func(password string) (entity.Account, bool, error) {
				if password != "secret" {
					t.Fatal("credential changed")
				}
				if reason == "account_lookup_failed" {
					return account, false, errors.New("private upstream detail")
				}
				return account, reason != "connection_password_mismatch", nil
			}, func() (map[string]int64, error) {
				onlineCalled = true
				if reason == "traffic_stats_unavailable" {
					return nil, errors.New("private upstream detail")
				}
				count := int64(1)
				if reason == "device_limit_exceeded" {
					count = 2
				}
				return map[string]int64{username: count}, nil
			}, 1000)
			if reason == "success" {
				if err != nil || gotID != id || gotName != username {
					t.Fatalf("DIRECT auth: %d %q %v", gotID, gotName, err)
				}
				return
			}
			failure, ok := err.(*Hysteria2AuthError)
			if !ok || failure.Reason != reason || gotName != "" || gotID != 0 {
				t.Fatalf("%d %q %v", gotID, gotName, err)
			}
			if reason != "traffic_stats_unavailable" && reason != "device_limit_exceeded" && onlineCalled {
				t.Fatal("policy rejection queried online API")
			}
		})
	}
}
