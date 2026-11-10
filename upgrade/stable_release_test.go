package upgrade

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStableVersionMigrationOrdering(t *testing.T) {
	ordered := []string{"v0.0.25-custom.8-rc.9", "v0.0.25-custom.8-rc.23", "v0.0.25-custom.8", "v1.0.0-custom.1-rc.1", "v1.0.0-custom.1", "v1.0.0", "v1.0.1", "v1.10.0"}
	for i, version := range ordered {
		if !releaseTagPattern.MatchString(version) {
			t.Fatalf("rejected %s", version)
		}
		if i > 0 && compareReleaseTags(ordered[i-1], version) >= 0 {
			t.Fatalf("wrong order: %s >= %s", ordered[i-1], version)
		}
	}
	for _, invalid := range []string{"v01.0.0", "v1.0.0-rc.01", "v2.13.0-hui.4", "latest", "v1.0.0/../../"} {
		if releaseTagPattern.MatchString(invalid) {
			t.Fatalf("accepted unsupported version %s", invalid)
		}
	}
}

func TestVerifiedCandidateMigratesRC23ToStable(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	f := newUpgradeFixture(t, health.URL)
	f.opts.CurrentVersion = "v0.0.25-custom.8-rc.23"
	f.opts.Prerelease = false
	original := f.deps.Command
	f.deps.Command = func(name string, args ...string) ([]byte, error) {
		if name != "systemctl" && len(args) == 1 && args[0] == "version" {
			return []byte("Better HY2 version v1.0.0"), nil
		}
		return original(name, args...)
	}
	config := filepath.Join(f.opts.DataDir, "bin", "hysteria2.yaml")
	if err := os.MkdirAll(filepath.Dir(config), 0755); err != nil {
		t.Fatal(err)
	}
	before := "userOutbounds:\n  existing-account: DIRECT\n"
	if err := os.WriteFile(config, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), f.opts, f.deps); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(config)
	if err != nil || string(after) != before {
		t.Fatalf("existing routing modified: %v", err)
	}
	if link, err := os.Readlink(filepath.Join(f.opts.InstallDir, "hui")); err != nil || link != filepath.Join(f.opts.InstallDir, "h-ui") {
		t.Fatalf("CLI link: %s %v", link, err)
	}
}

func TestStableChannelCannotDowngradeToLegacyRC(t *testing.T) {
	f := newUpgradeFixture(t, "")
	f.opts.CurrentVersion = "v1.0.0"
	f.opts.Prerelease = true
	err := Run(context.Background(), f.opts, f.deps)
	if err == nil || !strings.Contains(err.Error(), "refusing automatic downgrade") {
		t.Fatalf("unexpected error %v", err)
	}
	if len(f.commands) != 0 {
		t.Fatalf("downgrade touched service: %v", f.commands)
	}
}

func TestCLILinkDoesNotOverwriteOtherSoftware(t *testing.T) {
	dir := t.TempDir()
	link, target := filepath.Join(dir, "hui"), filepath.Join(dir, "h-ui")
	if err := os.WriteFile(link, []byte("other software"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCLILink(link, target); err == nil {
		t.Fatal("overwrote another command")
	}
	content, _ := os.ReadFile(link)
	if string(content) != "other software" {
		t.Fatal("existing command changed")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := EnsureCLILink(link, target); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrationRollbackKeepsHuiEntryPointOnRestoredBinary(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	f := newUpgradeFixture(t, health.URL)
	original := f.deps.Command
	failed := false
	f.deps.Command = func(name string, args ...string) ([]byte, error) {
		if name == "systemctl" && args[0] == "daemon-reload" && !failed {
			failed = true
			return nil, errors.New("injected reload failure")
		}
		return original(name, args...)
	}
	err := Run(context.Background(), f.opts, f.deps)
	if err == nil || !strings.Contains(err.Error(), "rollback successful") {
		t.Fatalf("unexpected outcome: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(f.opts.InstallDir, "hui"))
	if err != nil || string(content) != "old-binary" {
		t.Fatalf("CLI not restored: %v", err)
	}
	unit, err := os.ReadFile(f.opts.SystemdUnitPath)
	if err != nil || string(unit) != oldSystemdUnit {
		t.Fatalf("service unit not restored: %v", err)
	}
}

func TestInstallerStableMigrationAndChannelGuard(t *testing.T) {
	prefix := "HUI_INSTALLER_TEST=1; source " + shellQuote(installerPath(t)) + "; "
	script := prefix + `allowed_version v1.0.0; parse_args --version v1.0.0; [[ "$requested_version" == v1.0.0 ]]; version_is_older v0.0.25-custom.8-rc.23 v1.0.0; ! version_is_older v1.0.0 v0.0.25-custom.8-rc.23; curl(){ printf '%s' '{"tag_name":"v1.0.0","prerelease":false,"draft":false}'; }; requested_version=latest; [[ "$(resolve_version)" == v1.0.0 ]]`
	if output, err := runBash(t, script); err != nil {
		t.Fatalf("stable migration: %v %s", err, output)
	}
	script = prefix + `curl(){ printf '%s' '{"tag_name":"v1.0.0","prerelease":true}'; }; resolve_version`
	if output, err := runBash(t, script); err == nil || !strings.Contains(output, "stable channel returned") {
		t.Fatalf("prerelease accepted: %v %s", err, output)
	}
}
