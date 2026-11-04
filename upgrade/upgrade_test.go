package upgrade

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	oldVersion    = "v0.0.25-custom.8-rc.8"
	newVersion    = "v0.0.25-custom.8-rc.9"
	stableVersion = "v0.0.25-custom.8"
)

const oldSystemdUnit = `[Service]
ExecStart=/usr/local/h-ui/h-ui -p 8081
`

type upgradeFixture struct {
	opts      Options
	deps      Dependencies
	server    *httptest.Server
	commands  []string
	startCall int
}

func newUpgradeFixture(t *testing.T, healthURL string) *upgradeFixture {
	t.Helper()
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "h-ui"), []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	candidate := []byte("new-binary")
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(candidate), BinaryAsset)
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	mux.HandleFunc("/binary", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(candidate) })
	mux.HandleFunc("/sums", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(checksum)) })
	mux.HandleFunc("/repos/"+DefaultRepository+"/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		writeRelease(t, w, server.URL, stableVersion, false)
	})
	mux.HandleFunc("/repos/"+DefaultRepository+"/releases/tags/"+newVersion, func(w http.ResponseWriter, _ *http.Request) {
		writeRelease(t, w, server.URL, newVersion, true)
	})
	mux.HandleFunc("/repos/"+DefaultRepository+"/releases", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[{"tag_name":%q,"prerelease":true,"assets":[{"name":%q,"browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}]`, newVersion, BinaryAsset, server.URL+"/binary", ChecksumAsset, server.URL+"/sums")
	})

	fixture := &upgradeFixture{server: server}
	fixture.opts = DefaultOptions()
	fixture.opts.APIBase = server.URL
	fixture.opts.InstallDir = installDir
	fixture.opts.DataDir = installDir
	fixture.opts.StateDir = stateDir
	fixture.opts.SystemdUnitPath = filepath.Join(root, "h-ui.service")
	if err := os.WriteFile(fixture.opts.SystemdUnitPath, []byte(oldSystemdUnit), 0644); err != nil {
		t.Fatal(err)
	}
	fixture.opts.CurrentVersion = oldVersion
	fixture.opts.Prerelease = true
	fixture.opts.HealthURL = healthURL
	fixture.opts.Yes = true
	fixture.opts.Output = &strings.Builder{}
	fixture.deps = Dependencies{
		GOOS: "linux", GOARCH: "amd64", EUID: 0,
		HTTPClient: server.Client(),
		BackupDatabase: func(source, destination string) error {
			return copyFile(source, destination, 0600)
		},
		Now:   func() time.Time { return time.Date(2026, 11, 3, 1, 2, 3, 4, time.UTC) },
		Sleep: func(time.Duration) {},
	}
	fixture.deps.Command = func(name string, args ...string) ([]byte, error) {
		fixture.commands = append(fixture.commands, strings.Join(append([]string{name}, args...), " "))
		if name != "systemctl" {
			info, err := os.Stat(name)
			if err != nil {
				return nil, err
			}
			if info.Mode().Perm()&0111 == 0 {
				return nil, errors.New("candidate is not executable")
			}
			return []byte("h-ui version " + newVersion), nil
		}
		switch args[0] {
		case "is-active":
			return []byte("active\n"), nil
		case "start":
			fixture.startCall++
		}
		return nil, nil
	}
	t.Cleanup(server.Close)
	return fixture
}

func writeRelease(t *testing.T, w http.ResponseWriter, baseURL, version string, prerelease bool) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(releaseInfo{
		TagName: version, Prerelease: prerelease,
		Assets: []releaseAsset{{Name: BinaryAsset, URL: baseURL + "/binary"}, {Name: ChecksumAsset, URL: baseURL + "/sums"}},
	}); err != nil {
		t.Errorf("encode release: %v", err)
	}
}

func healthyPanel(t *testing.T, status func() int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status())
		_, _ = w.Write([]byte(`<html><div id="app"></div><script src="/assets/app.js"></script></html>`))
	})
	mux.HandleFunc("/assets/app.js", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestResolveReleaseChannelsAndSpecifiedVersion(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	fixture := newUpgradeFixture(t, health.URL)

	fixture.opts.Prerelease = false
	stable, err := resolveRelease(context.Background(), fixture.opts, fixture.deps)
	if err != nil || stable.TagName != stableVersion || stable.Prerelease {
		t.Fatalf("stable release = %#v, %v", stable, err)
	}
	fixture.opts.Prerelease = true
	prerelease, err := resolveRelease(context.Background(), fixture.opts, fixture.deps)
	if err != nil || prerelease.TagName != newVersion || !prerelease.Prerelease {
		t.Fatalf("prerelease = %#v, %v", prerelease, err)
	}
	fixture.opts.Version = newVersion
	specified, err := resolveRelease(context.Background(), fixture.opts, fixture.deps)
	if err != nil || specified.TagName != newVersion {
		t.Fatalf("specified release = %#v, %v", specified, err)
	}
}

func TestPrereleaseResolverUsesNumericRCOrdering(t *testing.T) {
	versions := []string{
		"v0.0.25-custom.8-rc.9",
		"v0.0.25-custom.8-rc.10",
		"v0.0.25-custom.8-rc.11",
		"v0.0.25-custom.8-rc.12",
		"v0.0.25-custom.8-rc.13",
		"v0.0.25-custom.8-rc.14",
	}
	for index := 1; index < len(versions); index++ {
		if compareReleaseTags(versions[index-1], versions[index]) >= 0 {
			t.Fatalf("expected %s < %s", versions[index-1], versions[index])
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		releases := make([]releaseInfo, 0, len(versions))
		for _, version := range versions {
			releases = append(releases, releaseInfo{TagName: version, Prerelease: true})
		}
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer server.Close()
	opts := DefaultOptions()
	opts.APIBase = server.URL
	opts.Prerelease = true
	release, err := resolveRelease(context.Background(), opts, Dependencies{HTTPClient: server.Client()})
	if err != nil || release.TagName != versions[len(versions)-1] {
		t.Fatalf("latest prerelease=%q err=%v", release.TagName, err)
	}
}

func TestAutomaticPrereleaseNeverDowngrades(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	fixture := newUpgradeFixture(t, health.URL)
	fixture.opts.CurrentVersion = "v0.0.25-custom.8-rc.14"
	err := Run(context.Background(), fixture.opts, fixture.deps)
	if err == nil || !strings.Contains(err.Error(), "refusing automatic downgrade") {
		t.Fatalf("automatic downgrade was not rejected: %v", err)
	}
	if len(fixture.commands) != 0 {
		t.Fatalf("automatic downgrade reached system commands: %v", fixture.commands)
	}
}

func TestAutomaticPrereleaseAlreadyCurrentDoesNothing(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	fixture := newUpgradeFixture(t, health.URL)
	fixture.opts.CurrentVersion = newVersion
	if err := Run(context.Background(), fixture.opts, fixture.deps); err != nil {
		t.Fatalf("already-current check failed: %v", err)
	}
	if !strings.Contains(fixture.opts.Output.(*strings.Builder).String(), "already at "+newVersion) {
		t.Fatalf("missing already-current message: %s", fixture.opts.Output)
	}
	if len(fixture.commands) != 0 {
		t.Fatalf("already-current upgrade reached system commands: %v", fixture.commands)
	}
}

func TestPreflightFailuresNeverStopCurrentService(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*upgradeFixture)
	}{
		{name: "release not found", mutate: func(f *upgradeFixture) { f.opts.APIBase = f.server.URL + "/missing" }},
		{name: "binary download failure", mutate: func(f *upgradeFixture) {
			original := f.deps.HTTPClient.Transport
			f.deps.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasSuffix(req.URL.Path, "/binary") {
					return response(http.StatusBadGateway, "unavailable"), nil
				}
				if original == nil {
					return http.DefaultTransport.RoundTrip(req)
				}
				return original.RoundTrip(req)
			})}
		}},
		{name: "checksum mismatch", mutate: func(f *upgradeFixture) {
			original := f.deps.HTTPClient.Transport
			f.deps.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasSuffix(req.URL.Path, "/sums") {
					return response(http.StatusOK, strings.Repeat("0", 64)+"  "+BinaryAsset+"\n"), nil
				}
				if original == nil {
					return http.DefaultTransport.RoundTrip(req)
				}
				return original.RoundTrip(req)
			})}
		}},
		{name: "version mismatch", mutate: func(f *upgradeFixture) {
			base := f.deps.Command
			f.deps.Command = func(name string, args ...string) ([]byte, error) {
				if name != "systemctl" {
					return []byte("h-ui version " + oldVersion), nil
				}
				return base(name, args...)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			health := healthyPanel(t, func() int { return http.StatusOK })
			fixture := newUpgradeFixture(t, health.URL)
			test.mutate(fixture)
			if err := Run(context.Background(), fixture.opts, fixture.deps); err == nil {
				t.Fatal("expected preflight failure")
			}
			for _, command := range fixture.commands {
				if strings.Contains(command, "systemctl stop") {
					t.Fatalf("preflight failure stopped production service: %s", command)
				}
			}
		})
	}
}

