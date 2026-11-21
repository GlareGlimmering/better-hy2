package controller

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestHysteria2LifecycleAsyncReturnsJobAndStartupError(t *testing.T) {
	const startupError = "DNS-01 TXT is missing on an authoritative server"
	gate := make(chan struct{})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/hysteria2/start?async=1", nil)
	hysteria2LifecycleResponse(ctx, func() error {
		<-gate
		return errors.New(startupError)
	})
	close(gate)
	var envelope struct {
		Data hysteria2ApplyResult
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || envelope.Data.State != "pending" || envelope.Data.ID == "" {
		t.Fatalf("expected a pending operation, response=%s error=%v", recorder.Body.String(), err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		result, ok := configApplyJobs.status(envelope.Data.ID)
		if ok && result.State == "failed" {
			if result.Error != startupError {
				t.Fatalf("unexpected startup error: %q", result.Error)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("startup result did not become available")
}
