package middleware

import (
	"crypto/sha256"
	"net"
	"net/http"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gin-gonic/gin"
	"h-ui/model/vo"
)

func newRequestLimiter(rate float64, burst int) *limiter.Limiter {
	return tollbooth.NewLimiter(rate, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Minute}).SetBurst(burst)
}

func RateLimiterHandler() gin.HandlerFunc {
	limit := newRequestLimiter(5, 5)
	return func(c *gin.Context) {
		// Retain separate buckets for distinct URLs without keeping subscription
		// credentials in the limiter's cache keys.
		pathDigest := sha256.Sum256([]byte(c.Request.URL.Path))
		if tollbooth.LimitByKeys(limit, []string{requestClientIP(c), string(pathDigest[:])}) != nil {
			vo.Fail("click too fast", c)
			c.Abort()
			return
		}
		c.Next()
	}
}

// Login and credential changes share a budget, independent of ordinary requests.
func PanelLoginLimiter() gin.HandlerFunc {
	limit := newRequestLimiter(1, 5)
	return func(c *gin.Context) {
		if tollbooth.LimitByKeys(limit, []string{requestClientIP(c)}) != nil {
			c.Header("Cache-Control", "no-store")
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"msg": "Too many login attempts. Try again shortly."})
			return
		}
		c.Next()
	}
}

func requestClientIP(c *gin.Context) string {
	if ip := net.ParseIP(c.ClientIP()); ip != nil {
		return ip.String()
	}
	return "unknown"
}
