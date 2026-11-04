package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/vo"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const testXrayUUID = "123e4567-e89b-12d3-a456-426614174000"

func validXraySettings() bo.XraySettings {
	return bo.XraySettings{
		Name: "TEST_VLESS", Version: "v25.10.15", Address: "vless.example.test",
		Port: 1075, UUID: testXrayUUID, TLSSNI: "tls.example.test", UseCustomCA: true,
	}
}

func TestGenerateXrayConfigIsLoopbackTLSVerifiedAndTyped(t *testing.T) {
	data, err := GenerateXrayConfig(validXraySettings())
	if err != nil {
		t.Fatal(err)
	}
	var config bo.XrayConfig
	if err = json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Inbounds) != 1 || config.Inbounds[0].Listen != "127.0.0.1" || config.Inbounds[0].Port != 10808 {
		t.Fatalf("SOCKS inbound is not loopback-only: %+v", config.Inbounds)
	}
	if !config.Inbounds[0].Settings.UDP {
		t.Fatal("SOCKS UDP is disabled")
	}
	if len(config.Outbounds) != 2 || config.Outbounds[0].Protocol != "blackhole" || config.Outbounds[1].Protocol != "vless" {
		t.Fatal("missing VLESS outbound")
	}
	outbound := config.Outbounds[1]
	settingsMap, ok := outbound.Settings.(map[string]interface{})
	if !ok || settingsMap["id"] != testXrayUUID || settingsMap["address"] != "vless.example.test" {
		t.Fatal("VLESS settings changed")
	}
	if outbound.StreamSettings == nil || outbound.StreamSettings.Network != "tcp" || outbound.StreamSettings.Security != "tls" {
		t.Fatal("TCP/TLS not configured")
	}
	tlsSettings := outbound.StreamSettings.TLSSettings
	if tlsSettings.AllowInsecure || !tlsSettings.DisableSystemRoot {
		t.Fatal("TLS verification is not strict")
	}
	if tlsSettings.ServerName != "tls.example.test" || len(tlsSettings.Certificates) != 1 || tlsSettings.Certificates[0].Usage != "verify" {
		t.Fatal("custom CA verification is not configured")
	}
}

func TestMaskedXraySettingsNeverReturnsUUID(t *testing.T) {
	masked := maskedXraySettings(validXraySettings())
	if len(masked.Outbounds) != 1 || !masked.Outbounds[0].UUIDSet || masked.Outbounds[0].UUID != "********" {
		t.Fatalf("unexpected masked settings: %+v", masked.Outbounds)
	}
	encoded, _ := json.Marshal(masked)
	if strings.Contains(string(encoded), testXrayUUID) {
		t.Fatal("full UUID leaked")
	}
}

func TestLegacyXraySettingsMigrateToGenericNode(t *testing.T) {
	settings := normalizeXraySettings(validXraySettings())
	if len(settings.Outbounds) != 1 {
		t.Fatalf("outbounds = %+v", settings.Outbounds)
	}
	outbound := settings.Outbounds[0]
	if outbound.ID != "legacy-vless" || outbound.Name != "TEST_VLESS" || outbound.SOCKSListen != "127.0.0.1" || outbound.SOCKSPort != 10808 || outbound.UUID != testXrayUUID {
		t.Fatalf("legacy outbound changed: %+v", outbound)
	}
}

