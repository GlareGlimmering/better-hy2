package controller

import (
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
