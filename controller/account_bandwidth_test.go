package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBandwidthAccountRejectsInvalidJSONRates(t *testing.T) {
	for _, rate := range []string{"-1", "100001", "1.25", `"2"`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("PUT", "/", strings.NewReader(`{"id":7,"uploadMbps":`+rate+`}`))
		c.Request.Header.Set("Content-Type", "application/json")
		UpdateAccount(c)
		if !strings.Contains(w.Body.String(), `"message":"invalid"`) {
			t.Fatalf("invalid rate %s was not rejected: %s", rate, w.Body.String())
		}
	}
}