func TestGenerateXrayConfigRoutesEachSOCKSPortToItsOwnOutbound(t *testing.T) {
	settings := normalizeXraySettings(validXraySettings())
	settings.Outbounds = append(settings.Outbounds, bo.XrayManagedOutbound{
		ID: "hk-1", Name: "HK_NET_1", Enabled: true, Address: "hk.example.com", Port: 443,
		UUID: "223e4567-e89b-12d3-a456-426614174000", Network: "tcp", TLSSNI: "hk.example.com",
		SOCKSListen: "127.0.0.1", SOCKSPort: 10809, OutboundTag: "hui-out-hk-1",
	})
	data, err := GenerateXrayConfig(settings)
	if err != nil {
		t.Fatal(err)
	}
	var config bo.XrayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Inbounds) != 2 || len(config.Outbounds) != 3 || len(config.Routing.Rules) != 3 {
		t.Fatalf("unexpected config sizes: %+v", config)
	}
	for index, port := range []uint16{10808, 10809} {
		if config.Inbounds[index].Port != port || config.Routing.Rules[index].InboundTag[0] != config.Inbounds[index].Tag || config.Routing.Rules[index].OutboundTag != config.Outbounds[index+1].Tag {
			t.Fatalf("route %d is ambiguous: inbound=%+v rule=%+v outbound=%+v", index, config.Inbounds[index], config.Routing.Rules[index], config.Outbounds[index+1])
		}
	}
	if config.Outbounds[2].StreamSettings == nil || config.Outbounds[2].StreamSettings.TLSSettings.AllowInsecure || config.Outbounds[2].StreamSettings.TLSSettings.DisableSystemRoot {
		t.Fatal("system CA outbound does not use strict system trust")
	}
	catchAll := config.Routing.Rules[len(config.Routing.Rules)-1]
	if catchAll.OutboundTag != "hui-unmatched-block" || catchAll.Network != "tcp,udp" {
		t.Fatalf("missing fail-closed fallback: %+v", catchAll)
	}
}

func TestXrayOutboundValidationRejectsDuplicateNamesAndPorts(t *testing.T) {
	settings := normalizeXraySettings(validXraySettings())
	second := settings.Outbounds[0]
	second.ID = "second"
	second.OutboundTag = "hui-out-second"
	settings.Outbounds = append(settings.Outbounds, second)
	if err := validateXraySettings(settings, false); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("duplicate name error = %v", err)
	}
	settings.Outbounds[1].Name = "HK_NET_1"
	if err := validateXraySettings(settings, false); err == nil || !strings.Contains(err.Error(), "SOCKS5 port") {
		t.Fatalf("duplicate port error = %v", err)
	}
}

func TestSaveXrayDraftCreatesEditsAndDeletesUnreferencedOutbound(t *testing.T) {
	_, _ = setupSubscriptionTestDatabase(t)
	settings := normalizeXraySettings(validXraySettings())
	settings.Outbounds = append(settings.Outbounds, bo.XrayManagedOutbound{
		Name: "HK_NET_1", Enabled: true, Address: "hk.example.com", Port: 443,
		UUID: "223e4567-e89b-12d3-a456-426614174000", Network: "tcp", TLSSNI: "hk.example.com",
		SOCKSListen: "127.0.0.1", SOCKSPort: 10809,
	})
	if err := SaveXrayDraft(settings); err != nil {
		t.Fatalf("create second outbound: %v", err)
	}
	stored, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil || len(stored.Outbounds) != 2 || stored.Outbounds[1].ID == "" {
		t.Fatalf("stored outbounds = %+v error = %v", stored.Outbounds, err)
	}
	stored.Outbounds[1].Name = "HK_NET_RENAMED"
	if err := SaveXrayDraft(stored); err != nil {
		t.Fatalf("edit outbound: %v", err)
	}
	stored.Outbounds = stored.Outbounds[:1]
	if err := SaveXrayDraft(stored); err != nil {
		t.Fatalf("delete unreferenced outbound: %v", err)
	}
}

func TestReferencedXrayOutboundCannotBeDisabledOrDeleted(t *testing.T) {
	_, _ = setupSubscriptionTestDatabase(t)
	current := normalizeXraySettings(validXraySettings())
	next := current
	next.Outbounds = append([]bo.XrayManagedOutbound(nil), current.Outbounds...)
	next.Outbounds[0].Enabled = false
	next.Outbounds = append(next.Outbounds, bo.XrayManagedOutbound{
		ID: "hk-1", Name: "HK_NET_1", Enabled: true, Address: "hk.example.com", Port: 443,
		UUID: "223e4567-e89b-12d3-a456-426614174000", Network: "tcp", TLSSNI: "hk.example.com",
		SOCKSListen: "127.0.0.1", SOCKSPort: 10809, OutboundTag: "hui-out-hk-1",
	})
	if err := validateXrayOutboundReferences(current, next); err == nil || !strings.Contains(err.Error(), "still referenced") {
		t.Fatalf("disable reference error = %v", err)
	}
	deleted := next
	deleted.Outbounds = append([]bo.XrayManagedOutbound(nil), next.Outbounds[1:]...)
	if err := validateXrayOutboundReferences(current, deleted); err == nil {
		t.Fatal("deleted referenced outbound was accepted")
	}
}

