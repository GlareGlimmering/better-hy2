package service

import (
	"errors"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/entity"
)

func TestBuildUserOutboundBindings(t *testing.T) {
	outbounds := testOutbounds()
	accounts := []entity.Account{
		testRoutingAccount("Alice", "DIRECT", 0, "user"),
		testRoutingAccount("bob", "test_vless", 0, "user"),
		testRoutingAccount("disabled", "MISSING", 1, "user"),
		testRoutingAccount("sysadmin", "", 0, "admin"),
	}

	bindings, err := BuildUserOutboundBindings(accounts, outbounds)
	if err != nil {
		t.Fatalf("BuildUserOutboundBindings() error = %v", err)
	}
	if len(bindings) != 2 || bindings["Alice"] != "DIRECT" || bindings["bob"] != "TEST_VLESS" {
		t.Fatalf("bindings = %#v", bindings)
	}
	if _, exists := bindings["alice"]; exists {
		t.Fatal("username was unexpectedly lower-cased")
	}
}

func TestBuildUserOutboundBindingsKeepsLegacyMode(t *testing.T) {
	accounts := []entity.Account{
		testRoutingAccount("alice", "", 0, "user"),
		testRoutingAccount("bob", "", 0, "user"),
	}
	bindings, err := BuildUserOutboundBindings(accounts, testOutbounds())
	if err != nil {
		t.Fatalf("BuildUserOutboundBindings() error = %v", err)
	}
	if bindings != nil {
		t.Fatalf("legacy bindings = %#v, want nil", bindings)
	}
}

func TestMergeManagedOutboundNodesCreatesAllProtocolMappingsAndIsIdempotent(t *testing.T) {
	settings := bo.XraySettings{Version: "v25.10.15", Outbounds: []bo.XrayManagedOutbound{
		{ID: "vless-1", Name: "TEST_VLESS", Protocol: "vless", Enabled: true, Address: "vless.example.com", Port: 443, UUID: "123e4567-e89b-12d3-a456-426614174000", Network: "tcp", TLSSNI: "vless.example.com", SOCKSListen: "127.0.0.1", SOCKSPort: 10808, OutboundTag: "hui-out-vless-1"},
		{ID: "http-1", Name: "US_HTTP_1", Protocol: "http", Enabled: true, Address: "http.example.com", Port: 8080, SOCKSListen: "127.0.0.1", SOCKSPort: 10809, OutboundTag: "hui-out-http-1"},
		{ID: "socks-1", Name: "JP_SOCKS_1", Protocol: "socks5", Enabled: true, Address: "socks.example.com", Port: 1080, Username: "alice", Password: "secret"},
	}}
	legacyName, legacyType, legacyAddr := "LEGACY", "socks5", "127.0.0.1:10999"
	config := bo.Hysteria2ServerConfig{Outbounds: []bo.ServerConfigOutboundEntry{{Name: &legacyName, Type: &legacyType, SOCKS5: &bo.ServerConfigOutboundSOCKS5{Addr: &legacyAddr}}}}
	merged, err := MergeManagedOutboundNodes(config, settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Outbounds) != 5 {
		t.Fatalf("outbounds = %+v", merged.Outbounds)
	}
	known, _ := outboundNames(merged.Outbounds)
	for _, name := range []string{"DIRECT", "TEST_VLESS", "US_HTTP_1", "JP_SOCKS_1", "LEGACY"} {
		if known[strings.ToLower(name)] != name {
			t.Fatalf("missing %s: %+v", name, known)
		}
	}
	second, err := MergeManagedOutboundNodes(merged, settings)
	if err != nil || len(second.Outbounds) != len(merged.Outbounds) {
		t.Fatalf("merge is not idempotent: len=%d err=%v", len(second.Outbounds), err)
	}
	for _, outbound := range second.Outbounds {
		if outbound.Name != nil && *outbound.Name == "JP_SOCKS_1" {
			if outbound.SOCKS5 == nil || outbound.SOCKS5.Addr == nil || *outbound.SOCKS5.Addr != "socks.example.com:1080" || outbound.SOCKS5.Username == nil || *outbound.SOCKS5.Username != "alice" {
				t.Fatalf("native SOCKS mapping changed: %+v", outbound)
			}
		}
	}
}

