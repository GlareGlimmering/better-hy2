package service

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/entity"
)

func TestHysteria2SubscriptionUsesNodeAddress(t *testing.T) {
	accountID, connectionPassword := setupSubscriptionTestDatabase(t)
	t.Setenv(nodeAddressEnvironment, "93.184.216.34")
	requestHost := "panel.example.test:443"
	if err := UpdateSubscriptionTemplate("{PROTOCOL}-{USERNAME}", "{SERVER_IP}"); err != nil {
		t.Fatalf("UpdateSubscriptionTemplate() error = %v", err)
	}

	t.Run("YAML keeps request host separate from node fields", func(t *testing.T) {
		_, config, err := Hysteria2Subscribe(connectionPassword, constant.Clash, requestHost)
		if err != nil {
			t.Fatalf("Hysteria2Subscribe() error = %v", err)
		}

		var parsed struct {
			Proxies []bo.Hysteria2 `yaml:"proxies"`
		}
		if err := yaml.Unmarshal([]byte(config), &parsed); err != nil {
			t.Fatalf("unmarshal subscription YAML: %v", err)
		}
		if len(parsed.Proxies) != 1 {
			t.Fatalf("len(proxies) = %d, want 1", len(parsed.Proxies))
		}
		proxy := parsed.Proxies[0]
		if proxy.Server != "93.184.216.34" {
			t.Errorf("server = %q, want node IPv4", proxy.Server)
		}
		if proxy.Name != "Hysteria2-subscription-test" {
			t.Errorf("name = %q", proxy.Name)
		}
		if proxy.Password != connectionPassword {
			t.Errorf("password = %q, want original %q", proxy.Password, connectionPassword)
		}
		if proxy.Sni != "tls.example.test" {
			t.Errorf("sni = %q, want original TLS SNI", proxy.Sni)
		}
		if strings.Contains(config, requestHost) {
			t.Errorf("subscription YAML unexpectedly contains HTTP request host %q", requestHost)
		}
	})

	t.Run("URI encodes authentication without changing its value", func(t *testing.T) {
		config, err := Hysteria2Url(accountID, requestHost)
		if err != nil {
			t.Fatalf("Hysteria2Url() error = %v", err)
		}
		parsed, err := url.Parse(config)
		if err != nil {
			t.Fatalf("parse Hysteria2 URI: %v", err)
		}
		if parsed.Hostname() != "93.184.216.34" {
			t.Errorf("URI hostname = %q, want node IPv4", parsed.Hostname())
		}
		if parsed.Port() != "443" {
			t.Errorf("URI port = %q, want 443", parsed.Port())
		}
		if got := parsed.User.Username(); got != connectionPassword {
			t.Errorf("decoded URI authentication = %q, want %q", got, connectionPassword)
		}
		if got := parsed.Query().Get("sni"); got != "tls.example.test" {
			t.Errorf("URI sni = %q, want original TLS SNI", got)
		}
		if got := parsed.Fragment; got != "Hysteria2-subscription-test" {
			t.Errorf("URI fragment = %q", got)
		}
		if strings.Contains(config, requestHost) {
			t.Errorf("Hysteria2 URI unexpectedly contains HTTP request host %q", requestHost)
		}
	})

	t.Run("HTTP subscription URL keeps the tunnel host", func(t *testing.T) {
		subscriptionURL, err := Hysteria2SubscribeUrl(accountID, "https:", requestHost)
		if err != nil {
			t.Fatalf("Hysteria2SubscribeUrl() error = %v", err)
		}
		parsed, err := url.Parse(subscriptionURL)
		if err != nil {
			t.Fatalf("parse HTTP subscription URL: %v", err)
		}
		if parsed.Scheme != "https" || parsed.Host != requestHost {
			t.Errorf("HTTP subscription endpoint = %q, want tunnel host %q", subscriptionURL, requestHost)
		}
	})

	t.Run("NekoBox receives a raw Hysteria2 URI", func(t *testing.T) {
		_, config, err := Hysteria2Subscribe(connectionPassword, constant.NekoBox, requestHost)
		if err != nil {
			t.Fatalf("Hysteria2Subscribe() error = %v", err)
		}
		parsed, err := url.Parse(config)
		if err != nil {
			t.Fatalf("parse NekoBox URI: %v", err)
		}
		if parsed.Scheme != "hysteria2" || parsed.Hostname() != "93.184.216.34" {
			t.Fatalf("NekoBox URI = %q", config)
		}
	})

	t.Run("IPv6 URI uses brackets", func(t *testing.T) {
		restore := stubNetworkInterfaces(
			[]net.Interface{{Index: 1, Name: "eth0", Flags: net.FlagUp}},
			map[int][]net.Addr{1: {ipNetwork("2001:4860:4860::8888/64")}},
		)
		defer restore()
		resetNodeIPv6Cache()
		if err := UpdateSubscriptionTemplate("IPv6-{USERNAME}", "{SERVER_IPV6}"); err != nil {
			t.Fatalf("UpdateSubscriptionTemplate() error = %v", err)
		}
		config, err := Hysteria2Url(accountID, requestHost)
		if err != nil {
			t.Fatalf("Hysteria2Url() error = %v", err)
		}
		parsed, err := url.Parse(config)
		if err != nil {
			t.Fatalf("parse Hysteria2 URI: %v", err)
		}
		if parsed.Hostname() != "2001:4860:4860::8888" || !strings.Contains(config, "@[2001:4860:4860::8888]:443") {
			t.Errorf("IPv6 URI = %q", config)
		}
	})
}