func TestDisabledXrayOutboundIsNotGenerated(t *testing.T) {
	settings := normalizeXraySettings(validXraySettings())
	disabled := settings.Outbounds[0]
	disabled.ID, disabled.Name, disabled.SOCKSPort, disabled.OutboundTag, disabled.Enabled = "jp-1", "JP_NET_1", 10810, "hui-out-jp-1", false
	settings.Outbounds = append(settings.Outbounds, disabled)
	data, err := GenerateXrayConfig(settings)
	if err != nil {
		t.Fatal(err)
	}
	var config bo.XrayConfig
	_ = json.Unmarshal(data, &config)
	if len(config.Inbounds) != 1 || len(config.Outbounds) != 2 || len(config.Routing.Rules) != 2 {
		t.Fatalf("disabled outbound was generated: %+v", config)
	}
}

func TestGenerateXrayConfigSupportsHTTPAndKeepsNativeSOCKSOutOfXray(t *testing.T) {
	settings := normalizeXraySettings(validXraySettings())
	settings.Outbounds = append(settings.Outbounds,
		bo.XrayManagedOutbound{ID: "http-1", Name: "US_HTTP_1", Protocol: "http", Enabled: true, Address: "proxy.example.com", Port: 8080, Username: "user", Password: "pass", SOCKSListen: "127.0.0.1", SOCKSPort: 10809, OutboundTag: "hui-out-http-1"},
		bo.XrayManagedOutbound{ID: "socks-1", Name: "JP_SOCKS_1", Protocol: "socks5", Enabled: true, Address: "socks.example.com", Port: 1080},
	)
	data, err := GenerateXrayConfig(settings)
	if err != nil {
		t.Fatal(err)
	}
	var config bo.XrayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Inbounds) != 2 || config.Inbounds[1].Settings.UDP {
		t.Fatalf("HTTP adapter must be TCP-only: %+v", config.Inbounds)
	}
	if len(config.Outbounds) != 3 || config.Outbounds[2].Protocol != "http" {
		t.Fatalf("unexpected Xray outbounds: %+v", config.Outbounds)
	}
	encoded := string(data)
	if strings.Contains(encoded, "socks.example.com") || !strings.Contains(encoded, `"user": "user"`) {
		t.Fatalf("native SOCKS leaked into Xray or HTTP auth missing: %s", encoded)
	}
}

func TestMaskedXraySettingsNeverReturnsProxyPassword(t *testing.T) {
	settings := normalizeXraySettings(validXraySettings())
	settings.Outbounds = append(settings.Outbounds, bo.XrayManagedOutbound{ID: "http-1", Name: "HTTP", Protocol: "http", Enabled: true, Address: "proxy.example.com", Port: 8080, Username: "alice", Password: "top-secret", SOCKSListen: "127.0.0.1", SOCKSPort: 10809, OutboundTag: "hui-out-http-1"})
	masked := maskedXraySettings(settings)
	data, _ := json.Marshal(masked)
	if strings.Contains(string(data), "top-secret") || !masked.Outbounds[1].PasswordSet {
		t.Fatalf("proxy password exposure or missing marker: %s", data)
	}
}

