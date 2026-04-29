package errcode

import "fmt"

// AppError 同时携带对外 HTTP 状态、稳定业务错误码与可读 message，便于中间件与 handler 统一输出。
type AppError struct {
	HTTPStatus int    `json:"-"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
}

// New 构造 AppError，供包级变量与业务代码复用。
func New(httpStatus, code int, message string) *AppError {
	return &AppError{HTTPStatus: httpStatus, Code: code, Message: message}
}

// Error 实现 error 接口，便于与 fmt、errors 等协作；panic 恢复时也可被识别。
func (e *AppError) Error() string {
	return fmt.Sprintf("code=%d, message=%s", e.Code, e.Message)
}

// WithMsg 浅拷贝并仅替换 message，HTTP 状态与业务 code 不变。
func (e *AppError) WithMsg(msg string) *AppError {
	return &AppError{HTTPStatus: e.HTTPStatus, Code: e.Code, Message: msg}
}

// WithMsgf 同 WithMsg，message 由 fmt.Sprintf 格式化生成。
func (e *AppError) WithMsgf(format string, args ...any) *AppError {
	return &AppError{HTTPStatus: e.HTTPStatus, Code: e.Code, Message: fmt.Sprintf(format, args...)}
}
