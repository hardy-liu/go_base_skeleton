package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"go_base_skeleton/internal/pkg/logger"
)

// AccessLog 在请求结束后记录访问日志：method、完整 path（含 query）、状态码、耗时、客户端 IP、响应体字节数。
// 依赖 Trace 中间件先写入 context，以便日志中带 trace_id。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		c.Next()

		latency := time.Since(start)
		logger.WithCtx(c.Request.Context()).Info("access",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.Int("body_size", c.Writer.Size()),
		)
	}
}
