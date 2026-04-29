package command

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go_base_skeleton/internal/app"
	"go_base_skeleton/internal/pkg/logger"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var cfgFile string

// rootCmd 为 cobra 根命令；子命令在各自文件中 init 注册。
var rootCmd = &cobra.Command{
	Use:   "go_base_skeleton-cli",
	Short: "go_base_skeleton CLI tools",
}

type commandRunner func(ctx context.Context, cancel context.CancelFunc, a *app.App, args []string) error

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config/config.yaml", "config file path")
}

// AddCommand 允许子包（如 debug）向根命令注册子命令。
func AddCommand(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}

// Execute 解析并执行命令行；出错时打印到 stderr 并以非零码退出。
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ConfigFile 返回 --config 路径；须在子命令 RunE 中调用（此时标志已解析）。
func ConfigFile() string {
	return cfgFile
}

// LogPrefixForCommand 根据 cobra 子命令推导日志文件名前缀：普通子命令为 cmd.Name()；
// 若祖先链中存在名为 debug 的父命令，则为 debug-<cmd.Name()>（与 debug 命令组 Use 一致）。
func LogPrefixForCommand(cmd *cobra.Command) string {
	for p := cmd.Parent(); p != nil; p = p.Parent() {
		if p.Name() == "debug" {
			return "debug-" + cmd.Name()
		}
		if p.Name() == "consume" {
			return "consume-" + cmd.Name()
		}
	}
	return cmd.Name()
}

// SetupSignalHandler 设置优雅退出信号处理
func SetupSignalHandler(ctx context.Context, cancel context.CancelFunc, timeout time.Duration) {
	go func() {
		log := logger.WithCtx(ctx)

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		defer signal.Stop(quit)

		log.Info("received shutdown signal, draining...",
			zap.Duration("timeout", timeout))
		cancel()
		//走到这里，上层监听ctx.Done()信号后会退出，command.Execute 会正常return，然后退出进程，进程结束code为0-正常

		//继续监听 quit channel，如果 Ctrl+C 再次触发，或者超时，执行 os.Exit(1)，进程结束code为1-异常
		select {
		case <-quit:
			log.Warn("received second signal, forcing exit")
		case <-time.After(timeout):
			log.Warn("shutdown timeout exceeded, forcing exit")
		}
		os.Exit(1)
	}()
}

func BuildRunE(runner commandRunner) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		logPrefix := LogPrefixForCommand(cmd)
		a, err := app.NewCLI(ConfigFile(), logPrefix)
		if err != nil {
			return fmt.Errorf("%s: %w", logPrefix, err)
		}
		defer a.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		return runner(ctx, cancel, a, args)
	}
}
