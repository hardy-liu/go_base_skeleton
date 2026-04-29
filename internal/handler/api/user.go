package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"go_base_skeleton/internal/pkg/errcode"
	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/pkg/response"
)

// GetUser 处理 GET /users/:uid：校验路径参数 uid 为整数，调用 UserSvc 查询并映射 *AppError 到 JSON。
func (h *Handler) GetUser(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.WithCtx(ctx)

	uidStr := c.Param("uid")
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		log.Warn("invalid uid param", zap.String("uid", uidStr))
		response.Fail(c, errcode.ErrParam.WithMsg("uid must be a number"))
		return
	}

	log.Info("handler GetUser", zap.Int64("uid", uid))

	user, svcErr := h.UserSvc.GetByUID(ctx, uid)
	if svcErr != nil {
		if appErr, ok := svcErr.(*errcode.AppError); ok {
			response.Fail(c, appErr)
			return
		}
		response.Fail(c, errcode.ErrInternal)
		return
	}

	response.OK(c, user)
}
