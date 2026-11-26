package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
	"h-ui/model/entity"
)

func TestUDPRelayRuntimeRequiresBinding(t *testing.T) {
	stored := []byte("outbounds:\n  - name: DIRECT\n    type: direct\n    direct: {}\n  - name: WG\n    type: udpRelay\n    udpRelay: {addr: '192.0.2.10:51820'}\n")
	idle, err := runtimeHysteria2YAML(stored)
	if err != nil || strings.Contains(string(idle), "udpRelay") {
		t.Fatalf("unbound relay entered runtime configuration: %s %v", idle, err)
	}
	bound := append(append([]byte{}, stored...), []byte("userOutbounds: {ordinary: DIRECT, relay: WG}\n")...)
	data, err := runtimeHysteria2YAML(bound)
	var config bo.Hysteria2ServerConfig
	if err != nil || yaml.Unmarshal(data, &config) != nil || len(config.Outbounds) != 2 ||
		config.Outbounds[1].UDPRelay == nil || config.Outbounds[1].UDPRelay.Addr != "192.0.2.10:51820" ||
		config.UserOutbounds["ordinary"] != "DIRECT" || config.UserOutbounds["relay"] != "WG" {
		t.Fatalf("configured relay or existing routing lost: %s %v", data, err)
	}
	if err := preserveUDPRelayTypes(stored, []byte("outbounds: [{name: wg, type: direct, direct: {}}]")); err == nil {
		t.Fatal("relay changed to unrestricted outbound")
	}
	for _, invalid := range []string{"example.org:51820", "[::1]:51820", "192.0.2.10:0", "255.255.255.255:42"} {
		config.Outbounds[1].UDPRelay.Addr = invalid
		if err := ValidateHysteria2OutboundConfig(config); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
}

func TestUDPRelayKernelCapability(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("shell fixture requires Linux")
	}
	path := filepath.Join(t.TempDir(), "hy2")
	for _, data := range []struct {
		json      string
		supported bool
	}{
		{`{"udpRelay":1}`, true}, {`{"udpRelay":2}`, false}, {`{}`, false}, {"invalid", false},
	} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s' '"+data.json+"'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		if kernelSupportsUDPRelay(path) != data.supported {
			t.Fatalf("unexpected capability for %s", data.json)
		}
	}
}

func TestChangeFixedUDPRelayPreservesExistingConfiguration(t *testing.T) {
	var config bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal([]byte("listen: ':443'\noutbounds: [{name: DIRECT, type: direct, direct: {}}]\nuserOutbounds: {ordinary: DIRECT}"), &config); err != nil {
		t.Fatal(err)
	}
	if err := changeFixedUDPRelay(&config, "", &FixedUDPRelay{Name: " WG ", Addr: " 192.0.2.10:51820 "}, nil); err != nil {
		t.Fatal(err)
	}
	relay := config.Outbounds[1]
	if len(config.Outbounds) != 2 || *config.Outbounds[0].Name != "DIRECT" ||
		!isUDPRelay(relay) || *relay.Name != "WG" || relay.UDPRelay.Addr != "192.0.2.10:51820" ||
		*config.Listen != ":443" || config.UserOutbounds["ordinary"] != "DIRECT" {
		t.Fatalf("unexpected relay merge: %#v", config)
	}
	for _, invalid := range []FixedUDPRelay{
		{Name: "wg", Addr: "192.0.2.20:51820"},
		{Name: "another", Addr: "example.org:51820"},
		{Name: " ", Addr: "192.0.2.20:51820"},
	} {
		if err := changeFixedUDPRelay(&config, "", &invalid, nil); err == nil || len(config.Outbounds) != 2 {
			t.Fatalf("invalid relay altered config: %#v, %v", invalid, err)
		}
	}
}

func TestFixedUDPRelayEditAndDeleteRespectBindings(t *testing.T) {
	var config bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal([]byte("outbounds: [{name: DIRECT, type: direct, direct: {}}, {name: WG, type: udpRelay, udpRelay: {addr: '192.0.2.10:51820'}}]"), &config); err != nil {
		t.Fatal(err)
	}
	bound := "wg"
	accounts := []entity.Account{{Outbound: &bound}}
	updated := FixedUDPRelay{Name: "WG", Addr: "192.0.2.20:51820"}
	if err := changeFixedUDPRelay(&config, "WG", &updated, accounts); err != nil || config.Outbounds[1].UDPRelay.Addr != updated.Addr {
		t.Fatalf("bound relay endpoint edit failed: %v", err)
	}
	renamed := FixedUDPRelay{Name: "WG_NEW", Addr: updated.Addr}
	for _, next := range []*FixedUDPRelay{nil, &renamed} {
		if err := changeFixedUDPRelay(&config, "WG", next, accounts); err == nil {
			t.Fatal("allowed a change that would break an account binding")
		}
	}
	if err := changeFixedUDPRelay(&config, "DIRECT", nil, nil); err == nil {
		t.Fatal("deleted a non-relay outbound")
	}
	duplicate := FixedUDPRelay{Name: "direct", Addr: updated.Addr}
	if err := changeFixedUDPRelay(&config, "WG", &duplicate, nil); err == nil || *config.Outbounds[1].Name != "WG" {
		t.Fatalf("duplicate rename changed the relay: %v", err)
	}
	if err := changeFixedUDPRelay(&config, "WG", &renamed, nil); err != nil {
		t.Fatal(err)
	}
	if err := changeFixedUDPRelay(&config, "WG_NEW", nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(config.Outbounds) != 1 || *config.Outbounds[0].Name != "DIRECT" {
		t.Fatalf("unexpected surviving outbounds: %#v", config.Outbounds)
	}
}
