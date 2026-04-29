package validate

import (
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Init 在进程启动时获取 gin 内置的 validator 引擎，可在此注册自定义 tag 与中文错误消息等。
// 当前仅占位，未注册额外规则。
func Init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v // register custom validators: v.RegisterValidation("tag", fn)
	}
}
