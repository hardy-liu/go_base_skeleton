// 启动第二个消费者组 或 消费者，验证多消费者组和多消费者的场景
package consume

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"go_base_skeleton/internal/app"
	"go_base_skeleton/internal/command"
	"go_base_skeleton/internal/event"
	eventhandler "go_base_skeleton/internal/event/handler"
)

func init() {
	consumeCmd.AddCommand(newDebug2Cmd())
}

func newDebug2Cmd() *cobra.Command {
	// const (	//不同消费者组
	// 	topicName    = event.TopicDebug
	// 	groupName    = "group_debug2"
	// 	consumerName = "consumer_debug2"
	// )
	const ( //同消费者组，不同consumer
		topicName    = event.TopicDebug
		groupName    = "group_debug"
		consumerName = "consumer_debug2"
	)

	return &cobra.Command{
		Use:   "debug2",
		Short: fmt.Sprintf("启动 debug2 事件处理器 - group: %s", groupName),
		Long:  "消费 debug 事件",
		RunE:  buildConsumeRunE(topicName, groupName, consumerName, runDebug2),
	}
}

func runDebug2(ctx context.Context, cancel context.CancelFunc, a *app.App,
	topicName, groupName, consumerName string, args []string) error {
	command.SetupSignalHandler(ctx, cancel, a.Config.Event.ShutdownTimeout)

	handler := eventhandler.ExampleHandler

	return RunConsumer(ctx, a.EventRedis, a.Config.Event,
		topicName, groupName, consumerName, handler)
}
