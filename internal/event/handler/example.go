package eventhandler

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"go_base_skeleton/internal/pkg/logger"
)

// ExampleHandler 演示事件处理器如何处理一个message：打印消息 ID 与字段，始终返回 nil 表示处理成功以便 Consumer ACK。
func ExampleHandler(ctx context.Context, msg redis.XMessage) error {
	log := logger.WithCtx(ctx)
	log.Info("processing example event",
		zap.String("id", msg.ID),
		zap.Any("values", msg.Values),
	)
	time.Sleep(2 * time.Second)
	return nil
}
