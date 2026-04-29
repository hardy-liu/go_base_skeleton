package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"go_base_skeleton/internal/config"
	apiHandler "go_base_skeleton/internal/handler/api"
	"go_base_skeleton/internal/middleware"
)

// RegisterAPI 注册业务 API 路由到指定的引擎上。
// /health 无需鉴权；用户相关接口需要 JWT。
func RegisterAPI(e *gin.Engine, h *apiHandler.Handler, jwtCfg config.JWTConfig) {
	e.Use(middleware.Timeout(10 * time.Second))
	{
		e.GET("/health", h.Health)
		if !h.AppCfg.IsProduction() {
			e.Match([]string{"GET", "POST"}, "/debug", h.DebugHeaders)
		}
	}

	// 需要 JWT 鉴权的路由组
	authed := e.Group("/")
	authed.Use(middleware.JWT(jwtCfg), middleware.Timeout(10*time.Second))
	{
		authed.GET("/users/:uid", h.GetUser)
	}
}
