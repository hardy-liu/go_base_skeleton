package api

import (
	"github.com/gin-gonic/gin"

	"go_base_skeleton/internal/pkg/database"
	"go_base_skeleton/internal/pkg/errcode"
	"go_base_skeleton/internal/pkg/response"
)

// Health 处理 GET /health：分别 Ping MySQL、业务 Redis 与事件 Redis（Publisher 所用连接），全部成功则 HTTP 200 + 统一信封 data 中标记 up；
// 任一下游失败则 HTTP 503，body 内 code=50000、message=service degraded，data 中分项标记 down。
func (h *Handler) Health(c *gin.Context) {
	ctx := c.Request.Context()
	status := make(map[string]string, 3)
	healthy := true

	if err := database.Ping(h.DB); err != nil {
		status["mysql"] = "down"
		healthy = false
	} else {
		status["mysql"] = "up"
	}

	if err := h.Redis.Ping(ctx).Err(); err != nil {
		status["redis"] = "down"
		healthy = false
	} else {
		status["redis"] = "up"
	}

	if err := h.Publisher.Ping(ctx); err != nil {
		status["event_redis"] = "down"
		healthy = false
	} else {
		status["event_redis"] = "up"
	}

	if !healthy {
		response.Fail(c, errcode.ErrServiceDegraded, status)
		return
	}

	response.OK(c, status)
}