func TestPerOutboundConnectivityTestUsesSelectedSOCKSProxy(t *testing.T) {
	selected := "127.0.0.1:10809"
	tcpProxy := ""
	result := runXrayOutboundTest(vo.XrayTestVo{Protocol: "tcp", Supported: true}, selected,
		func() error { return nil },
		func(proxy string, _ time.Duration) (string, error) { tcpProxy = proxy; return "203.0.113.9", nil },
		func(string, time.Duration) error { return errors.New("unexpected UDP test") },
	)
	if tcpProxy != selected || !result.RemoteConnectivity || !result.ExitIdentityVerified || result.ExitIP != "203.0.113.9" {
		t.Fatalf("TCP result = %+v proxy = %q", result, tcpProxy)
	}

	udpProxy := ""
	result = runXrayOutboundTest(vo.XrayTestVo{Protocol: "udp", Supported: true}, selected,
		func() error { return nil },
		func(string, time.Duration) (string, error) { return "", errors.New("unexpected TCP test") },
		func(proxy string, _ time.Duration) error { udpProxy = proxy; return nil },
	)
	if udpProxy != selected || !result.RemoteConnectivity || result.ExitIdentityVerified {
		t.Fatalf("UDP result = %+v proxy = %q", result, udpProxy)
	}
}

func TestPerOutboundConnectivityTestStopsWhenManagedServiceIsUnavailable(t *testing.T) {
	called := false
	result := runXrayOutboundTest(vo.XrayTestVo{Protocol: "tcp", Supported: true}, "127.0.0.1:10808",
		func() error { return errors.New("h-ui-xray.service is not active") },
		func(string, time.Duration) (string, error) { called = true; return "", nil },
		func(string, time.Duration) error { called = true; return nil },
	)
	if called || !strings.Contains(result.Error, "not active") || result.LocalProxyAvailable {
		t.Fatalf("result = %+v called = %v", result, called)
	}
}

func TestGenericConfigAPIRejectsXraySecrets(t *testing.T) {
	for _, key := range []string{constant.XrayDraftConfig, constant.XrayActiveConfig} {
		if !IsSensitiveConfigKey(key) {
			t.Fatalf("Xray config key %q is not marked sensitive", key)
		}
		if _, err := GetConfig(key); err == nil || err.Error() != constant.ForbiddenError {
			t.Fatalf("GetConfig exposed %q: %v", key, err)
		}
		if _, err := ListConfig([]string{constant.HUIWebPort, key}); err == nil || err.Error() != constant.ForbiddenError {
			t.Fatalf("ListConfig exposed %q: %v", key, err)
		}
	}
}

func TestValidateCAAcceptsCAAndRejectsLeaf(t *testing.T) {
	ca := makeTestCertificate(t, true)
	status, err := validateCA(ca)
	if err != nil || !status.Valid || status.Fingerprint == "" {
		t.Fatalf("valid CA rejected: %+v %v", status, err)
	}
	if _, err = validateCA(makeTestCertificate(t, false)); err == nil {
		t.Fatal("leaf certificate accepted as CA")
	}
	if _, err = validateCA([]byte("not pem")); err == nil {
		t.Fatal("non-PEM data accepted")
	}
}

func TestSystemdUnitUsesDedicatedAccountAndFixedPaths(t *testing.T) {
	unit := xraySystemdUnit(managedXrayPaths)
	for _, expected := range []string{
		"User=h-ui-xray", "Group=h-ui-xray", "Restart=on-failure",
		"ExecStart=/var/lib/h-ui/xray/current/xray run -c /var/lib/h-ui/xray/config.json",
		"ReadOnlyPaths=/var/lib/h-ui/xray", "NoNewPrivileges=true",
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("unit missing %q", expected)
		}
	}
	if strings.Contains(unit, "0.0.0.0") {
		t.Fatal("systemd unit exposes a public listener")
	}
}

func TestWriteXraySystemdUnitCreatesUnitAndReloadsSystemd(t *testing.T) {
	paths := testXrayPaths(t.TempDir())
	oldCommand := xrayCommand
	var calls [][]string
	xrayCommand = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return nil, nil
	}
	defer func() { xrayCommand = oldCommand }()

	if err := writeXraySystemdUnit(paths); err != nil {
		t.Fatal(err)
	}
	unit, err := os.ReadFile(paths.systemdUnit)
	if err != nil {
		t.Fatal(err)
	}
	if string(unit) != xraySystemdUnit(paths) {
		t.Fatal("installed systemd unit does not match the managed unit")
	}
	if len(calls) != 1 || strings.Join(calls[0], " ") != "systemctl daemon-reload" {
		t.Fatalf("systemd reload calls = %v", calls)
	}
	if err := writeXraySystemdUnit(paths); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("unchanged unit unexpectedly reloaded systemd: %v", calls)
	}
}

