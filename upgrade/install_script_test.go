package upgrade

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func installerPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(path)
}

func runBash(t *testing.T, source string) (string, error) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available on this development host; GitHub Actions runs this test on Linux")
	}
	command := exec.Command(bash, "-c", source)
	output, runErr := command.CombinedOutput()
	return string(output), runErr
}

func TestInstallerShellSyntaxAndVersionValidation(t *testing.T) {
	path := installerPath(t)
	if output, err := runBash(t, "bash -n "+shellQuote(path)); err != nil {
		t.Fatalf("installer syntax: %v\n%s", err, output)
	}
	script := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; " +
		"allowed_version v0.0.25-custom.8-rc.9; ! allowed_version latest; ! allowed_version v2.13.0-hui.4"
	if output, err := runBash(t, script); err != nil {
		t.Fatalf("installer version validation: %v\n%s", err, output)
	}
}

func TestInstallerUsesFixedPanelPortAndRejectsOverride(t *testing.T) {
	path := installerPath(t)
	script := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; " +
		`[[ "$panel_port" == "8000" ]]`
	if output, err := runBash(t, script); err != nil {
		t.Fatalf("fixed panel port: %v\n%s", err, output)
	}

	rejectOverride := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; parse_args --port 8081"
	output, err := runBash(t, rejectOverride)
	if err == nil || !strings.Contains(output, "always uses port 8000") {
		t.Fatalf("--port override was not rejected clearly: %v\n%s", err, output)
	}
}

func TestInstallerExtractsViteFrontendEntryAsset(t *testing.T) {
	path := installerPath(t)
	html := `<script type="module" crossorigin src="/assets/index-fe5a903a.js"></script>
<div id="app"></div>`
	script := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; " +
		"asset=$(extract_frontend_entry_asset " + shellQuote(html) + "); " +
		`[[ "$asset" == "/assets/index-fe5a903a.js" ]] && printf '%s' "$asset"`
	output, err := runBash(t, script)
	if err != nil {
		t.Fatalf("extract frontend entry asset: %v\n%s", err, output)
	}
	if output != "/assets/index-fe5a903a.js" {
		t.Fatalf("unexpected frontend entry asset %q", output)
	}
}

func TestInstallerSelectsHighestPrereleaseRegardlessOfAPIOrder(t *testing.T) {
	path := installerPath(t)
	fixture := `[{"tag_name":"v0.0.25-custom.8-rc.11"},{"tag_name":"v0.0.25-custom.8-rc.10"},{"tag_name":"v0.0.25-custom.8-rc.9"}]`
	script := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; " +
		"requested_version=latest; use_prerelease=1; " +
		"curl(){ printf '%s' " + shellQuote(fixture) + "; }; " +
		"resolve_version"
	output, err := runBash(t, script)
	if err != nil {
		t.Fatalf("resolve latest prerelease: %v\n%s", err, output)
	}
	if strings.TrimSpace(output) != "v0.0.25-custom.8-rc.11" {
		t.Fatalf("expected RC.11, got %q", output)
	}
}

func TestInstallerRefusesAutomaticPrereleaseDowngrade(t *testing.T) {
	path := installerPath(t)
	script := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; " +
		"requested_version=latest; ensure_no_automatic_downgrade v0.0.25-custom.8-rc.9 v0.0.25-custom.8-rc.10"
	output, err := runBash(t, script)
	if err == nil || !strings.Contains(output, "refusing automatic downgrade") {
		t.Fatalf("automatic downgrade was not rejected: %v\n%s", err, output)
	}
}

func TestInstallerWorkDirectoryCleanupExitsCleanly(t *testing.T) {
	path := installerPath(t)
	script := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; " +
		`create_installer_work_dir; printf '%s' "$installer_work_dir"`
	output, err := runBash(t, script)
	if err != nil {
		t.Fatalf("installer cleanup should exit cleanly: %v\n%s", err, output)
	}
	workDir := strings.TrimSpace(output)
	if workDir == "" {
		t.Fatal("installer did not report its work directory")
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Fatalf("installer work directory was not removed: %s (%v)", workDir, err)
	}
}

func TestInstallerPlatformChecks(t *testing.T) {
	path := installerPath(t)
	temporary := filepath.ToSlash(t.TempDir())
	supported := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; " +
		"HUI_TEST_EUID=0 HUI_TEST_OS=Linux HUI_TEST_ARCH=amd64 HUI_TEST_SKIP_COMMANDS=1 HUI_TEST_SYSTEMD_DIR=" + shellQuote(temporary) + " preflight_environment"
	if output, err := runBash(t, supported); err != nil {
		t.Fatalf("Linux AMD64 should pass: %v\n%s", err, output)
	}
	unsupported := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; " +
		"HUI_TEST_EUID=0 HUI_TEST_OS=Linux HUI_TEST_ARCH=arm64 HUI_TEST_SKIP_COMMANDS=1 HUI_TEST_SYSTEMD_DIR=" + shellQuote(temporary) + " preflight_environment"
	if output, err := runBash(t, unsupported); err == nil || !strings.Contains(output, "unsupported architecture") {
		t.Fatalf("ARM64 should fail clearly: %v\n%s", err, output)
	}
}

