package upgrade

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Run the real installer fresh path and real CLI schema/reset against an empty
// data root. Only external HY2 download, systemd and HTTP responses are fixtures;
// no host service, network listener or privileged host path is modified.
func TestFreshStableInstallerWithRealAdministratorInitialization(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("installer requires Linux")
	}
	dir := t.TempDir()
	realBinary := filepath.Join(dir, "panel")
	build := exec.Command("go", "build", "-ldflags=-X h-ui/model/constant.Version=v1.0.0", "-o", realBinary, "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real CLI: %v %s", err, output)
	}
	installDir, stateDir, unitDir := filepath.Join(dir, "install"), filepath.Join(dir, "state"), filepath.Join(dir, "units")
	if err := os.Mkdir(unitDir, 0755); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(dir, "candidate")
	wrapper := "#!/bin/bash\nif [[ $1 == install-hysteria2 ]]; then mkdir -p bin; printf fixture > bin/hysteria2; exit 0; fi\nexec " + shellQuote(realBinary) + " \"$@\"\n"
	if err := os.WriteFile(candidate, []byte(wrapper), 0755); err != nil {
		t.Fatal(err)
	}
	installer, err := os.ReadFile(installerPath(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(installer)
	for original, replacement := range map[string]string{"/usr/local/h-ui": installDir, "/var/lib/h-ui": stateDir, "/etc/systemd/system": unitDir, "/var/lib/hui": filepath.Join(dir, "access"), "/usr/local/bin": filepath.Join(dir, "usrbin")} {
		text = strings.ReplaceAll(text, original, replacement)
	}
	script := filepath.Join(dir, "install.sh")
	if err := os.WriteFile(script, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	commands := "HUI_INSTALLER_TEST=1; source " + shellQuote(script) + "; " +
		`systemctl(){ return 0; }; curl(){ if [[ "${*: -1}" == http://127.0.0.1:8000/ ]]; then printf '%s' '<div id="app"></div><script type="module" src="/assets/entry.js"></script>'; elif [[ "${*: -1}" == http://127.0.0.1:8000/assets/entry.js ]]; then printf 'verified-entry'; else return 22; fi; }; ` +
		"fresh_install v1.0.0 " + shellQuote(candidate) + "; install_cli_link"
	output, err := runBash(t, commands)
	if err != nil {
		t.Fatalf("fresh install: %v %s", err, output)
	}
	username := regexp.MustCompile(`Better HY2 Login Username: (\w+)`).FindStringSubmatch(output)
	password := regexp.MustCompile(`Better HY2 Login Password: (\w+)`).FindStringSubmatch(output)
	if len(username) != 2 || len(password) != 2 {
		t.Fatal("initial credentials not emitted")
	}
	database, err := sql.Open("sqlite", filepath.Join(installDir, "data", "h_ui.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var name, pass string
	if err := database.QueryRow("SELECT username, pass FROM account WHERE id=1").Scan(&name, &pass); err != nil {
		t.Fatal(err)
	}
	if name != username[1] || pass != fmt.Sprintf("%x", sha256.Sum224([]byte(password[1]))) {
		t.Fatal("administrator reset did not update actual fresh database")
	}
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM config").Scan(&count); err != nil || count == 0 {
		t.Fatalf("default configuration missing: %v", err)
	}
	unit, err := os.ReadFile(filepath.Join(unitDir, "h-ui.service"))
	if err != nil || !strings.Contains(string(unit), "-p 8000") {
		t.Fatalf("fresh unit: %v", err)
	}
	if target, err := os.Readlink(filepath.Join(dir, "usrbin", "hui")); err != nil || target != filepath.Join(installDir, "h-ui") {
		t.Fatalf("hui link: %s %v", target, err)
	}
}
