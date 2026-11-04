package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"h-ui/model/constant"
)

func TestHysteria2ChangeVersionAcceptsCustomVersion(t *testing.T) {
	originalRunning := hysteria2IsRunning
	originalDownload := downloadHysteria2Version
	t.Cleanup(func() {
		hysteria2IsRunning = originalRunning
		downloadHysteria2Version = originalDownload
	})
	hysteria2IsRunning = func() bool { return false }
	called := false
	downloadHysteria2Version = func(version string) error {
		called = true
		if version != constant.Hysteria2CustomVersion {
			t.Fatalf("version = %q", version)
		}
		return nil
	}

	recorder := performHysteria2VersionRequest(t, constant.Hysteria2CustomVersion)
	if !called {
		t.Fatal("custom kernel installer was not called")
	}
	if !strings.Contains(recorder.Body.String(), `"type":"ok"`) {
		t.Fatalf("response = %s", recorder.Body.String())
	}
}

func TestHysteria2ChangeVersionReturnsInstallerError(t *testing.T) {
	originalRunning := hysteria2IsRunning
	originalDownload := downloadHysteria2Version
	t.Cleanup(func() {
		hysteria2IsRunning = originalRunning
		downloadHysteria2Version = originalDownload
	})
	hysteria2IsRunning = func() bool { return false }
	downloadHysteria2Version = func(string) error {
		return errors.New("resolve custom Hysteria2 release: asset missing")
	}

	recorder := performHysteria2VersionRequest(t, constant.Hysteria2CustomVersion)
	body := recorder.Body.String()
	if strings.Contains(body, `"message":"invalid"`) || !strings.Contains(body, "asset missing") {
		t.Fatalf("response = %s", body)
	}
}

func TestListReleaseOffersPinnedCustomKernel(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("GET", "/hysteria2/listRelease", nil)
	ListRelease(context)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), constant.Hysteria2CustomVersion) {
		t.Fatalf("ListRelease() response = %s", recorder.Body.String())
	}
}

func performHysteria2VersionRequest(t *testing.T, version string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("POST", "/hysteria2/hysteria2ChangeVersion", strings.NewReader(`{"version":"`+version+`"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	Hysteria2ChangeVersion(context)
	return recorder
}
