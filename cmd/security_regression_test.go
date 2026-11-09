package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"h-ui/dao"
	"h-ui/model/constant"
	"h-ui/model/entity"
	"h-ui/service"
	"h-ui/util"
)

func TestPasswordResetRevokesPreviousAdministratorJWT(t *testing.T) {
	resetTestDatabase(t)
	username, password, err := resetAdminCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if err = dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	old, err := service.Login(username, util.SHA224String(password))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ParseToken(old); err != nil {
		t.Fatal(err)
	}
	statsBefore, err := dao.GetConfig("key = ?", constant.Hysteria2TrafficStatsSecret)
	if err != nil {
		t.Fatal(err)
	}
	secondName, secondPass, role := "second-admin-fixture", util.SHA224String("second-password-fixture"), "admin"
	secondID, err := dao.SaveAccount(entity.Account{Username: &secondName, Pass: &secondPass, Role: &role})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Login(secondName, secondPass)
	if err != nil {
		t.Fatal(err)
	}
	if err = dao.CloseSqliteDB(); err != nil {
		t.Fatal(err)
	}
	username, password, err = resetAdminCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if err = dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	defer dao.CloseSqliteDB()
	statsAfter, err := dao.GetConfig("key = ?", constant.Hysteria2TrafficStatsSecret)
	if err != nil || *statsBefore.Value != *statsAfter.Value {
		t.Fatal("password reset changed the Traffic Stats credential")
	}
	if _, err = service.ParseToken(old); err == nil {
		t.Error("previous JWT survived reset and database reopen")
	}
	current, err := service.Login(username, util.SHA224String(password))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ParseToken(current); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ParseToken(second); err != nil {
		t.Fatal("reset revoked another administrator's session")
	}
	for _, state := range []map[string]interface{}{{"deleted": 1}, {"deleted": 0, "role": "user"}} {
		if err := dao.UpdateAccount([]int64{secondID}, state); err != nil {
			t.Fatal(err)
		}
		if _, err := service.ParseToken(second); err == nil {
			t.Error("inactive administrator token accepted")
		}
	}
	if err := dao.DeleteAccount([]int64{secondID}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ParseToken(second); err == nil {
		t.Error("deleted administrator token accepted")
	}
}

func TestProtectHY2ConfigurationRepairsLegacyPermissions(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX file modes")
	}
	path := filepath.Join(t.TempDir(), "hysteria2.yaml")
	for _, file := range []string{path, path + ".candidate"} {
		if err := os.WriteFile(file, []byte("legacy fixture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := protectHY2Configuration(path); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{path, path + ".candidate"} {
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("legacy mode remains %o", info.Mode().Perm())
		}
	}
}

func TestPanelRequestLogsNeverContainSubscriptionCredentials(t *testing.T) {
	resetTestDatabase(t)
	t.Setenv("HUI_NODE_ADDRESS", "93.184.216.34")
	if err := dao.InitSqliteSchema(); err != nil {
		t.Fatal(err)
	}
	defer dao.CloseSqliteDB()
	if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{
		"value": "listen: :443\ntrafficStats:\n  listen: 127.0.0.1:9999\n",
	}); err != nil {
		t.Fatal(err)
	}
	secret := "audit-fixture.subscription"
	if err := dao.UpdateAccount([]int64{1}, map[string]interface{}{"con_pass": secret}); err != nil {
		t.Fatal(err)
	}
	oldWriter, oldErrorWriter, oldMode := gin.DefaultWriter, gin.DefaultErrorWriter, gin.Mode()
	var logs bytes.Buffer
	gin.DefaultWriter, gin.DefaultErrorWriter = &logs, &logs
	gin.SetMode(gin.DebugMode)
	defer func() { gin.DefaultWriter, gin.DefaultErrorWriter = oldWriter, oldErrorWriter; gin.SetMode(oldMode) }()
	r := newPanelRouter(nil)
	r.GET("/test/unauthorized/:credential", func(c *gin.Context) { c.Status(401) })
	r.GET("/test/panic/:credential", func(c *gin.Context) { panic("panic-value-fixture") })
	encoded := url.QueryEscape(secret)
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/hui/" + encoded, 200},
		{"/hui/unknown-password-fixture", 200},
		{"/hui/%25malformed-secret-fixture", 200},
		{"/not-found/" + encoded + "/extra", 404},
		{"/test/unauthorized/" + encoded, 401},
		{"/test/panic/" + encoded, 500},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path+"?token=query-secret-fixture", nil)
		req.Header.Set("User-Agent", "Clash")
		req.Header.Set("Authorization", "Bearer header-secret-fixture")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if tc.path == "/hui/"+encoded && w.Header().Get("Content-Disposition") == "" {
			t.Error("valid subscription did not return an attachment")
		}
		if w.Code != tc.status {
			t.Errorf("expected %d, got %d", tc.status, w.Code)
		}
	}
	for _, bad := range []string{secret, encoded, "unknown-password-fixture", "malformed-secret-fixture", "query-secret-fixture", "header-secret-fixture", "panic-value-fixture"} {
		if strings.Contains(logs.String(), bad) {
			t.Error("request log contained a sensitive fixture")
		}
	}
	if !strings.Contains(logs.String(), "404") || !strings.Contains(logs.String(), "500") {
		t.Error("request status missing from logs")
	}
}
