package controller

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"h-ui/dao"
)

func TestSaveRegularAccountWithoutLoginPassword(t *testing.T) {
	_ = setupControllerSubscriptionTestDatabase(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("POST", "/account/saveAccount", strings.NewReader(`{
		"username":"new-user",
		"conPass":"connection-secret",
		"quota":-1,
		"expireTime":4102444800000,
		"deviceNo":3,
		"deleted":0,
		"remark":"no login password"
	}`))
	context.Request.Header.Set("Content-Type", "application/json")

	SaveAccount(context)
	if !strings.Contains(recorder.Body.String(), `"type":"ok"`) {
		t.Fatalf("SaveAccount() response = %s", recorder.Body.String())
	}
	account, err := dao.GetAccount("username = ?", "new-user")
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if account.Pass == nil || *account.Pass != "" {
		t.Fatalf("regular account login password pointer = %#v", account.Pass)
	}
	if got := *account.ConPass; got != "new-user.connection-secret" {
		t.Fatalf("connection password = %q", got)
	}
}

func TestShortProxyUsernameCreateAndEdit(t *testing.T) {
	setupControllerSubscriptionTestDatabase(t)
	gin.SetMode(gin.TestMode)
	request := func(handler gin.HandlerFunc, body string, success bool) {
		t.Helper()
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		handler(c)
		if strings.Contains(response.Body.String(), `"type":"ok"`) != success {
			t.Fatalf("unexpected account response: %s", response.Body.String())
		}
	}
	create := `{"username":%q,"quota":1073741824,"expireTime":4102444800000,"deviceNo":3,"deleted":0}`
	request(SaveAccount, fmt.Sprintf(create, "a"), false)
	request(SaveAccount, fmt.Sprintf(create, "ab"), true)
	account, err := dao.GetAccount("username = ?", "ab")
	if err != nil {
		t.Fatal(err)
	}
	id := *account.Id
	originalCredential := *account.ConPass
	request(UpdateAccount, fmt.Sprintf(`{"id":%d,"username":"a"}`, id), false)
	request(UpdateAccount, fmt.Sprintf(`{"id":%d,"username":"ab","remark":"edited","conPass":""}`, id), true)
	account, err = dao.GetAccount("id = ?", id)
	if err != nil || *account.Remark != "edited" || *account.Quota != 1073741824 || *account.ConPass != originalCredential {
		t.Fatalf("remark update must preserve legacy quota and credential: %v", err)
	}
}

func TestGetConnectionPasswordReturnsOnlyRequestedCredentialWithoutCaching(t *testing.T) {
	credential := setupControllerSubscriptionTestDatabase(t)
	account, err := dao.GetAccount("con_pass = ?", credential)
	if err != nil {
		t.Fatal(err)
	}
	get := func(id int64) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		context.Request = httptest.NewRequest("GET", fmt.Sprintf("/?id=%d", id), nil)
		GetConnectionPassword(context)
		return response
	}
	response := get(*account.Id)
	var body struct {
		Data string
	}
	if response.Header().Get("Cache-Control") != "no-store" ||
		json.Unmarshal(response.Body.Bytes(), &body) != nil || body.Data != credential {
		t.Fatal("credential reveal did not return the stored value with no-store")
	}
	if response := get(*account.Id + 1000); response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Body.String(), `"type":"ok"`) {
		t.Fatal("credential reveal succeeded for a nonexistent account")
	}
}