func TestUpgradeMakesDownloadedCandidateExecutableBeforeStopping(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	fixture := newUpgradeFixture(t, health.URL)
	if err := Run(context.Background(), fixture.opts, fixture.deps); err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(fixture.opts.InstallDir, "h-ui"))
	if err != nil || string(content) != "new-binary" {
		t.Fatalf("active binary = %q, %v", content, err)
	}
	if len(fixture.commands) == 0 || strings.Contains(fixture.commands[0], "systemctl") {
		t.Fatalf("candidate was not checked before systemd operations: %v", fixture.commands)
	}
}

func TestUpgradeMigratesSystemdUnitAndHealthPortTo8000(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	fixture := newUpgradeFixture(t, health.URL)
	if err := Run(context.Background(), fixture.opts, fixture.deps); err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}
	unit, err := os.ReadFile(fixture.opts.SystemdUnitPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(unit), "ExecStart="+fixture.opts.InstallDir+"/h-ui -p 8000") || strings.Contains(string(unit), "-p 8081") {
		t.Fatalf("systemd unit was not migrated to 8000:\n%s", unit)
	}
	if !containsCommand(fixture.commands, "systemctl daemon-reload") {
		t.Fatalf("systemd daemon-reload was not called: %v", fixture.commands)
	}
	if endpoint := panelHealthURL(filepath.Join(fixture.opts.DataDir, "data", "missing.db"), DefaultPanelPort); endpoint != "http://127.0.0.1:8000/" {
		t.Fatalf("new-version health endpoint=%q", endpoint)
	}
}

