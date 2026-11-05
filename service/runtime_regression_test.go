package service

import (
	"encoding/json"
	"h-ui/dao"
	"h-ui/model/constant"
	"strings"
	"testing"
)

func TestAuthCallbackAlwaysUsesStandardLoopbackPanel(t *testing.T) {
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

	callback, err := GetAuthHttpUrl()
	if err != nil {
		t.Fatal(err)
	}
	if callback != "http://127.0.0.1:8000/admin/hui/hysteria2/auth" {
		t.Fatalf("auth callback = %q", callback)
	}
	if strings.Contains(callback, "8081") || strings.HasPrefix(callback, "https://") {
		t.Fatalf("auth callback retained stale panel settings: %q", callback)
	}
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
