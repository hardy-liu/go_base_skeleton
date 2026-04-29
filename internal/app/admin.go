package app

import (
	"github.com/gin-gonic/gin"

	"go_base_skeleton/internal/handler/admin"
	"go_base_skeleton/internal/middleware"
	"go_base_skeleton/internal/pkg/validate"
	"go_base_skeleton/internal/router"
)

// RunAdmin 组装管理后台 API 的 Gin 引擎并启动 HTTP 服务。
// 全局中间件顺序：Trace → Recovery → AccessLog → CORS（gin-contrib/cors）→ RateLimit（JWT/Timeout 仅在路由组上，见 router.RegisterAdmin）。
func RunAdmin(app *App) error {
	if app.Config.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	validate.Init()

	engine := gin.New()

	engine.Use(
		middleware.Trace(),
		middleware.RequestContext(),
		middleware.Recovery(),
		middleware.AccessLog(),
		middleware.CORS(),
		middleware.RateLimit(app.Config.RateLimit),
	)

	handler := admin.NewHandler(app.DB, app.Redis)

	router.RegisterAdmin(engine, handler, app.Config.JWT)

	return serve(engine, app.Config.Server.Admin.Addr(), app.Config.Server.Admin, app.Logger)
}
