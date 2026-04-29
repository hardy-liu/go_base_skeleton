package app

import "fmt"

// NewCLI 仅初始化 CLI 命令所需的组件（配置、日志器、数据库、Redis）。
// logPrefix 一般传入 cobra 子命令名，日志写入 log/cli/<命令名>/ 下，便于按命令区分文件。
func NewCLI(configPath, logPrefix string) (*App, error) {
	a, err := New(configPath, "cli", logPrefix)
	if err != nil {
		return nil, fmt.Errorf("init cli app: %w", err)
	}
	return a, nil
}
