package command

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go_base_skeleton/internal/app"
	"go_base_skeleton/internal/pkg/logger"
)

func init() {
	rootCmd.AddCommand(exampleCmd)
}

// exampleCmd 演示子命令如何拉起 App（含 DB/Redis）并打一条日志；args 透传仅作示例。
var exampleCmd = &cobra.Command{
	Use:   "example",
	Short: "An example CLI command",
	RunE: func(cmd *cobra.Command, args []string) error {
		logPrefix := LogPrefixForCommand(cmd)
		a, err := app.NewCLI(ConfigFile(), logPrefix)
		if err != nil {
			return fmt.Errorf("%s: %w", logPrefix, err)
		}
		defer a.Close()

		logger.Default().Info("example command executed", zap.Strings("args", args))
		fmt.Println("example command done")
		return nil
	},
}
