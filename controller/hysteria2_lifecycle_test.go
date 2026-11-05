package controller

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHysteria2LifecycleControllersReportOperationResult(t *testing.T) {
	originalStart, originalStop, originalRestart := startHysteria2, stopHysteria2, restartHysteria2
	t.Cleanup(func() {
		startHysteria2, stopHysteria2, restartHysteria2 = originalStart, originalStop, originalRestart
	})

	called := ""
	startHysteria2 = func() error { called = "start"; return nil }
	stopHysteria2 = func() error { called = "stop"; return nil }
	restartHysteria2 = func() error { called = "restart"; return nil }

	for name, handler := range map[string]gin.HandlerFunc{
		"start":   Hysteria2Start,
		"stop":    Hysteria2Stop,
		"restart": Hysteria2Restart,
	} {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		handler(context)
		if called != name || !strings.Contains(recorder.Body.String(), `"type":"ok"`) {
			t.Fatalf("%s called=%q response=%s", name, called, recorder.Body.String())
		}
	}

	startHysteria2 = func() error { return errors.New("start failed") }
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	Hysteria2Start(context)
	if !strings.Contains(recorder.Body.String(), "start failed") || strings.Contains(recorder.Body.String(), `"type":"ok"`) {
		t.Fatalf("failure response=%s", recorder.Body.String())
	}
}
