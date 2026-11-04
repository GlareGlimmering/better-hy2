package cmd

import (
	"errors"
	"net"
	"testing"

	"h-ui/model/constant"
)

func TestIncompatibleHysteria2DoesNotAbortPanelInitialization(t *testing.T) {
	original := initializeHysteria2
	t.Cleanup(func() { initializeHysteria2 = original })
	called := false
	initializeHysteria2 = func() error {
		called = true
		return errors.New("Hysteria2 v2.13.0 does not support managed userOutbounds; required v2.13.0-hui.2")
	}
	initializeManagedHysteria2()
	if !called {
		t.Fatal("Hysteria2 initialization was not attempted")
	}
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
