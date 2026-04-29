package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 处理跨域：无 Origin 头时不添加 CORS 响应头（与历史行为一致）；有 Origin 时回显该源并允许常用方法与头（含 Authorization、X-Trace-Id）。
func CORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return origin != ""
		},
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Content-Type",
			"Authorization",
			"X-Trace-Id",
			"X-Requested-With",
		},
		ExposeHeaders:    []string{"X-Trace-Id"},
		AllowCredentials: true,
		MaxAge:           86400 * time.Second,
	})
}
