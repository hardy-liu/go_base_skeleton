package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go_base_skeleton/internal/pkg/errcode"
)

// Body 为统一 JSON 信封：成功时 code=0；失败时为业务错误码，HTTP 状态码由 Fail 使用 AppError.HTTPStatus。
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK 返回 HTTP 200，body 为 { code:0, message:"ok", data }。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// Fail 使用 err 中的 HTTPStatus 作为 HTTP 状态码，body 为 { code, message }，可选地附带 data（一个参数时写入 body.data）。
func Fail(c *gin.Context, err *errcode.AppError, data ...any) {
	b := Body{
		Code:    err.Code,
		Message: err.Message,
	}
	if len(data) > 0 {
		b.Data = data[0]
	}
	c.JSON(err.HTTPStatus, b)
}

// FailWithMsg 与 Fail 相同 HTTP 状态与业务 code，仅将 message 替换为自定义文案。
func FailWithMsg(c *gin.Context, err *errcode.AppError, msg string) {
	c.JSON(err.HTTPStatus, Body{
		Code:    err.Code,
		Message: msg,
	})
}
