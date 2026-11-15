package service

import (
	"encoding/json"
	"h-ui/dao"
	"h-ui/model/constant"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAuthCallbackUsesBoundLoopbackIndependentOfPanel(t *testing.T) {
	t.Setenv("HUI_DATA", t.TempDir())
	if err := dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dao.CloseSqliteDB() })
	if err := dao.UpdateConfig([]string{constant.HUIWebPort, constant.HUICrtPath, constant.HUIKeyPath, constant.HUIWebContext}, map[string]interface{}{"value": ""}); err != nil {
		t.Fatal(err)
	}
	if err := dao.UpdateConfig([]string{constant.HUIWebPort}, map[string]interface{}{"value": "8081"}); err != nil {
		t.Fatal(err)
	}
	if err := dao.UpdateConfig([]string{constant.HUIWebContext}, map[string]interface{}{"value": "/admin"}); err != nil {
		t.Fatal(err)
	}

	if _, err := GetAuthHttpUrl(); err == nil {
		t.Fatal("published callback before binding")
	}
	startTestHysteria2AuthListener(t)
	callback, err := GetAuthHttpUrl()
	if err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse(callback)
	if err != nil {
		t.Fatal(err)
	}
	if target.Scheme != "http" || target.Hostname() != "127.0.0.1" || target.Port() == "" || target.Port() == "8081" || target.Path != Hysteria2AuthPath {
		t.Fatalf("callback did not use its own loopback listener: %s", callback)
	}
	conn, err := net.Dial("tcp", target.Host)
	if err != nil {
		t.Fatal("published callback before listening", err)
	}
	conn.Close()
}

func startTestHysteria2AuthListener(t *testing.T) {
	t.Helper()
	stop, err := StartHysteria2AuthListener(http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
}

func TestXrayFreshRuntimeIsExplicitlyNotInstalled(t *testing.T) {
	t.Setenv("HUI_DATA", t.TempDir())
	if err := dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dao.CloseSqliteDB() })
	settings := validXraySettings()
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err = dao.UpdateConfig([]string{constant.XrayDraftConfig}, map[string]interface{}{"value": string(data)}); err != nil {
		t.Fatal(err)
	}

	oldPaths, oldCommand := managedXrayPaths, xrayCommand
	managedXrayPaths = testXrayPaths(t.TempDir())
	commandCalls := 0
	xrayCommand = func(string, ...string) ([]byte, error) {
		commandCalls++
		return nil, assertCommandUnavailable{}
	}
	t.Cleanup(func() { managedXrayPaths, xrayCommand = oldPaths, oldCommand })

	status := GetXrayStatus()
	if status.RuntimeState != "not_installed" || status.RuntimeReady || status.VersionInstalled || status.ServiceInstalled {
		t.Fatalf("unexpected fresh Xray runtime status: %+v", status)
	}
	if !strings.Contains(status.Error, "not installed") {
		t.Fatalf("missing actionable status error: %q", status.Error)
	}
	if commandCalls != 0 {
		t.Fatalf("fresh missing runtime executed a system command %d time(s)", commandCalls)
	}
}

type assertCommandUnavailable struct{}

func (assertCommandUnavailable) Error() string { return "command unavailable in test" }
