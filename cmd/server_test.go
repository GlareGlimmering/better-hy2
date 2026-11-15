package cmd

import (
	"errors"
	"fmt"
	"h-ui/dao"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"h-ui/model/constant"
)

func TestIncompatibleHysteria2DoesNotAbortPanelInitialization(t *testing.T) {
	original, originalLaunch := initializeHysteria2, launchBackground
	t.Cleanup(func() { initializeHysteria2, launchBackground = original, originalLaunch })
	called := false
	initializeHysteria2 = func() error {
		called = true
		return errors.New("Hysteria2 v2.13.0 does not support managed userOutbounds; required v2.13.0-hui.4")
	}
	launchBackground = func(operation func()) { operation() }
	initializeManagedHysteria2()
	if !called {
		t.Fatal("Hysteria2 initialization was not attempted")
	}
}

func TestRC16UpgradeCandidateDoesNotWaitForManagedHysteria2Readiness(t *testing.T) {
	original, originalLaunch := initializeHysteria2, launchBackground
	t.Cleanup(func() { initializeHysteria2, launchBackground = original, originalLaunch })
	started, release := make(chan struct{}), make(chan struct{})
	initializeHysteria2 = func() error {
		close(started)
		<-release
		return nil
	}
	launchBackground = func(operation func()) { go operation() }

	returned := make(chan struct{})
	go func() {
		initializeManagedHysteria2()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("candidate startup remained blocked on HY2 readiness")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background HY2 initialization was not started")
	}
	close(release)
}

func TestManagedXrayInitializationInstallsPinnedKernelWithoutStartingRuntime(t *testing.T) {
	originalRequired := managedXrayRequired
	managedXrayRequired = func() (bool, error) { return true, nil }
	t.Cleanup(func() { managedXrayRequired = originalRequired })
	originalInstall, originalLaunch := installManagedXray, launchBackground
	t.Cleanup(func() { installManagedXray, launchBackground = originalInstall, originalLaunch })
	installed := ""
	installManagedXray = func(version string) error { installed = version; return nil }
	launchBackground = func(operation func()) { operation() }
	initializeManagedXray()
	if installed != constant.XrayPinnedVersion {
		t.Fatalf("installed version=%q want %q", installed, constant.XrayPinnedVersion)
	}
}

func TestFreshInstallationDoesNotDownloadXray(t *testing.T) {
	originalInstall, originalLaunch, originalRequired := installManagedXray, launchBackground, managedXrayRequired
	t.Cleanup(func() {
		installManagedXray, launchBackground, managedXrayRequired = originalInstall, originalLaunch, originalRequired
	})
	managedXrayRequired = func() (bool, error) { return false, nil }
	installManagedXray = func(string) error { t.Fatal("fresh installation downloaded Xray"); return nil }
	launchBackground = func(operation func()) { operation() }
	initializeManagedXray()
}

func TestStandardPanelListenAddressIsLoopbackOnly(t *testing.T) {
	if panelBindAddress != constant.PanelBindAddress {
		t.Fatalf("default panel bind=%q", panelBindAddress)
	}
	if address := panelListenAddress(constant.PanelBindAddress, 8000); address != "127.0.0.1:8000" {
		t.Fatalf("standard panel address=%q", address)
	}
	listener, err := net.Listen("tcp", panelListenAddress(constant.PanelBindAddress, 0))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	tcpAddress, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !tcpAddress.IP.IsLoopback() || tcpAddress.IP.IsUnspecified() {
		t.Fatalf("panel listener is not loopback-only: %v", listener.Addr())
	}
}

func TestPanelLoginLimitOnlyTrustsConfiguredProxy(t *testing.T) {
	t.Setenv("HUI_DATA", t.TempDir())
	username, password, err := resetAdminCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if err := dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dao.CloseSqliteDB() })
	r := newPanelRouter(nil)
	if err := configurePanelTrustedProxies(r, "127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	send := func(endpoint, remote, forwarded, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", "/hui/auth/"+endpoint, strings.NewReader(body))
		req.RemoteAddr = remote
		req.Header.Set("X-Forwarded-For", forwarded)
		req.Header.Set("X-Real-IP", "192.0.2.2")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		return res
	}
	const bad = "{\"username\":\"unknown\",\"pass\":\"wrong\"}"
	for i := 0; i < 5; i++ {
		spoofed := fmt.Sprintf("192.0.2.%d", i+10)
		if got := send("login", "198.51.100.10:1000", spoofed, bad); got.Code != http.StatusOK {
			t.Fatalf("unexpected response before consuming login budget: %d", got.Code)
		}
	}
	if got := send("login", "198.51.100.10:1001", "192.0.2.99", bad); got.Code != http.StatusTooManyRequests {
		t.Fatalf("login did not reject a spoofed address after exhausting the budget: %d", got.Code)
	}
	if got := send("changePassword", "198.51.100.10:1001", "192.0.2.100", bad); got.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed header or alternate endpoint bypassed the login limit: %d", got.Code)
	}
	for i := 0; i < 5; i++ {
		spoofed := fmt.Sprintf("192.0.2.%d, 198.51.100.12", i+20)
		send("login", "127.0.0.1:1000", spoofed, bad)
	}
	got := send("changePassword", "127.0.0.1:1000", "192.0.2.120, 198.51.100.12", bad)
	if got.Code != http.StatusTooManyRequests {
		t.Fatalf("forged hop bypassed limit behind trusted proxy: %d", got.Code)
	}
	got = send("login", "127.0.0.1:1000", "198.51.100.11", fmt.Sprintf("{\"username\":%q,\"pass\":%q}", username, password))
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "\"accessToken\"") {
		t.Fatalf("normal login behind trusted proxy failed: %d", got.Code)
	}
}
