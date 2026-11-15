package controller

import (
	"database/sql"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"h-ui/dao"
	"h-ui/model/constant"
	"h-ui/service"
	"h-ui/util"
)

func TestACMEDNSValueRequiresCurrentAdminAndOrdinaryConfigIsMasked(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HUI_DATA", root)
	if err := dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	defer dao.CloseSqliteDB()
	const fixture = "fixture-dns-value"
	const yaml = "acme:\n  type: dns\n  dns:\n    name: cloudflare\n    config:\n      token: " + fixture + "\n"
	if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{"value": yaml}); err != nil {
		t.Fatal(err)
	}
	const username, password = "fixture-admin", "fixture-admin-password"
	hash, err := util.HashPanelPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, constant.SqliteDBPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = dao.ResetPanelAdmin(username, hash); err != nil {
		t.Fatal(err)
	}
	token, err := service.Login(username, password)
	if err != nil {
		t.Fatal(err)
	}

	call := func(handler gin.HandlerFunc, bearer, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		c.Request.Header.Set("Authorization", "Bearer "+bearer)
		handler(c)
		return w
	}
	masked := call(GetHysteria2Config, token, "")
	if strings.Contains(masked.Body.String(), fixture) || !strings.Contains(masked.Body.String(), "********") {
		t.Fatal("ordinary configuration did not mask the credential")
	}
	const request = `{"provider":"cloudflare","key":"token"}`
	for _, attempt := range []struct{ bearer, body string }{
		{"", request}, {token, `{"provider":"other","key":"token"}`}, {token, `{"provider":"cloudflare","key":"JWT_SECRET"}`},
	} {
		if w := call(ReadACMEDNSValue, attempt.bearer, attempt.body); strings.Contains(w.Body.String(), fixture) {
			t.Fatal("unauthorized credential read")
		}
	}
	w := call(ReadACMEDNSValue, token, request)
	if !strings.Contains(w.Body.String(), fixture) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("admin response missing value or cache restriction")
	}
	if _, err := db.Exec("DELETE FROM panel_admin WHERE username = ?", username); err != nil {
		t.Fatal(err)
	}
	if w := call(ReadACMEDNSValue, token, request); strings.Contains(w.Body.String(), fixture) {
		t.Fatal("previous administrator session still had access after deletion")
	}
}
