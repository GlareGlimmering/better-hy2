package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

func TestGeneratedSubscriptionCredentialImportsIntoLegacyHomeProxy(t *testing.T) {
	setupControllerSubscriptionTestDatabase(t)
	t.Setenv("HUI_NODE_ADDRESS", "93.184.216.34")
	gin.SetMode(gin.TestMode)
	const username = "HP+legacy-1._"
	legacyUserinfo := regexp.MustCompile("^[-A-Za-z0-9+_.]+$")
	request := func(method, path, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		handler(c)
		return w
	}
	created := request("POST", "/", fmt.Sprintf(
		`{"username":%q,"quota":-1,"expireTime":4102444800000,"deviceNo":3,"deleted":0}`, username,
	), SaveAccount)
	if !strings.Contains(created.Body.String(), `"type":"ok"`) {
		t.Fatal("creating the account with a generated password failed")
	}
	verifySubscription := func() string {
		t.Helper()
		account, err := dao.GetAccount("username = ?", username)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/hui/subscription", nil)
		c.Request.Header.Set("User-Agent", "Wget/1.21 (HomeProxy, like v2rayN)")
		c.Params = []gin.Param{{Key: "conPass", Value: url.QueryEscape(*account.ConPass)}}
		Hysteria2Subscribe(c)
		body, err := base64.StdEncoding.DecodeString(w.Body.String())
		if w.Code != 200 || err != nil {
			t.Fatal("expected the real HomeProxy user agent to receive a Base64 subscription")
		}
		parsed, err := url.Parse(string(body))
		if err != nil || parsed.Scheme != "hysteria2" || parsed.User == nil || parsed.Hostname() != "93.184.216.34" {
			t.Fatal("expected a complete Hysteria2 URI with authentication")
		}
		rawUserinfo, _, hasAuth := strings.Cut(strings.TrimPrefix(string(body), "hysteria2://"), "@")
		if !hasAuth || !legacyUserinfo.MatchString(rawUserinfo) || !strings.HasPrefix(rawUserinfo, username+".") {
			t.Fatal("legacy HomeProxy would reject the full authentication segment")
		}
		// LuCI's urldecode leaves literal '+' intact when called without decode_plus.
		imported, err := url.PathUnescape(rawUserinfo)
		if err != nil || imported != *account.ConPass || len(strings.TrimPrefix(imported, username+".")) != 24 {
			t.Fatal("imported credential does not match the saved account credential")
		}
		authenticated, found, err := dao.FindHysteria2AuthAccount(imported)
		if err != nil || !found || *authenticated.Id != *account.Id {
			t.Fatal("imported credential did not resolve to the expected account in HY2 auth")
		}
		return imported
	}
	legacy, err := dao.GetAccount("username = ?", "controller-subscription-test")
	if err != nil {
		t.Fatal(err)
	}
	details := request("GET", fmt.Sprintf("/?id=%d", *legacy.Id), "", GetAccount)
	if !strings.Contains(details.Body.String(), `"homeProxyCredentialIncompatible":true`) ||
		strings.Contains(details.Body.String(), "conPass") {
		t.Fatal("account details must report legacy incompatibility without exposing the credential")
	}
	initial := verifySubscription()
	account, err := dao.GetAccount("username = ?", username)
	if err != nil {
		t.Fatal(err)
	}
	w := request("GET", "/", "", GenerateConnectionPassword)
	var response struct {
		Data string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || len(response.Data) != 24 {
		t.Fatal("password regeneration did not return a 24-character password")
	}
	updated := request("POST", "/", fmt.Sprintf(`{"id":%d,"conPass":%q}`, *account.Id, response.Data), UpdateAccount)
	if !strings.Contains(updated.Body.String(), `"type":"ok"`) {
		t.Fatal("saving the regenerated password failed")
	}
	if verifySubscription() == initial {
		t.Fatal("regenerating the password did not rotate the credential")
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
