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
)

func TestPairPreflightRejectsMissingManifestAndIncompatibleCapabilities(t *testing.T) {
	if err := preflightReleasePair(context.Background(), releaseInfo{}, "", "", "", Dependencies{}); err == nil {
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
		func(p *releasePair) { p.Panel.Version = "v1.1.0-rc.2" },
	} {
		broken := p
		change(&broken)
		if err := broken.validate(p.Panel.Version); err == nil {
			t.Fatal("incompatible pair accepted")
		}
	}
}

func TestPairPreflightCompiledArtifacts(t *testing.T) {
	panel, kernel := os.Getenv("HUI_PAIR_TEST_PANEL"), os.Getenv("HUI_PAIR_TEST_KERNEL")
	if panel == "" || kernel == "" {
		t.Skip("requires locally built Linux AMD64 panel and kernel")
	}
	deps := DefaultDependencies()
	read := func(path string) []byte {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	panelDigest, kernelDigest := fmt.Sprintf("%x", sha256.Sum256(read(panel))), fmt.Sprintf("%x", sha256.Sum256(read(kernel)))
	version := os.Getenv("HUI_PAIR_TEST_PANEL_VERSION")
	if version == "" {
		version = "v1.1.0-rc.1"
	}
	p := releasePair{
		Panel:  pairArtifact{version, os.Getenv("HUI_PAIR_TEST_PANEL_COMMIT"), panelDigest},
		Kernel: pairArtifact{"v2.13.0-hui.5-dev", os.Getenv("HUI_PAIR_TEST_KERNEL_COMMIT"), kernelDigest},
		OS:     "linux", Arch: "amd64", AccountConsistency: 2, AccountBandwidth: 1,
	}
	manifest, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	payload := read(kernel)
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
	content := fmt.Sprintf("%s  %s\n%s  %s\n%x  %s\n", panelDigest, BinaryAsset, kernelDigest, pairKernelAsset, sha256.Sum256(manifest), pairManifestAsset)
	if err := os.WriteFile(sums, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCandidate(panel, sums, p.Panel.Version, deps); err != nil {
		t.Fatal(err)
	}
	if err := preflightReleasePair(context.Background(), release, panel, sums, t.TempDir(), deps); err != nil {
		t.Fatal(err)
	}
	corrupt.Store(true)
	if err := preflightReleasePair(context.Background(), release, panel, sums, t.TempDir(), deps); err == nil || !strings.Contains(err.Error(), "SHA256 mismatch") {
		t.Fatalf("corrupt candidate result: %v", err)
	}
}
