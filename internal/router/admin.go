package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"go_base_skeleton/internal/config"
	adminHandler "go_base_skeleton/internal/handler/admin"
	"go_base_skeleton/internal/middleware"
)

// RegisterAdmin 注册 管理后台 路由到指定的引擎上。
// 当前示例仅 /users/:uid，全部需 JWT + 超时（与业务 API 共用 jwtCfg）。
func RegisterAdmin(e *gin.Engine, h *adminHandler.Handler, jwtCfg config.JWTConfig) {
	authed := e.Group("/")
	authed.Use(middleware.JWT(jwtCfg), middleware.Timeout(10*time.Second))
	{
		authed.GET("/users/:uid", h.GetUser)
	}
}
