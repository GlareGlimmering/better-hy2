package controller

import (
	"encoding/base64"
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
)

func TestHysteria2SubscribeBase64IgnoresTunnelHost(t *testing.T) {
	connectionPassword := setupControllerSubscriptionTestDatabase(t)
	t.Setenv("HUI_NODE_ADDRESS", "93.184.216.34")

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = []gin.Param{{Key: "conPass", Value: url.QueryEscape(connectionPassword)}}
	context.Request = httptest.NewRequest("GET", "/hui/subscription", nil)
	context.Request.Host = "tunnel.example.test"
	context.Request.Header.Set("User-Agent", "v2rayN")

	Hysteria2Subscribe(context)

	if recorder.Code != 200 {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	decoded, err := base64.StdEncoding.DecodeString(recorder.Body.String())
	if err != nil {
		t.Fatalf("decode Base64 subscription: %v", err)
	}
	parsed, err := url.Parse(string(decoded))
	if err != nil {
		t.Fatalf("parse decoded Hysteria2 URI: %v", err)
	}
	if parsed.Hostname() != "93.184.216.34" {
		t.Errorf("decoded URI hostname = %q, want node IPv4", parsed.Hostname())
	}
	if parsed.User.Username() != connectionPassword {
		t.Errorf("decoded URI authentication = %q, want %q", parsed.User.Username(), connectionPassword)
	}
	if parsed.Query().Get("sni") != "tls.example.test" {
		t.Errorf("decoded URI sni = %q, want original TLS SNI", parsed.Query().Get("sni"))
	}
	if strings.Contains(string(decoded), context.Request.Host) {
		t.Errorf("decoded node URI unexpectedly contains tunnel host %q", context.Request.Host)
	}
}

func setupControllerSubscriptionTestDatabase(t *testing.T) string {
	t.Helper()
	dataDirectory := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDirectory, constant.SqliteDBDir), 0o755); err != nil {
		t.Fatalf("create test data directory: %v", err)
	}
	t.Setenv("HUI_DATA", dataDirectory+string(os.PathSeparator))
	if err := dao.InitSql(""); err != nil {
		t.Fatalf("initialize test database: %v", err)
	}
	t.Cleanup(func() {
		if err := dao.CloseSqliteDB(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	serverConfig := "listen: :443\nacme:\n  domains:\n    - tls.example.test\ntrafficStats:\n  listen: 127.0.0.1:9999\n"
	if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{"value": serverConfig}); err != nil {
		t.Fatalf("store Hysteria2 config: %v", err)
	}

	connectionPassword := "pa@ss:#/?&%+ ü"
	username := "controller-subscription-test"
	password := "unused"
	quota := int64(1000)
	download := int64(20)
	upload := int64(10)
	expireTime := int64(4102444800000)
	kickUntil := int64(0)
	deviceNumber := int64(3)
	role := "user"
	deleted := int64(0)
	if _, err := dao.SaveAccount(entity.Account{
		Username:     &username,
		Pass:         &password,
		ConPass:      &connectionPassword,
		Quota:        &quota,
		Download:     &download,
		Upload:       &upload,
		ExpireTime:   &expireTime,
		KickUtilTime: &kickUntil,
		DeviceNo:     &deviceNumber,
		Role:         &role,
		Deleted:      &deleted,
	}); err != nil {
		t.Fatalf("create test account: %v", err)
	}
	return connectionPassword
}
