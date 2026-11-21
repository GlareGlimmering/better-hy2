package upgrade

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestTRCPairUpgradeOnDisposableSystemd(t *testing.T) {
	if os.Getenv("HUI_DISPOSABLE_VM") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" ||
		os.Getenv("HUI_TRC_UPGRADER_TEST") == "" || currentEUID() != 0 {
		t.Skip("requires the disposable GitHub Actions systemd fixture")
	}
	pid1, err := os.ReadFile("/proc/1/comm")
	if err != nil || strings.TrimSpace(string(pid1)) != "systemd" {
		t.Fatal("a real systemd instance is required")
	}
	opts := DefaultOptions()
	opts.CurrentVersion = "v2.3.1-trc.1"
	opts.Version = "v2.3.1-trc.2"
	opts.Yes = true
	if err := Run(context.Background(), opts, DefaultDependencies()); err != nil {
		t.Fatal(err)
	}
}
