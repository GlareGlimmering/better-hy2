package controller

import (
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
	request(UpdateAccount, fmt.Sprintf(`{"id":%d,"username":"a"}`, id), false)
	request(UpdateAccount, fmt.Sprintf(`{"id":%d,"username":"ab","remark":"edited"}`, id), true)
	account, err = dao.GetAccount("id = ?", id)
	if err != nil || *account.Remark != "edited" || *account.Quota != 1073741824 {
		t.Fatalf("remark update must preserve legacy quota: %v", err)
	}
	request(UpdateAccount, `{"id":1,"username":"xy"}`, false)
}