func TestXrayStartRejectsMissingServiceBeforeSystemctl(t *testing.T) {
	oldPaths, oldCommand := managedXrayPaths, xrayCommand
	managedXrayPaths = testXrayPaths(t.TempDir())
	called := false
	xrayCommand = func(string, ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	defer func() { managedXrayPaths, xrayCommand = oldPaths, oldCommand }()

	err := StartXray()
	if err == nil || !strings.Contains(err.Error(), "systemd service is not installed") {
		t.Fatalf("missing service error = %v", err)
	}
	if called {
		t.Fatal("missing Xray service still invoked systemctl")
	}
}

func TestFinalizedXrayInstallVerifiesBinaryAndEnsuresRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable permission semantics differ on Windows")
	}
	paths := testXrayPaths(t.TempDir())
	version := "v25.10.15"
	binaryPath := filepath.Join(paths.versionsDir, version, "xray")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("trusted binary"), 0750); err != nil {
		t.Fatal(err)
	}
	writeXrayManifest(t, paths, version, binaryPath)

	oldCommand := xrayCommand
	xrayCommand = func(_ string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "version" {
			return []byte("Xray 25.10.15\n"), nil
		}
		return []byte("Configuration OK"), nil
	}
	defer func() { xrayCommand = oldCommand }()
	runtime := &fakeXrayRuntime{}
	if err := finalizeInstalledXrayVersion(paths, version, runtime); err != nil {
		t.Fatal(err)
	}
	if runtime.ensureCalls != 1 {
		t.Fatalf("systemd runtime ensure calls = %d", runtime.ensureCalls)
	}
}

func TestAtomicWriteUsesRestrictedPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := atomicWriteFile(path, []byte("{}"), 0640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatalf("unexpected config mode: %o", info.Mode().Perm())
	}
}

