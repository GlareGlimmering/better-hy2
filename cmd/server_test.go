package cmd

import (
	"errors"
	"net"
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
		return errors.New("Hysteria2 v2.13.0 does not support managed userOutbounds; required v2.13.0-hui.3")
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