func setupSubscriptionTestDatabase(t *testing.T) (int64, string) {
	t.Helper()
	originalRunning := accountHysteria2IsRunning
	originalApply := accountApplyHysteria2Config
	accountHysteria2IsRunning = func() bool { return false }
	accountApplyHysteria2Config = func(bo.Hysteria2ServerConfig) error { return nil }
	t.Cleanup(func() {
		accountHysteria2IsRunning = originalRunning
		accountApplyHysteria2Config = originalApply
	})
	dataDirectory := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDirectory, constant.SqliteDBDir), 0o755); err != nil {
		t.Fatalf("create test data directory: %v", err)
	}
	t.Setenv("HUI_DATA", dataDirectory+string(os.PathSeparator))
	if err := dao.InitSql(""); err != nil {
		t.Fatalf("initialize test database: %v", err)
	}
	t.Cleanup(func() {
		if err := dao.CloseSqliteDB(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	serverConfig := "listen: :443\nacme:\n  domains:\n    - tls.example.test\noutbounds:\n  - name: DIRECT\n    type: direct\n    direct:\n      mode: auto\n  - name: TEST_VLESS\n    type: socks5\n    socks5:\n      addr: 127.0.0.1:10808\ntrafficStats:\n  listen: 127.0.0.1:9999\n"
	if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{"value": serverConfig}); err != nil {
		t.Fatalf("store Hysteria2 config: %v", err)
	}

	connectionPassword := "pa@ss:#/?&%+ ü"
	username := "subscription-test"
	password := "unused"
	quota := int64(1000)
	download := int64(20)
	upload := int64(10)
	expireTime := int64(4102444800000)
	kickUntil := int64(0)
	deviceNumber := int64(3)
	role := "user"
	deleted := int64(0)
	accountID, err := dao.SaveAccount(entity.Account{
		Username:     &username,
		Pass:         &password,
		ConPass:      &connectionPassword,
		Quota:        &quota,
		Download:     &download,
		Upload:       &upload,
		ExpireTime:   &expireTime,
		KickUtilTime: &kickUntil,
		DeviceNo:     &deviceNumber,
		Role:         &role,
		Deleted:      &deleted,
	})
	if err != nil {
		t.Fatalf("create test account: %v", err)
	}
	return accountID, connectionPassword
}
