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
	consumeCmd.AddCommand(newDebugCmd())
}

func newDebugCmd() *cobra.Command {
	const (
		topicName    = event.TopicDebug
		groupName    = "group_debug"
		consumerName = "consumer_debug"
	)

	return &cobra.Command{
		Use:   "debug",
		Short: fmt.Sprintf("启动 debug 事件处理器 - group: %s", groupName),
		Long:  "消费 debug 事件",
		RunE:  buildConsumeRunE(topicName, groupName, consumerName, runDebug),
	}
}

func runDebug(ctx context.Context, cancel context.CancelFunc, a *app.App,
	topicName, groupName, consumerName string, args []string) error {
	command.SetupSignalHandler(ctx, cancel, a.Config.Event.ShutdownTimeout)

	handler := eventhandler.ExampleHandler

	return RunConsumer(ctx, a.EventRedis, a.Config.Event,
		topicName, groupName, consumerName, handler)
}
