package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/handler/api"
	"go_base_skeleton/internal/middleware"
	"go_base_skeleton/internal/pkg/validate"
	"go_base_skeleton/internal/router"
)

// RunAPI 组装业务 API 的 Gin 引擎并启动 HTTP 服务。
// 全局中间件顺序：Trace → Recovery → AccessLog → CORS（gin-contrib/cors）→ RateLimit（JWT/Timeout 仅在路由组上，见 router.RegisterAPI）。
func RunAPI(app *App) error {
	if app.Config.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	validate.Init()

	engine := gin.New()

	//中间件本质是一个栈模型，http请求进来的时候执行入栈操作，c.Next() 之前的操作在入栈的时候执行，c.Next() 之后的操作在出栈的时候(http请求返回时)执行
	engine.Use(
		middleware.Trace(),
		middleware.RequestContext(),
		middleware.Recovery(), //Recover必须放在最前面，才能捕获中间件的panic
		middleware.AccessLog(),
		middleware.CORS(),
		middleware.RateLimit(app.Config.RateLimit),
	)

	handler := api.NewHandler(app.DB, app.Redis, *app.Config, app.Publisher)

	router.RegisterAPI(engine, handler, app.Config.JWT)

	return serve(engine, app.Config.Server.API.Addr(), app.Config.Server.API, app.Logger)
}

// serve 在独立 goroutine 中 ListenAndServe，并阻塞等待 SIGINT/SIGTERM。
// 收到退出信号后，在 10s 超时内调用 Shutdown 完成优雅关闭；ListenAndServe 返回的 http.ErrServerClosed 视为正常。
func serve(engine *gin.Engine, addr string, cfg config.ServerEntry, log *zap.Logger) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("shutdown signal received", zap.String("signal", sig.String()))
	case err := <-errCh:
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced shutdown", zap.Error(err))
		return err
	}
	log.Info("server exited gracefully")
	return nil
}
