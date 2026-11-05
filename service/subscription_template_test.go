package service

import (
	"net"
	"strings"
	"testing"
	"time"

	"h-ui/model/entity"
)

func TestRenderSubscriptionNodeVariables(t *testing.T) {
	t.Setenv(nodeAddressEnvironment, "93.184.216.34")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	username := "user-special"
	quota, download, upload := int64(10*1024), int64(1024), int64(2048)
	expire := now.Add(49 * time.Hour).UnixMilli()
	deleted, kick := int64(0), int64(0)
	account := entity.Account{Username: &username, Quota: &quota, Download: &download, Upload: &upload, ExpireTime: &expire, Deleted: &deleted, KickUtilTime: &kick}

	node, err := renderSubscriptionNode("{PROTOCOL} {USERNAME} {DATA_USAGE} {DATA_LEFT} {DAYS_LEFT} {STATUS_EMOJI} {TRANSPORT}", "{SERVER_IP}", account, now)
	if err != nil {
		t.Fatalf("renderSubscriptionNode() error = %v", err)
	}
	if node.Address != "93.184.216.34" {
		t.Errorf("address = %q", node.Address)
	}
	for _, part := range []string{"Hysteria2", username, "3.00 KiB", "7.00 KiB", "2", "✅", "QUIC"} {
		if !strings.Contains(node.Name, part) {
			t.Errorf("name %q does not contain %q", node.Name, part)
		}
	}
}

func TestRenderSubscriptionNodeIPv6(t *testing.T) {
	restore := stubNetworkInterfaces(
		[]net.Interface{{Index: 1, Name: "eth0", Flags: net.FlagUp}},
		map[int][]net.Addr{1: {ipNetwork("2001:4860:4860::8888/64")}},
	)
	defer restore()
	resetNodeAddressCache()
	resetNodeIPv6Cache()
	account := templateTestAccount(time.Now().Add(time.Hour))
	node, err := renderSubscriptionNode("IPv6", "{SERVER_IPV6}", account, time.Now())
	if err != nil {
		t.Fatalf("renderSubscriptionNode() error = %v", err)
	}
	if node.Address != "2001:4860:4860::8888" {
		t.Fatalf("address = %q", node.Address)
	}
}

func TestTemplateValidation(t *testing.T) {
	for _, test := range []struct {
		value   string
		address bool
	}{
		{"{UNKNOWN}", false},
		{"{USERNAME}", true},
		{"https://example.com", true},
		{"{SERVER_IP", false},
	} {
		if err := validateTemplate(test.value, test.address); err == nil {
			t.Errorf("validateTemplate(%q, %v) unexpectedly succeeded", test.value, test.address)
		}
	}
}

func TestJalaliDate(t *testing.T) {
	date := time.Date(2024, time.March, 20, 0, 0, 0, 0, time.UTC)
	if got := formatJalaliDate(date); got != "1403-01-01" {
		t.Fatalf("formatJalaliDate() = %q", got)
	}
}

func TestPermanentAndUnlimitedTemplateValues(t *testing.T) {
	t.Setenv(nodeAddressEnvironment, "93.184.216.34")
	account := templateTestAccount(time.UnixMilli(permanentExpireTime))
	values, err := subscriptionTemplateValuesForAccount(account, time.Now())
	if err != nil {
		t.Fatalf("subscriptionTemplateValuesForAccount() error = %v", err)
	}
	for _, variable := range []string{"{DAYS_LEFT}", "{EXPIRE_DATE}", "{JALALI_EXPIRE_DATE}", "{TIME_LEFT}"} {
		if values[variable] != "Permanent" {
			t.Errorf("%s = %q", variable, values[variable])
		}
	}
	if values["{DATA_LIMIT}"] != "Unlimited" || values["{DATA_LEFT}"] != "Unlimited" {
		t.Errorf("unlimited values = %q, %q", values["{DATA_LIMIT}"], values["{DATA_LEFT}"])
	}
}

func templateTestAccount(expire time.Time) entity.Account {
	username := "user"
	zero, unlimited := int64(0), int64(-1)
	expireMillis := expire.UnixMilli()
	return entity.Account{Username: &username, Quota: &unlimited, Download: &zero, Upload: &zero, ExpireTime: &expireMillis, Deleted: &zero, KickUtilTime: &zero}
}
