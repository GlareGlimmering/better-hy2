package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
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