func TestXrayParentAccessIsTraverseOnlyForServiceGroup(t *testing.T) {
	dataDir := filepath.Join("var", "lib", "h-ui", "xray")
	var chownPath, chmodPath string
	var uid, gid int
	var mode os.FileMode
	err := applyXrayParentAccess(dataDir, 4321,
		func(path string, owner, group int) error { chownPath, uid, gid = path, owner, group; return nil },
		func(path string, permissions os.FileMode) error { chmodPath, mode = path, permissions; return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join("var", "lib", "h-ui")
	if chownPath != expected || chmodPath != expected || uid != 0 || gid != 4321 || mode.Perm() != 0710 {
		t.Fatalf("unexpected parent access: chown=%q chmod=%q uid=%d gid=%d mode=%o", chownPath, chmodPath, uid, gid, mode.Perm())
	}
}

func TestTCPTableRequiresListenerOwnedByManagedPID(t *testing.T) {
	table := []byte("  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:2A38 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 4242 1\n")
	if !tcpTableHasOwnedListener(table, "127.0.0.1", 10808, map[string]struct{}{"4242": {}}) {
		t.Fatal("managed listener was not recognized")
	}
	if tcpTableHasOwnedListener(table, "127.0.0.1", 10808, map[string]struct{}{"9999": {}}) {
		t.Fatal("listener owned by another process was accepted")
	}
}

func TestInstalledXrayRequiresManifestAndMatchingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable permission semantics differ on Windows")
	}
	paths := testXrayPaths(t.TempDir())
	version := "v25.10.15"
	binaryPath := filepath.Join(paths.versionsDir, version, "xray")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("trusted binary"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := verifyInstalledXrayVersion(paths, version); err == nil {
		t.Fatal("version without an install manifest was trusted")
	}

	oldCommand := xrayCommand
	xrayCommand = func(name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "version" {
			return []byte("Xray 25.10.15\n"), nil
		}
		return []byte("Configuration OK"), nil
	}
	defer func() { xrayCommand = oldCommand }()
	officialHash, err := fileSHA256(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(xrayInstallManifest{Version: version, BinarySHA256: officialHash, ArchiveSHA256: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if err = attestExistingXrayVersion(paths, version, officialHash, manifest, func(string) error { return nil }); err != nil {
		t.Fatalf("official legacy binary could not be attested: %v", err)
	}
	if err := verifyInstalledXrayVersion(paths, version); err != nil {
		t.Fatalf("trusted version rejected: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("tampered binary"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := attestExistingXrayVersion(paths, version, officialHash, manifest, func(string) error { return nil }); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("non-official legacy binary was attested: %v", err)
	}
	if err := verifyInstalledXrayVersion(paths, version); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("tampered version accepted: %v", err)
	}
}

func TestValidateSnapshotReturnsExactCandidateForSelectedVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable permission semantics differ on Windows")
	}
	paths := testXrayPaths(t.TempDir())
	if err := os.MkdirAll(paths.certDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ca, makeTestCertificate(t, true), 0640); err != nil {
		t.Fatal(err)
	}
	settings := validXraySettings()
	binaryPath := filepath.Join(paths.versionsDir, settings.Version, "xray")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("trusted binary"), 0750); err != nil {
		t.Fatal(err)
	}
	writeXrayManifest(t, paths, settings.Version, binaryPath)

	oldPaths, oldCommand := managedXrayPaths, xrayCommand
	managedXrayPaths = paths
	var commands []string
	xrayCommand = func(name string, args ...string) ([]byte, error) {
		commands = append(commands, name)
		if name != binaryPath {
			return nil, fmt.Errorf("unexpected Xray binary %s", name)
		}
		if len(args) == 1 && args[0] == "version" {
			return []byte("Xray 25.10.15\n"), nil
		}
		return []byte("Configuration OK"), nil
	}
	defer func() { managedXrayPaths, xrayCommand = oldPaths, oldCommand }()

	result, snapshot, err := validateXraySnapshot(settings)
	if err != nil || !result.Valid {
		t.Fatalf("snapshot validation failed: %+v %v", result, err)
	}
	candidate, err := os.ReadFile(paths.candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(snapshot, candidate) {
		t.Fatal("validated bytes differ from returned immutable snapshot")
	}
	if len(commands) < 3 {
		t.Fatalf("expected integrity and config checks with selected binary, got %v", commands)
	}
}

func TestVerifyXrayDigestRequiresMatchingOfficialSHA256Line(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Xray-linux-64.zip")
	content := []byte("official archive fixture")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	digest := []byte("SHA2-256(Xray-linux-64.zip)= " + hex.EncodeToString(sum[:]))
	if err := verifyXrayDigest(path, digest); err != nil {
		t.Fatal(err)
	}
	if err := verifyXrayDigest(path, []byte("SHA2-256(Xray-linux-64.zip)= "+strings.Repeat("0", 64))); err == nil {
		t.Fatal("mismatched digest accepted")
	}
}

func TestApplyRollbackRestoresPreviousFilesAndRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows test environments commonly disallow symlink creation")
	}
	root := t.TempDir()
	paths := xrayPaths{
		dataDir: root, versionsDir: filepath.Join(root, "versions"), currentLink: filepath.Join(root, "current"),
		config: filepath.Join(root, "config.json"), previous: filepath.Join(root, "previous.json"),
		candidate: filepath.Join(root, "candidate.json"), certDir: filepath.Join(root, "certs"), ca: filepath.Join(root, "certs", "ca.crt"),
		systemdUnit: filepath.Join(root, "h-ui-xray.service"),
	}
	oldVersion := filepath.Join(paths.versionsDir, "v1.0.0")
	newVersion := filepath.Join(paths.versionsDir, "v25.10.15")
	for _, dir := range []string{oldVersion, newVersion} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(oldVersion, "xray"), []byte("old"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newVersion, "xray"), []byte("new"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldVersion, paths.currentLink); err != nil {
		t.Fatal(err)
	}
	oldConfig := []byte(`{"old":true}`)
	if err := os.WriteFile(paths.config, oldConfig, 0640); err != nil {
		t.Fatal(err)
	}
	runtime := &fakeXrayRuntime{running: true, failFirstRestart: true}
	err := applyXrayConfiguration(validXraySettings(), []byte(`{"new":true}`), paths, runtime)
	if err == nil || !strings.Contains(err.Error(), "previous state was restored") {
		t.Fatalf("expected restored apply failure, got %v", err)
	}
	currentConfig, _ := os.ReadFile(paths.config)
	if string(currentConfig) != string(oldConfig) {
		t.Fatalf("old config not restored: %s", currentConfig)
	}
	target, err := filepath.EvalSymlinks(paths.currentLink)
	if err != nil || target != oldVersion {
		t.Fatalf("old binary link not restored: %s %v", target, err)
	}
	if runtime.restartCalls != 2 {
		t.Fatalf("expected apply and rollback restart, got %d", runtime.restartCalls)
	}
	if runtime.prepareCalls != 2 || runtime.readyCalls != 1 {
		t.Fatalf("rollback did not restore permissions and verify readiness: prepare=%d ready=%d", runtime.prepareCalls, runtime.readyCalls)
	}
}

