package debug

import (
	"context"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go_base_skeleton/internal/app"
	"go_base_skeleton/internal/command"
	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/pkg/util"
)

var miscCmd = &cobra.Command{
	Use:   "misc",
	Short: "miscellaneous debugs",
	RunE:  command.BuildRunE(miscCmdRun),
}

func miscCmdRun(ctx context.Context, cancel context.CancelFunc, a *app.App, args []string) error {
	log := logger.WithCtx(ctx)

	// testTruncateUTF8(log)

	log.Info("miscellaneous debug executed", zap.Strings("args", args))
	return nil
}

func testTruncateUTF8(log *zap.Logger) {
	str := "a中文"
	log.Info("outStr " + string(util.TruncateUTF8([]byte(str), 2)))
	log.Info("outStr " + string(util.TruncateUTF8([]byte(str), 4)))
	log.Info("outStr " + string(util.TruncateUTF8([]byte(str), 5)))
	log.Info("outStr " + string(util.TruncateUTF8([]byte(str), 7)))
}

func init() {
	debugCmd.AddCommand(miscCmd)
}
