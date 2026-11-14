package upgrade

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	plan := uninstallPlan{service: true, files: []string{program}}
	fail := true
	command := func(args ...string) (string, error) {
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
	err := runUninstall(opts, false, strings.NewReader("UNINSTALL\n"), io.Discard, func(args ...string) (string, error) {
		t.Fatalf("must not manipulate unknown service: %v", args)
		return "", nil
	})
	if err == nil || !strings.Contains(err.Error(), "unrelated service unit") {
		t.Fatalf("expected unrelated unit refusal, got %v", err)
	}
}
