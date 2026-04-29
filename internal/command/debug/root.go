package debug

import (
	"go_base_skeleton/internal/command"

	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "debug子命令用于执行调试相关的脚本",
}

func init() {
	command.AddCommand(debugCmd)
}
