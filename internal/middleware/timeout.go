package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout 用 context.WithTimeout 为本次请求设置截止时间，defer cancel 避免泄漏。
// 下游 DB/HTTP 等若支持 ctx，可在超时后取消；handler 结束即取消 context。
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