func TestInstallerDownloadAndChecksumFailures(t *testing.T) {
	path := installerPath(t)
	work := filepath.ToSlash(t.TempDir())
	prefix := "HUI_INSTALLER_TEST=1; source " + shellQuote(path) + "; "
	downloadFailure := prefix + "curl(){ return 22; }; download_and_verify v0.0.25-custom.8-rc.9 " +
		shellQuote(work+"/"+BinaryAsset) + " " + shellQuote(work+"/"+ChecksumAsset)
	if output, err := runBash(t, downloadFailure); err == nil || !strings.Contains(output, "failed to download") {
		t.Fatalf("download failure was not reported: %v\n%s", err, output)
	}

	checksumFailure := prefix + `curl(){ local output=""; while (($#)); do if [[ "$1" == "--output" ]]; then output="$2"; shift 2; else shift; fi; done; if [[ "$output" == *SHA256SUMS ]]; then printf '%064d  h-ui-linux-amd64\n' 0 >"$output"; else printf '#!/bin/sh\necho h-ui version v0.0.25-custom.8-rc.9\n' >"$output"; fi; }; ` +
		"download_and_verify v0.0.25-custom.8-rc.9 " + shellQuote(work+"/"+BinaryAsset) + " " + shellQuote(work+"/"+ChecksumAsset)
	if output, err := runBash(t, checksumFailure); err == nil || !strings.Contains(output, "SHA-256 verification failed") {
		t.Fatalf("checksum failure was not reported: %v\n%s", err, output)
	}
}

func TestInstallerContainsPinnedKernelSystemdAndIdempotentUpgradePaths(t *testing.T) {
	content, err := os.ReadFile(installerPath(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	required := []string{
		`"$candidate" install-hysteria2`,
		"install_systemd_unit",
		"systemctl enable",
		`local -a arguments=(upgrade "$version" --yes)`,
		`"${HUI_STATE_DIR}/xray/certs"`,
		"SHA-256 verification failed",
	}
	for _, fragment := range required {
		if !strings.Contains(text, fragment) {
			t.Errorf("installer is missing %q", fragment)
		}
	}
	if strings.Contains(text, "jonssonyan/h-ui") {
		t.Error("custom installer must not silently use the upstream repository")
	}
	freshStart := strings.Index(text, "fresh_install()")
	freshEnd := strings.Index(text, "\nupgrade_existing()")
	if freshStart < 0 || freshEnd <= freshStart {
		t.Fatal("installer fresh_install function was not found")
	}
	freshInstall := text[freshStart:freshEnd]
	ordered := []string{
		`"$candidate" install-hysteria2`,
		`"$candidate" reset`,
		`mv -f "$staged" "${HUI_INSTALL_DIR}/h-ui"`,
		"install_systemd_unit",
		`systemctl restart "$HUI_SERVICE"`,
		"panel_health_check",
	}
	upgradeStart := freshEnd + 1
	upgradeEnd := strings.Index(text[upgradeStart:], "\nvalid_existing_installation()")
	if upgradeEnd < 0 {
		t.Fatal("installer upgrade_existing function was not found")
	}
	upgradeExisting := text[upgradeStart : upgradeStart+upgradeEnd]
	previous := -1
	if !strings.Contains(upgradeExisting, `"$candidate" "${arguments[@]}"`) {
		t.Fatal("installer existing-upgrade path does not invoke the authoritative binary upgrader")
	}
	for _, duplicate := range []string{"install_systemd_unit", `systemctl restart "$HUI_SERVICE"`, "panel_health_check"} {
		if strings.Contains(upgradeExisting, duplicate) {
			t.Fatalf("installer existing-upgrade path duplicates binary upgrader behavior: %s", duplicate)
		}
	}
	if !strings.Contains(text, `readonly panel_port="8000"`) ||
		!strings.Contains(text, `ExecStart=${HUI_INSTALL_DIR}/h-ui -p ${panel_port}`) {
		t.Error("fresh installation does not use the fixed port 8000 systemd unit")
	}
	previous = -1
	for _, fragment := range ordered {
		position := strings.Index(freshInstall, fragment)
		if position < 0 {
			t.Fatalf("installer fresh path is missing %q", fragment)
		}
		if position <= previous {
			t.Fatalf("installer fresh path has %q out of order", fragment)
		}
		previous = position
	}
	if !strings.Contains(text, "valid_existing_installation") || !strings.Contains(text, "Incomplete h-ui installation detected") {
		t.Error("installer does not distinguish a valid installation from a partial fresh install")
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