func TestApplyRollbackNeverReportsSuccessWhenRestoredServiceIsNotReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows test environments commonly disallow symlink creation")
	}
	paths := testXrayPaths(t.TempDir())
	oldVersion := filepath.Join(paths.versionsDir, "v1.0.0")
	newVersion := filepath.Join(paths.versionsDir, "v25.10.15")
	for _, dir := range []string{oldVersion, newVersion} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(oldVersion, paths.currentLink); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.config, []byte(`{"old":true}`), 0640); err != nil {
		t.Fatal(err)
	}
	runtime := &fakeXrayRuntime{running: true, failFirstRestart: true, readyErr: errors.New("old service did not listen")}
	err := applyXrayConfiguration(validXraySettings(), []byte(`{"new":true}`), paths, runtime)
	if err == nil || !strings.Contains(err.Error(), "rollback failed") || strings.Contains(err.Error(), "previous state was restored") {
		t.Fatalf("rollback readiness failure was misreported: %v", err)
	}
}

type fakeXrayRuntime struct {
	running          bool
	ensureCalls      int
	failFirstRestart bool
	restartCalls     int
	prepareCalls     int
	readyCalls       int
	readyErr         error
}

func (f *fakeXrayRuntime) Ensure() error   { f.ensureCalls++; return nil }
func (f *fakeXrayRuntime) IsRunning() bool { return f.running }
func (f *fakeXrayRuntime) Start() error    { f.running = true; return nil }
func (f *fakeXrayRuntime) Stop() error     { f.running = false; return nil }
func (f *fakeXrayRuntime) Restart() error {
	f.restartCalls++
	if f.failFirstRestart && f.restartCalls == 1 {
		return errors.New("simulated restart failure")
	}
	return nil
}
func (f *fakeXrayRuntime) Ready() error              { f.readyCalls++; return f.readyErr }
func (f *fakeXrayRuntime) PrepareFiles(string) error { f.prepareCalls++; return nil }

func testXrayPaths(root string) xrayPaths {
	return xrayPaths{
		dataDir: root, versionsDir: filepath.Join(root, "versions"), currentLink: filepath.Join(root, "current"),
		config: filepath.Join(root, "config.json"), previous: filepath.Join(root, "previous.json"),
		candidate: filepath.Join(root, "candidate.json"), certDir: filepath.Join(root, "certs"), ca: filepath.Join(root, "certs", "ca.crt"),
		systemdUnit: filepath.Join(root, "h-ui-xray.service"),
	}
}

func writeXrayManifest(t *testing.T, paths xrayPaths, version, binaryPath string) {
	t.Helper()
	hash, err := fileSHA256(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(xrayInstallManifest{Version: version, BinarySHA256: hash, ArchiveSHA256: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(filepath.Dir(binaryPath), xrayManifestName), data, 0640); err != nil {
		t.Fatal(err)
	}
}

func makeTestCertificate(t *testing.T, isCA bool) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "H-UI test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		BasicConstraintsValid: true, IsCA: isCA, KeyUsage: x509.KeyUsageDigitalSignature,
	}
	if isCA {
		template.KeyUsage |= x509.KeyUsageCertSign
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