func TestDefaultUpgradeTargetsProductionSystemdUnit(t *testing.T) {
	opts := DefaultOptions()
	if opts.SystemdUnitPath != "/etc/systemd/system/h-ui.service" {
		t.Fatalf("production systemd target=%q", opts.SystemdUnitPath)
	}
}

func TestStandardHealthURLForcesLoopbackHTTP8000(t *testing.T) {
	database := filepath.Join(t.TempDir(), "h_ui.db")
	db, err := sql.Open("sqlite", database)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE config (key TEXT PRIMARY KEY, value TEXT);
		INSERT INTO config VALUES ('H_UI_WEB_PORT', '8081');
		INSERT INTO config VALUES ('H_UI_WEB_CONTEXT', '/panel');
		INSERT INTO config VALUES ('H_UI_CRT_PATH', '/old/cert.pem');
		INSERT INTO config VALUES ('H_UI_KEY_PATH', '/old/key.pem');`); err != nil {
		t.Fatal(err)
	}
	if endpoint := panelHealthURL(database, DefaultPanelPort); endpoint != "http://127.0.0.1:8000/panel/" {
		t.Fatalf("standard health endpoint=%q", endpoint)
	}
}

func TestStoppedServiceRemainsStoppedAfterVerifiedReplacement(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	fixture := newUpgradeFixture(t, health.URL)
	base := fixture.deps.Command
	fixture.deps.Command = func(name string, args ...string) ([]byte, error) {
		if name == "systemctl" && args[0] == "is-active" {
			fixture.commands = append(fixture.commands, strings.Join(append([]string{name}, args...), " "))
			return []byte("inactive\n"), errors.New("exit 3")
		}
		return base(name, args...)
	}
	if err := Run(context.Background(), fixture.opts, fixture.deps); err != nil {
		t.Fatalf("upgrade stopped service: %v", err)
	}
	for _, command := range fixture.commands {
		if strings.Contains(command, "systemctl start") || strings.Contains(command, "systemctl stop") {
			t.Fatalf("stopped service state was changed: %v", fixture.commands)
		}
	}
}

func TestStartFailureRollsBackOldBinaryAndService(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	fixture := newUpgradeFixture(t, health.URL)
	config := filepath.Join(fixture.opts.InstallDir, "bin", "hysteria2.yaml")
	if err := os.MkdirAll(filepath.Dir(config), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("old-config"), 0640); err != nil {
		t.Fatal(err)
	}
	base := fixture.deps.Command
	fixture.deps.Command = func(name string, args ...string) ([]byte, error) {
		if name == "systemctl" && args[0] == "start" && fixture.startCall == 0 {
			fixture.startCall++
			fixture.commands = append(fixture.commands, strings.Join(append([]string{name}, args...), " "))
			if err := os.WriteFile(config, []byte("new-config"), 0640); err != nil {
				return nil, err
			}
			return []byte("new service failed"), errors.New("exit 1")
		}
		return base(name, args...)
	}
	err := Run(context.Background(), fixture.opts, fixture.deps)
	if err == nil || !strings.Contains(err.Error(), "rollback successful") {
		t.Fatalf("expected successful rollback report, got %v", err)
	}
	content, readErr := os.ReadFile(filepath.Join(fixture.opts.InstallDir, "h-ui"))
	if readErr != nil || string(content) != "old-binary" {
		t.Fatalf("old binary was not restored: %q, %v", content, readErr)
	}
	configContent, configErr := os.ReadFile(config)
	if configErr != nil || string(configContent) != "old-config" {
		t.Fatalf("old config was not restored: %q, %v", configContent, configErr)
	}
}

func TestHTTPHealthFailureRollsBackAfterCheckingFrontendAsset(t *testing.T) {
	var requests atomic.Int32
	health := healthyPanel(t, func() int {
		if requests.Add(1) <= panelHealthAttempts {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	fixture := newUpgradeFixture(t, health.URL)
	err := Run(context.Background(), fixture.opts, fixture.deps)
	if err == nil || !strings.Contains(err.Error(), "rollback successful") {
		t.Fatalf("expected health rollback, got %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(fixture.opts.InstallDir, "h-ui"))
	if string(content) != "old-binary" {
		t.Fatalf("old binary was not restored: %q", content)
	}
	unit, unitErr := os.ReadFile(fixture.opts.SystemdUnitPath)
	if unitErr != nil || string(unit) != oldSystemdUnit {
		t.Fatalf("old systemd unit was not restored: %q, %v", unit, unitErr)
	}
}

func TestWaitForPanelURLRetriesConnectionRefusedUntilReady(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/assets/index.js" {
			_, _ = w.Write([]byte("console.log('ready')"))
			return
		}
		_, _ = w.Write([]byte(`<html><div id="app"></div><script type="module" src="/assets/index.js"></script></html>`))
	})}
	started := false
	deps := DefaultDependencies()
	deps.Sleep = func(time.Duration) {
		if started {
			return
		}
		started = true
		readyListener, listenErr := net.Listen("tcp", address)
		if listenErr != nil {
			t.Fatalf("start delayed panel: %v", listenErr)
		}
		go func() { _ = server.Serve(readyListener) }()
	}
	t.Cleanup(func() { _ = server.Close() })

	if err = waitForPanelURL("http://"+address+"/", deps); err != nil {
		t.Fatalf("temporary connection refusal was not retried: %v", err)
	}
	if !started {
		t.Fatal("test did not exercise a connection-refused startup attempt")
	}
}

func TestPanelHealthRequiresFrontendEntryAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><div id="app"></div></html>`))
	}))
	defer server.Close()
	if err := checkPanelHealth(server.URL); err == nil || !strings.Contains(err.Error(), "entry asset is missing") {
		t.Fatalf("missing frontend asset was not rejected: %v", err)
	}
}

