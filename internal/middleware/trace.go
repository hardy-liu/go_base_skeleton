package middleware

import (
	"github.com/gin-gonic/gin"

	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/pkg/trace"
)

// Trace 为每个请求注入链路追踪 ID：若请求头已带 X-Trace-Id 则沿用，否则生成新 UUID。
// ID 会写入 Request.Context（供 logger、GORM 等使用）并回写响应头，便于客户端排查。
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(trace.HeaderKey)
		if len(traceID) > 128 {
			// 记录长度过长的 traceID，并丢弃使用该值
			logger.WithCtx(c.Request.Context()).Warn(trace.HeaderKey + " header too long, ignore custom trace id - " + traceID)
			traceID = ""
		}
		if traceID == "" {
			traceID = trace.New()
		}

		ctx := trace.WithCtx(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)
		c.Header(trace.HeaderKey, traceID)

		c.Next()
	}
}
