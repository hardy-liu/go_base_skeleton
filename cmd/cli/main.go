package main

import (
	"go_base_skeleton/internal/command"
	_ "go_base_skeleton/internal/command/consume"
	_ "go_base_skeleton/internal/command/debug"
)

// main 入口委托给 internal/command（cobra），支持 consume、example 等子命令。
func main() {
	command.Execute()
}
