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

func TestHysteria2ConfigApplyRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Router(engine, nil)
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(method, "/hui/config/applyHysteria2Config", nil))
		if !strings.Contains(recorder.Body.String(), `"message":"unauthorized"`) {
			t.Fatalf("%s response=%s", method, recorder.Body.String())
		}
	}
}

func TestBindUnassignedAccountOutboundsRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Router(engine, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/hui/account/bindUnassignedOutbounds", nil))
	if !strings.Contains(w.Body.String(), `"message":"unauthorized"`) {
		t.Fatalf("unprotected endpoint: %s", w.Body.String())
	}
}
