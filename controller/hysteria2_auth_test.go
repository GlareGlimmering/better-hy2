package controller

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"h-ui/model/entity"
	"h-ui/service"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHysteria2HTTPAuthProtocolAndSafeDiagnostics(t *testing.T) {
	oldAuth, oldUpdate := authenticateHysteria2, updateHysteria2AuthAccount
	oldOutput := logrus.StandardLogger().Out
	defer func() {
		authenticateHysteria2 = oldAuth
		updateHysteria2AuthAccount = oldUpdate
		logrus.SetOutput(oldOutput)
	}()
	var logs bytes.Buffer
	logrus.SetOutput(&logs)
	for _, tc := range []struct{ name, body, want, reason string }{
		{"numeric zero", `{"addr":"192.0.2.1:1234","auth":"private-test-secret","tx":0}`, `{"ok":true,"id":"TESTsss"}`, "success"},
		{"numeric bandwidth", `{"addr":"192.0.2.1:1234","auth":"private-test-secret","tx":123456}`, `{"ok":true,"id":"TESTsss"}`, "success"},
		{"wrong tx type", `{"addr":"a","auth":"private-test-secret","tx":"0"}`, `{"ok":false,"id":""}`, "request_invalid"},
		{"missing tx", `{"addr":"a","auth":"private-test-secret"}`, `{"ok":false,"id":""}`, "request_invalid"},
		{"bad password", `{"addr":"a","auth":"private-test-secret","tx":0}`, `{"ok":false,"id":""}`, "connection_password_mismatch"},
		{"update failed", `{"addr":"a","auth":"private-test-secret","tx":0}`, `{"ok":false,"id":""}`, "connection_time_update_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			called := false
			authenticateHysteria2 = func(string) (int64, string, error) {
				called = true
				if tc.name == "bad password" {
					return 0, "", &service.Hysteria2AuthError{Reason: tc.reason}
				}
				return 3, "TESTsss", nil
			}
			updateHysteria2AuthAccount = func(entity.Account) error {
				if tc.name == "update failed" {
					return errors.New("private-test-secret")
				}
				return nil
			}
			r := gin.New()
			r.POST("/auth", Hysteria2Auth)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/auth", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			if w.Code != 200 || w.Body.String() != tc.want {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if tc.reason == "request_invalid" && called {
				t.Fatal("invalid JSON reached service")
			}
			if !strings.Contains(logs.String(), tc.reason) || strings.Contains(logs.String(), "private-test-secret") {
				t.Fatalf("unsafe or missing diagnostic: %s", logs.String())
			}
		})
	}
}
