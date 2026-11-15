package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"h-ui/util"
)

func TestPairPreflightRejectsMissingManifestAndIncompatibleCapabilities(t *testing.T) {
	if _, err := preflightReleasePair(context.Background(), releaseInfo{}, "", "", "", Dependencies{}); err == nil {
		t.Fatal("missing pair accepted")
	}
	id := strings.Repeat("a", 40)
	digest := strings.Repeat("b", 64)
	version := os.Getenv("HUI_PAIR_TEST_PANEL_VERSION")
	if version == "" {
		version = "v1.1.0-rc.1"
	}
	p := releasePair{
		Panel:  pairArtifact{version, id, digest},
		Kernel: pairArtifact{"v2.13.0-hui.5", id, digest},
		OS:     "linux", Arch: "amd64", AccountConsistency: 2, AccountBandwidth: 1,
	}
	if err := p.validate(p.Panel.Version); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*releasePair){
		func(p *releasePair) { p.AccountConsistency = 1 },
		func(p *releasePair) { p.AccountBandwidth = 0 },
		func(p *releasePair) { p.Kernel.SHA256 = "" },
		func(p *releasePair) { p.Arch = "arm64" },
		func(p *releasePair) { p.Panel.Version += "-unexpected" },
	} {
		broken := p
		change(&broken)
		if err := broken.validate(p.Panel.Version); err == nil {
			t.Fatal("incompatible pair accepted")
		}
	}
}

func TestPairPreflightCompiledArtifacts(t *testing.T) {
	panel, kernel, p := compiledPairFixture(t)
	manifest, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	payload := readPairFixture(t, kernel)
	var corrupt atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest":
			_, _ = w.Write(manifest)
		case "/kernel":
			if corrupt.Load() {
				_, _ = w.Write([]byte("corrupt candidate"))
				return
			}
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	release := releaseInfo{TagName: p.Panel.Version, Assets: []releaseAsset{
		{Name: pairManifestAsset, URL: srv.URL + "/manifest"}, {Name: pairKernelAsset, URL: srv.URL + "/kernel"},
	}}
	sums := filepath.Join(t.TempDir(), ChecksumAsset)
	content := fmt.Sprintf("%s  %s\n%s  %s\n%x  %s\n", p.Panel.SHA256, BinaryAsset, p.Kernel.SHA256, pairKernelAsset, sha256.Sum256(manifest), pairManifestAsset)
	if err := os.WriteFile(sums, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCandidate(panel, sums, p.Panel.Version, DefaultDependencies()); err != nil {
		t.Fatal(err)
	}
	if _, err := preflightReleasePair(context.Background(), release, panel, sums, t.TempDir(), DefaultDependencies()); err != nil {
		t.Fatal(err)
	}
	corrupt.Store(true)
	if _, err := preflightReleasePair(context.Background(), release, panel, sums, t.TempDir(), DefaultDependencies()); err == nil || !strings.Contains(err.Error(), "SHA256 mismatch") {
		t.Fatalf("corrupt candidate result: %v", err)
	}
}

func compiledPairFixture(t *testing.T) (string, string, releasePair) {
	t.Helper()
	panel, kernel := os.Getenv("HUI_PAIR_TEST_PANEL"), os.Getenv("HUI_PAIR_TEST_KERNEL")
	if panel == "" || kernel == "" {
		t.Skip("requires locally built Linux AMD64 panel and kernel")
	}
	version := os.Getenv("HUI_PAIR_TEST_PANEL_VERSION")
	if version == "" {
		version = "v1.1.0-rc.1"
	}
	return panel, kernel, releasePair{
		Panel:  pairArtifact{version, os.Getenv("HUI_PAIR_TEST_PANEL_COMMIT"), fmt.Sprintf("%x", sha256.Sum256(readPairFixture(t, panel)))},
		Kernel: pairArtifact{"v2.13.0-hui.5-dev", os.Getenv("HUI_PAIR_TEST_KERNEL_COMMIT"), fmt.Sprintf("%x", sha256.Sum256(readPairFixture(t, kernel)))},
		OS:     "linux", Arch: "amd64", AccountConsistency: 2, AccountBandwidth: 1,
	}
}

func readPairFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPairPreflightCompiledArtifactsUpgradesWithMatchingInstalledKernel(t *testing.T) {
	panel, kernel, pair := compiledPairFixture(t)
	manifest, err := json.Marshal(pair)
	if err != nil {
		t.Fatal(err)
	}
	health := healthyPanel(t, func() int { return http.StatusOK })
	f := newUpgradeFixture(t, health.URL)
	f.opts.Version = pair.Panel.Version
	f.opts.Prerelease = false // explicit version works with the default channel
	if usesPairedStableLine(pair.Panel.Version) {
		f.opts.CurrentVersion = "v1.1.0-rc.7"
	}
	installedKernel := filepath.Join(f.opts.DataDir, util.GetHysteria2BinPath())
	if err := os.MkdirAll(filepath.Dir(installedKernel), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedKernel, []byte("different kernel"), 0755); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	f.opts.APIBase = server.URL
	mux.HandleFunc("/repos/"+DefaultRepository+"/releases/tags/"+pair.Panel.Version, func(w http.ResponseWriter, r *http.Request) {
		release := releaseInfo{TagName: pair.Panel.Version, Prerelease: strings.Contains(pair.Panel.Version, "-rc.")}
		for _, asset := range []string{BinaryAsset, ChecksumAsset, pairManifestAsset, pairKernelAsset} {
			release.Assets = append(release.Assets, releaseAsset{Name: asset, URL: server.URL + "/" + asset})
		}
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("/"+BinaryAsset, func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, panel) })
	mux.HandleFunc("/"+pairKernelAsset, func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, kernel) })
	mux.HandleFunc("/"+pairManifestAsset, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(manifest) })
	mux.HandleFunc("/"+ChecksumAsset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  %s\n%s  %s\n%x  %s\n", pair.Panel.SHA256, BinaryAsset, pair.Kernel.SHA256, pairKernelAsset, sha256.Sum256(manifest), pairManifestAsset)
	})
	systemctl := f.deps.Command
	f.deps.Command = func(name string, args ...string) ([]byte, error) {
		if name == "systemctl" {
			return systemctl(name, args...)
		}
		return DefaultDependencies().Command(name, args...)
	}
	binary := filepath.Join(f.opts.InstallDir, "h-ui")
	err = Run(context.Background(), f.opts, f.deps)
	if err == nil || !strings.Contains(err.Error(), "installed HY2 kernel does not match") ||
		len(f.commands) != 0 || string(readPairFixture(t, binary)) != "old-binary" {
		t.Fatalf("mismatch must be rejected before systemd or activation, got %v (%v)", err, f.commands)
	}
	if err := copyFile(kernel, installedKernel, 0755); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(installedKernel)
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), f.opts, f.deps); err != nil {
		t.Fatal(err)
	}
	if err := verifyFileDigest(binary, pair.Panel.SHA256); err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(installedKernel)
	if err != nil || !os.SameFile(original, current) {
		t.Fatalf("installed HY2 kernel was replaced: %v", err)
	}
	if err := verifyFileDigest(installedKernel, pair.Kernel.SHA256); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(f.commands, "\n"), "systemctl start h-ui.service") {
		t.Fatalf("panel was not started: %v", f.commands)
	}
}