func TestRollbackFailureIsReportedAsCritical(t *testing.T) {
	health := healthyPanel(t, func() int { return http.StatusOK })
	fixture := newUpgradeFixture(t, health.URL)
	base := fixture.deps.Command
	fixture.deps.Command = func(name string, args ...string) ([]byte, error) {
		if name == "systemctl" && args[0] == "start" {
			fixture.commands = append(fixture.commands, strings.Join(append([]string{name}, args...), " "))
			return []byte("failed"), errors.New("exit 1")
		}
		return base(name, args...)
	}
	err := Run(context.Background(), fixture.opts, fixture.deps)
	if err == nil || !strings.Contains(err.Error(), "CRITICAL: rollback failed") {
		t.Fatalf("expected critical rollback failure, got %v", err)
	}
}

func TestSQLiteBackupIsConsistent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	destination := filepath.Join(dir, "backup.db")
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE values_table (value TEXT); INSERT INTO values_table VALUES ('preserved')`); err != nil {
		t.Fatal(err)
	}
	if err = backupSQLiteDatabase(source, destination); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	backup, err := sql.Open("sqlite", destination)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var value string
	if err = backup.QueryRow(`SELECT value FROM values_table`).Scan(&value); err != nil || value != "preserved" {
		t.Fatalf("backup value = %q, %v", value, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: ioNopCloser{strings.NewReader(body)}, Header: make(http.Header)}
}

type ioNopCloser struct{ *strings.Reader }

func (ioNopCloser) Close() error { return nil }

func containsCommand(commands []string, expected string) bool {
	for _, command := range commands {
		if command == expected {
			return true
		}
	}
	return false
}
