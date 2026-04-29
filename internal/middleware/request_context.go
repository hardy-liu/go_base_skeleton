package middleware

import (
	"context"
	"go_base_skeleton/internal/constant"

	"github.com/gin-gonic/gin"
)

// RequestContext 将请求级上下文数据注入到 Request.Context，便于下游统一读取。
func RequestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), constant.ContextKeyClientIP, c.ClientIP())
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
