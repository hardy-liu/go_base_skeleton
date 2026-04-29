package errcode

import "net/http"

// 以下为系统级预定义错误（HTTP 状态与业务 code 配套）。业务层可继续定义 6xxxx 等。
var (
	Success         = New(http.StatusOK, 0, "ok")
	ErrBadRequest   = New(http.StatusBadRequest, 40000, "bad request")
	ErrParam        = New(http.StatusBadRequest, 40001, "invalid parameter")
	ErrUnauthorized = New(http.StatusUnauthorized, 40100, "unauthorized")
	ErrForbidden    = New(http.StatusForbidden, 40300, "forbidden")
	ErrNotFound     = New(http.StatusNotFound, 40400, "not found")
	ErrRateLimit    = New(http.StatusTooManyRequests, 42900, "too many requests")
	ErrInternal     = New(http.StatusInternalServerError, 50000, "internal server error")
	ErrDatabase     = New(http.StatusInternalServerError, 50001, "database error")
	ErrRedis        = New(http.StatusInternalServerError, 50002, "redis error")
	// ErrServiceDegraded 用于健康检查等：下游不可用时 HTTP 503，与 ErrInternal 同为业务码 50000 以便现有约定一致。
	ErrServiceDegraded = New(http.StatusServiceUnavailable, 50000, "service degraded")
)

// 业务级错误码（6xxxx 段示例）。
var (
	ErrUserNotFound = New(http.StatusOK, 60001, "user not found")
)

// 预留业务错误码可按领域继续扩展。
var ()
