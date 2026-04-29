package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"go_base_skeleton/internal/pkg/errcode"
	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/pkg/response"
)

// Recovery 捕获 handler 链中的 panic：若恢复值为 *errcode.AppError（或包装了该类型的 error）则原样映射到 JSON；
// 其他情况记错误栈并返回 ErrInternal。最后通过 response.Fail 写入与 AppError 一致的 HTTP 状态与 body。
// AbortWithStatus(http.StatusOK) 在响应已写出时主要起 Abort 作用，防止后续中间件继续执行。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				ctx := c.Request.Context()
				l := logger.WithCtx(ctx)

				var appErr *errcode.AppError
				switch v := r.(type) {
				case *errcode.AppError:
					appErr = v
				case error:
					if errors.As(v, &appErr) {
						break
					}
					l.Error("panic recovered", zap.Error(v), zap.Stack("stack"))
					appErr = errcode.ErrInternal
				case string:
					l.Error("panic recovered", zap.String("error", v), zap.Stack("stack"))
					appErr = errcode.ErrInternal
				default:
					l.Error("panic recovered", zap.Any("error", v), zap.Stack("stack"))
					appErr = errcode.ErrInternal
				}

				response.Fail(c, appErr)
				c.AbortWithStatus(http.StatusOK) // body already written
			}
		}()
		c.Next()
	}
}
