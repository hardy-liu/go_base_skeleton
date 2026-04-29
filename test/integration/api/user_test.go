//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/handler/api"
	"go_base_skeleton/internal/middleware"
	"go_base_skeleton/internal/router"
	"go_base_skeleton/test/testhelper"
)

// TestGetUser_InvalidUID 验证：访问需 JWT 的 /users/:uid 且未带 Authorization 时，由 JWT 中间件直接返回 401 / 业务码 40100。
// 因 JWT 在 handler 之前，路径中的非数字 uid 不会进入 GetUser 的 ParseInt 分支。
func TestGetUser_InvalidUID(t *testing.T) {
	r := testhelper.NewTestRouter()
	r.Use(middleware.Trace(), middleware.RequestContext(), middleware.Recovery())

	h := api.NewHandler(nil, nil, config.Config{}, nil)
	jwtCfg := config.JWTConfig{Secret: "test-secret", Issuer: "test"}
	router.RegisterAPI(r, h, jwtCfg)

	w := testhelper.PerformRequest(r, http.MethodGet, "/users/abc")
	testhelper.AssertStatus(t, w, http.StatusUnauthorized)
	testhelper.AssertCode(t, w, 40100)
}

// TestHealth_NoInfra：全 nil 构造时 database.Ping(nil) 在 GORM 上触发 nil 解引用 panic，由 Recovery 转为 ErrInternal：HTTP 500、业务码 50000。
func TestHealth_NoInfra(t *testing.T) {
	r := testhelper.NewTestRouter()
	r.Use(middleware.Trace(), middleware.RequestContext(), middleware.Recovery())

	h := api.NewHandler(nil, nil, config.Config{}, nil)
	r.GET("/health", h.Health)

	w := testhelper.PerformRequest(r, http.MethodGet, "/health")
	testhelper.AssertStatus(t, w, http.StatusInternalServerError)
	testhelper.AssertCode(t, w, 50000)
}
