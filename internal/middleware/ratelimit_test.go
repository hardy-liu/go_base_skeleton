package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go_base_skeleton/internal/config"
)

func newRateLimitRouter(cfg config.RateLimitConfig) *gin.Engine {
	r := gin.New()
	r.Use(RateLimit(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})
	return r
}

func doGetRL(r *gin.Engine, clientIP string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	if clientIP != "" {
		req.RemoteAddr = clientIP + ":12345"
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimit_Disabled(t *testing.T) {
	r := newRateLimitRouter(config.RateLimitConfig{Enabled: false})

	for i := 0; i < 10; i++ {
		w := doGetRL(r, "1.2.3.4")
		assert.Equal(t, 200, w.Code)
	}
}

func TestRateLimit_AllowUnderLimit(t *testing.T) {
	cfg := config.RateLimitConfig{Enabled: true, Rate: 5, Window: 60 * time.Second, Burst: 5}
	r := newRateLimitRouter(cfg)

	for i := 0; i < 5; i++ {
		w := doGetRL(r, "10.0.0.1")
		assert.Equal(t, 200, w.Code, "request %d should pass", i+1)
	}
}

func TestRateLimit_BlockOverLimit(t *testing.T) {
	cfg := config.RateLimitConfig{Enabled: true, Rate: 3, Window: 60 * time.Second, Burst: 3}
	r := newRateLimitRouter(cfg)

	for i := 0; i < 3; i++ {
		w := doGetRL(r, "10.0.0.2")
		assert.Equal(t, 200, w.Code)
	}

	w := doGetRL(r, "10.0.0.2")
	assert.Equal(t, 429, w.Code)
	require.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestRateLimit_DifferentIPsIndependent(t *testing.T) {
	cfg := config.RateLimitConfig{Enabled: true, Rate: 2, Window: 60 * time.Second, Burst: 2}
	r := newRateLimitRouter(cfg)

	for i := 0; i < 2; i++ {
		w := doGetRL(r, "10.0.0.3")
		assert.Equal(t, 200, w.Code)
	}
	// IP-A 超限
	w := doGetRL(r, "10.0.0.3")
	assert.Equal(t, 429, w.Code)

	// IP-B 不受影响
	w = doGetRL(r, "10.0.0.4")
	assert.Equal(t, 200, w.Code)
}
