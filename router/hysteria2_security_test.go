package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHysteria2LifecycleRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Router(engine, nil)
	for _, path := range []string{
		"/hui/hysteria2/start",
		"/hui/hysteria2/stop",
		"/hui/hysteria2/restart",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		engine.ServeHTTP(recorder, request)
		if !strings.Contains(recorder.Body.String(), `"message":"unauthorized"`) {
			t.Fatalf("%s response=%s", path, recorder.Body.String())
		}
	}
}

func TestAccountOutboundOptionsRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Router(engine, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hui/account/outboundOptions", nil))
	if !strings.Contains(w.Body.String(), `"message":"unauthorized"`) {
		t.Fatalf("unprotected endpoint: %s", w.Body.String())
	}
}
