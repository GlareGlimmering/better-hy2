package controller

import (
	"bytes"
	"crypto/rand"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGeneratedConnectionPassword(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 64; i++ {
		password, err := generateConnectionPassword(rand.Reader)
		if err != nil || len(password) != 24 || seen[password] {
			t.Fatal("invalid or repeated generated password")
		}
		seen[password] = true
		for _, class := range []string{"ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz", "0123456789", "!@#$&*_"} {
			if !strings.ContainsAny(password, class) {
				t.Fatal("missing required character class")
			}
		}
	}
	if password, err := generateConnectionPassword(bytes.NewReader(nil)); err == nil || password != "" {
		t.Fatal("entropy failure must not return a password")
	}
}

func TestGeneratedPasswordAlphabetRoundTripsThroughSubscriptionFormats(t *testing.T) {
	credential := "testuser.Aa1!@#$&*_"
	engine := gin.New()
	engine.GET("/hui/:credential", func(c *gin.Context) {
		got, err := url.QueryUnescape(c.Param("credential"))
		if err != nil || got != credential {
			t.Error("subscription path did not preserve the credential")
		}
		c.Status(204)
	})
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/hui/"+url.QueryEscape(credential), nil))
	if w.Code != 204 {
		t.Fatal("encoded subscription path did not match")
	}
	uri := url.URL{Scheme: "hysteria2", User: url.User(credential), Host: "example.test:443"}
	decoded, err := url.Parse(uri.String())
	if err != nil || decoded.User.Username() != credential {
		t.Fatal("HY2 URI did not preserve the credential")
	}
}
