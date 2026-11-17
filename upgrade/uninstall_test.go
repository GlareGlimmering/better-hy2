package upgrade

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"h-ui/model/constant"
	"strings"
	"testing"
)

func TestUninstallStopFailureLeavesFilesForRetry(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux mount table and paths")
	}
	opts := DefaultOptions()
	opts.SystemdUnitPath = filepath.Join(t.TempDir(), opts.ServiceName)
	opts.InstallDir = t.TempDir()
	program := filepath.Join(opts.InstallDir, "h-ui")
	for _, filename := range []string{opts.SystemdUnitPath, program} {
		if err := os.WriteFile(filename, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan := uninstallPlan{services: []uninstallService{{opts.ServiceName, opts.SystemdUnitPath}}, files: []string{program}}
	fail := true
	command := func(_ string, args ...string) (string, error) {
		if fail {
			if args[0] != "stop" {
				t.Fatalf("must stop before %s", args[0])
			}
			return "", errors.New("injected stop failure")
		}
		if args[0] == "show" {
			return "LoadState=not-found\nActiveState=inactive\n", nil
		}
		return "", nil
	}
	if err := applyUninstall(opts, plan, command); err == nil {
		t.Fatal("expected failed uninstall")
	}
	for _, filename := range []string{opts.SystemdUnitPath, program} {
		if _, err := os.Stat(filename); err != nil {
			t.Fatal(err)
		}
	}
	fail = false
	if err := applyUninstall(opts, plan, command); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{opts.SystemdUnitPath, program} {
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			t.Fatalf("file should be gone: %s (%v)", filename, err)
		}
	}
}

func TestUninstallRefusesUnrelatedPanelUnit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("standard Linux installer paths")
	}
	opts := DefaultOptions()
	opts.SystemdUnitPath = filepath.Join(t.TempDir(), opts.ServiceName)
	opts.InstallDir = t.TempDir()
	if err := os.WriteFile(opts.SystemdUnitPath, []byte("[Service]\nExecStart=/bin/sleep infinity\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := runUninstall(opts, false, strings.NewReader("UNINSTALL\n"), io.Discard, func(_ string, args ...string) (string, error) {
		t.Fatalf("must not manipulate unknown service: %v", args)
		return "", nil
	})
	if err == nil || !strings.Contains(err.Error(), "unrelated service unit") {
		t.Fatalf("expected unrelated unit refusal, got %v", err)
	}
}

func TestUninstallPlansOnlyManagedXrayAndTaggedNetwork(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux paths and firewall rules")
	}
	root := t.TempDir()
	opts := DefaultOptions()
	opts.InstallDir = filepath.Join(root, "usr/local/h-ui")
	opts.DataDir = opts.InstallDir
	opts.StateDir = filepath.Join(root, "var/lib/h-ui")
	opts.SystemdUnitPath = filepath.Join(root, "etc/systemd/system/h-ui.service")
	xrayDir := filepath.Join(opts.StateDir, "xray")
	xrayUnit := filepath.Join(filepath.Dir(opts.SystemdUnitPath), constant.XraySystemdUnitName)
	for _, path := range []string{
		filepath.Join(opts.InstallDir, "bin"), xrayDir, filepath.Dir(xrayUnit),
		filepath.Join(root, "var/lib/hui"),
	} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"hy2_access.log", "hy2_access-2026-01-01T00-00-00.000.log.gz", "unrelated.log"} {
		if err := os.WriteFile(filepath.Join(root, "var/lib/hui", name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected := constant.ManagedXraySystemdUnit(filepath.Join(xrayDir, "current"), filepath.Join(xrayDir, "config.json"), xrayDir)
	if err := os.WriteFile(xrayUnit, []byte(expected), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(name string, args ...string) (string, error) {
		switch name {
		case "systemctl":
			unit := args[len(args)-1]
			if unit == constant.XraySystemdUnitName {
				return "LoadState=loaded\nNeedDaemonReload=no\nFragmentPath=" + xrayUnit + "\nActiveState=active\n", nil
			}
			return "LoadState=not-found\nActiveState=inactive\n", nil
		case "getent":
			if args[0] == "group" {
				return "h-ui-xray:x:993:\n", nil
			}
			return "h-ui-xray:x:993:993::" + xrayDir + ":/usr/sbin/nologin\n", nil
		case "nft":
			return "table inet other\ntable inet hui_porthopping\n", nil
		case "iptables-save":
			return "*nat\n-A PREROUTING -m comment --comment hui_hysteria_porthopping -j REDIRECT --to-ports 443\n" +
				"-A PREROUTING -m comment --comment unrelated -j ACCEPT\nCOMMIT\n*filter\n-A PREROUTING -m comment --comment hui_hysteria_porthopping -j ACCEPT\nCOMMIT\n", nil
		case "ip6tables-save":
			return "", nil
		default:
			t.Fatalf("unexpected command: %s %v", name, args)
			return "", nil
		}
	}
	plan, err := inspectUninstall(opts, run)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.services) != 1 || plan.services[0].path != xrayUnit || !slices.Contains(plan.files, xrayDir) {
		t.Fatalf("managed Xray missing: %+v", plan)
	}
	if slices.Contains(plan.dirs, filepath.Join(root, "var/lib/hui")) || slices.Contains(plan.files, filepath.Join(root, "var/lib/hui/unrelated.log")) {
		t.Fatal("shared log file was scheduled for deletion")
	}
	if len(plan.commands) != 4 || strings.Join(plan.commands[3], " ") != "iptables -w -t nat -D PREROUTING -m comment --comment hui_hysteria_porthopping -j REDIRECT --to-ports 443" {
		t.Fatalf("unexpected cleanup: %v", plan.commands)
	}
	if err := os.WriteFile(xrayUnit, []byte("[Service]\nExecStart=/bin/sleep infinity\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err = inspectUninstall(opts, run)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.services) != 0 || slices.Contains(plan.files, xrayDir) || len(plan.preserved) == 0 {
		t.Fatalf("independent Xray not protected: %+v", plan)
	}
}
