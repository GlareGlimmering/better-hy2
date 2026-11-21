package upgrade

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	assets := os.Getenv("HUI_TRC_ASSETS")
	if !filepath.IsAbs(assets) {
		t.Fatal("expected absolute path to verified release fixtures")
	}
	// Serve the exact published assets locally so retries do not depend on
	// an Actions runner's shared anonymous GitHub API rate limit.
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	for _, fixture := range []struct {
		version string
		dir     string
		names   []string
	}{
		{opts.CurrentVersion, filepath.Join(filepath.Dir(assets), opts.CurrentVersion), []string{ChecksumAsset, pairManifestAsset}},
		{opts.Version, assets, []string{ChecksumAsset, pairManifestAsset, BinaryAsset, pairKernelAsset}},
	} {
		release := releaseInfo{TagName: fixture.version, Prerelease: true}
		for _, name := range fixture.names {
			path := filepath.Join(fixture.dir, name)
			route := "/assets/" + fixture.version + "/" + name
			mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, path) })
			release.Assets = append(release.Assets, releaseAsset{Name: name, URL: server.URL + route})
		}
		mux.HandleFunc("/repos/"+DefaultRepository+"/releases/tags/"+fixture.version, func(w http.ResponseWriter, _ *http.Request) {
			if err := json.NewEncoder(w).Encode(release); err != nil {
				t.Error(err)
			}
		})
	}
	opts.APIBase = server.URL
	if err := Run(context.Background(), opts, DefaultDependencies()); err != nil {
		t.Fatal(err)
	}
}
