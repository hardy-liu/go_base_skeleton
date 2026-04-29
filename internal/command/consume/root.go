package consume

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go_base_skeleton/internal/app"
	"go_base_skeleton/internal/command"
	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/event"
	"go_base_skeleton/internal/pkg/logger"
)

var consumeCmd = &cobra.Command{
	Use:   "consume",
	Short: "consume子命令用于执行事件消费者相关的脚本",
	Long:  "消费 Redis Stream 事件，每个子命令对应一个独立的事件处理器",
}

type consumeRunner func(ctx context.Context, cancel context.CancelFunc, a *app.App, topic, group, consumerName string, args []string) error

func init() {
	command.AddCommand(consumeCmd)
}

// RunConsumer 统一封装 consumer 生命周期管理
func RunConsumer(ctx context.Context, rdb *redis.Client, cfg config.EventConfig,
	topic, group, consumerName string, handler event.HandlerFunc) error {

	log := logger.WithCtx(ctx)
	log.Info("consumer starting",
		zap.String("topic", topic),
		zap.String("group", group),
		zap.String("consumer", consumerName),
	)

	consumer := event.NewConsumer(rdb, cfg, group, consumerName)
	consumer.Register(topic, handler)

	return consumer.Run(ctx)
}

// 构建 consume 命令的 RunE 函数 避免重复代码
func buildConsumeRunE(topic, group, consumerName string, runner consumeRunner) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		logPrefix := command.LogPrefixForCommand(cmd)
		a, err := app.NewCLI(command.ConfigFile(), logPrefix)
		if err != nil {
			return fmt.Errorf("%s: %w", logPrefix, err)
		}
		defer a.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		return runner(ctx, cancel, a, topic, group, consumerName, args)
	}
}