func TestBuildUserOutboundBindingsRejectsPartialAndUnknownBindings(t *testing.T) {
	tests := []struct {
		name     string
		accounts []entity.Account
		want     string
	}{
		{
			name: "partial",
			accounts: []entity.Account{
				testRoutingAccount("alice", "DIRECT", 0, "user"),
				testRoutingAccount("bob", "", 0, "user"),
			},
			want: `account "bob" has no outbound binding`,
		},
		{
			name:     "unknown",
			accounts: []entity.Account{testRoutingAccount("alice", "MISSING", 0, "user")},
			want:     `account "alice" references unknown outbound "MISSING"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildUserOutboundBindings(tt.accounts, testOutbounds())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestPrepareHysteria2OutboundConfigGeneratesYAML(t *testing.T) {
	config := bo.Hysteria2ServerConfig{Outbounds: testOutbounds()}
	accounts := []entity.Account{
		testRoutingAccount("test_direct", "DIRECT", 0, "user"),
		testRoutingAccount("test_vless_user", "TEST_VLESS", 0, "user"),
	}
	prepared, err := PrepareHysteria2OutboundConfigWithAccounts(config, accounts)
	if err != nil {
		t.Fatalf("PrepareHysteria2OutboundConfigWithAccounts() error = %v", err)
	}
	encoded, err := yaml.Marshal(prepared)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	text := string(encoded)
	for _, expected := range []string{
		"userOutbounds:",
		"test_direct: DIRECT",
		"test_vless_user: TEST_VLESS",
		"addr: 127.0.0.1:10808",
		"password: proxy-secret",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("generated YAML missing %q:\n%s", expected, text)
		}
	}
}

func TestValidateHysteria2OutboundConfigFailClosedConstraints(t *testing.T) {
	bindings := map[string]string{"alice": "DIRECT"}
	t.Run("duplicate names ignore case", func(t *testing.T) {
		config := bo.Hysteria2ServerConfig{Outbounds: append(testOutbounds(), directOutbound("direct"))}
		if err := ValidateHysteria2OutboundConfig(config); err == nil || !strings.Contains(err.Error(), "duplicate outbound") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("unknown outbound", func(t *testing.T) {
		config := bo.Hysteria2ServerConfig{Outbounds: testOutbounds(), UserOutbounds: map[string]string{"alice": "MISSING"}}
		if err := ValidateHysteria2OutboundConfig(config); err == nil || !strings.Contains(err.Error(), "unknown outbound") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("ACL conflict", func(t *testing.T) {
		config := bo.Hysteria2ServerConfig{Outbounds: testOutbounds(), UserOutbounds: bindings}
		var decoded bo.Hysteria2ServerConfig
		if err := yaml.Unmarshal([]byte("acl:\n  inline: ['direct(all)']\n"), &decoded); err != nil {
			t.Fatal(err)
		}
		config.ACL = decoded.ACL
		if err := ValidateHysteria2OutboundConfig(config); err == nil || !strings.Contains(err.Error(), "ACL") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("custom resolver conflict", func(t *testing.T) {
		config := bo.Hysteria2ServerConfig{Outbounds: testOutbounds(), UserOutbounds: bindings}
		var decoded bo.Hysteria2ServerConfig
		if err := yaml.Unmarshal([]byte("resolver:\n  type: udp\n"), &decoded); err != nil {
			t.Fatal(err)
		}
		config.Resolver = decoded.Resolver
		if err := ValidateHysteria2OutboundConfig(config); err == nil || !strings.Contains(err.Error(), "system resolver") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("invalid SOCKS5 address", func(t *testing.T) {
		outbounds := testOutbounds()
		bad := "127.0.0.1"
		outbounds[1].SOCKS5.Addr = &bad
		if err := ValidateHysteria2OutboundConfig(bo.Hysteria2ServerConfig{Outbounds: outbounds}); err == nil {
			t.Fatal("invalid SOCKS5 address was accepted")
		}
	})
}

func TestEnsureHysteria2UserRoutingCompatible(t *testing.T) {
	original := installedHysteria2Version
	t.Cleanup(func() { installedHysteria2Version = original })

	legacy := bo.Hysteria2ServerConfig{Outbounds: testOutbounds()}
	installedHysteria2Version = func() (string, error) {
		return "", errors.New("binary missing")
	}
	if err := EnsureHysteria2UserRoutingCompatible(legacy); err != nil {
		t.Fatalf("legacy mode must remain compatible: %v", err)
	}

	strict := legacy
	strict.UserOutbounds = map[string]string{"alice": "DIRECT"}
	installedHysteria2Version = func() (string, error) {
		return constant.Hysteria2CustomVersion, nil
	}
	if err := EnsureHysteria2UserRoutingCompatible(strict); err != nil {
		t.Fatalf("custom kernel was rejected: %v", err)
	}

	installedHysteria2Version = func() (string, error) { return "v2.13.0", nil }
	if err := EnsureHysteria2UserRoutingCompatible(strict); err == nil || !strings.Contains(err.Error(), constant.Hysteria2CustomVersion) {
		t.Fatalf("official kernel error = %v", err)
	}
}

func TestStrictUserRoutingPreflightChecksConflictsWithoutBindings(t *testing.T) {
	var aclConfig bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal([]byte("acl:\n  inline: ['direct(all)']\n"), &aclConfig); err != nil {
		t.Fatal(err)
	}
	if err := validateStrictUserRoutingCompatibility(aclConfig); err == nil || !strings.Contains(err.Error(), "ACL") {
		t.Fatalf("ACL preflight error = %v", err)
	}

	var resolverConfig bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal([]byte("resolver:\n  type: udp\n"), &resolverConfig); err != nil {
		t.Fatal(err)
	}
	if err := validateStrictUserRoutingCompatibility(resolverConfig); err == nil || !strings.Contains(err.Error(), "system resolver") {
		t.Fatalf("resolver preflight error = %v", err)
	}
}

func TestHysteria2RoutingStatusReportsUnboundUsers(t *testing.T) {
	_, _ = setupSubscriptionTestDatabase(t)
	original := installedHysteria2Version
	installedHysteria2Version = func() (string, error) {
		return constant.Hysteria2CustomVersion, nil
	}
	t.Cleanup(func() { installedHysteria2Version = original })

	status, err := Hysteria2RoutingStatus()
	if err != nil {
		t.Fatalf("Hysteria2RoutingStatus() error = %v", err)
	}
	if status.ActiveUsers != 1 || status.BoundUsers != 0 {
		t.Fatalf("user counts = active %d bound %d", status.ActiveUsers, status.BoundUsers)
	}
	if len(status.UnboundUsers) != 1 || status.UnboundUsers[0] != "subscription-test" {
		t.Fatalf("unbound users = %#v", status.UnboundUsers)
	}
	if status.Ready {
		t.Fatal("routing unexpectedly reported ready")
	}
}

func TestOutboundInUse(t *testing.T) {
	accounts := []entity.Account{testRoutingAccount("alice", "TEST_VLESS", 0, "user")}
	if !OutboundInUse("test_vless", accounts) {
		t.Fatal("referenced outbound reported unused")
	}
	if OutboundInUse("DIRECT", accounts) {
		t.Fatal("unreferenced outbound reported in use")
	}
}

func TestOutboundCRUDReferenceProtection(t *testing.T) {
	accounts := []entity.Account{testRoutingAccount("alice", "TEST_VLESS", 0, "user")}

	t.Run("add and edit unreferenced outbound", func(t *testing.T) {
		config := bo.Hysteria2ServerConfig{Outbounds: append(testOutbounds(), directOutbound("BACKUP"))}
		if _, err := PrepareHysteria2OutboundConfigWithAccounts(config, accounts); err != nil {
			t.Fatalf("add outbound: %v", err)
		}
		renamed := "BACKUP_RENAMED"
		config.Outbounds[len(config.Outbounds)-1].Name = &renamed
		if _, err := PrepareHysteria2OutboundConfigWithAccounts(config, accounts); err != nil {
			t.Fatalf("edit unreferenced outbound: %v", err)
		}
	})

	t.Run("delete referenced outbound", func(t *testing.T) {
		config := bo.Hysteria2ServerConfig{Outbounds: []bo.ServerConfigOutboundEntry{directOutbound("DIRECT")}}
		_, err := PrepareHysteria2OutboundConfigWithAccounts(config, accounts)
		if err == nil || !strings.Contains(err.Error(), `references unknown outbound "TEST_VLESS"`) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("rename referenced outbound", func(t *testing.T) {
		outbounds := testOutbounds()
		renamed := "TM_NET_2"
		outbounds[1].Name = &renamed
		_, err := PrepareHysteria2OutboundConfigWithAccounts(bo.Hysteria2ServerConfig{Outbounds: outbounds}, accounts)
		if err == nil || !strings.Contains(err.Error(), `references unknown outbound "TEST_VLESS"`) {
			t.Fatalf("error = %v", err)
		}
	})
}

func testOutbounds() []bo.ServerConfigOutboundEntry {
	mode := "auto"
	address := "127.0.0.1:10808"
	username := "proxy-user"
	password := "proxy-secret"
	return []bo.ServerConfigOutboundEntry{
		directOutbound("DIRECT"),
		{
			Name: stringPointer("TEST_VLESS"),
			Type: stringPointer("socks5"),
			SOCKS5: &bo.ServerConfigOutboundSOCKS5{
				Addr: &address, Username: &username, Password: &password,
			},
		},
		{
			Name:   stringPointer("DIRECT_V6"),
			Type:   stringPointer("direct"),
			Direct: &bo.ServerConfigOutboundDirect{Mode: &mode},
		},
	}
}

func directOutbound(name string) bo.ServerConfigOutboundEntry {
	mode := "auto"
	return bo.ServerConfigOutboundEntry{
		Name: &name, Type: stringPointer("direct"),
		Direct: &bo.ServerConfigOutboundDirect{Mode: &mode},
	}
}

func testRoutingAccount(username, outbound string, deleted int64, role string) entity.Account {
	return entity.Account{Username: &username, Outbound: &outbound, Deleted: &deleted, Role: &role}
}

func stringPointer(value string) *string {
	return &value
}
