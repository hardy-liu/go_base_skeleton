package debug

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go_base_skeleton/internal/app"
	"go_base_skeleton/internal/command"
	"go_base_skeleton/internal/event"
	"go_base_skeleton/internal/pkg/logger"
)

var publishEventCmd = &cobra.Command{
	Use:   "publish_event",
	Short: "debug publish event",
	RunE:  command.BuildRunE(publishEventCmdRun),
}

func publishEventCmdRun(ctx context.Context, cancel context.CancelFunc, a *app.App, args []string) error {
	log := logger.WithCtx(ctx)
	if err := a.Publisher.Publish(ctx, event.TopicDebug, map[string]any{
		"message": "debug event",
		"time":    time.Now().Unix(),
		// "client_ip":   "1.1.1.1",
		// "time":        12332,
		// "trace_id":    "trace-123",
	}); err != nil {
		log.Error("publish debug event failed", zap.Error(err))
	}

	log.Info("debug publish_event executed", zap.Strings("args", args))
	return nil
}

func init() {
	debugCmd.AddCommand(publishEventCmd)
}
