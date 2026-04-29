package main

import (
	"log"

	"go_base_skeleton/internal/app"
)

// main 启动管理后台 API，日志写入 log/admin，前缀 admin。
func main() {
	a, err := app.New("config/config.yaml", "admin", "admin")
	if err != nil {
		log.Fatalf("init app: %v", err)
	}
	defer a.Close()

	if err := app.RunAdmin(a); err != nil {
		log.Fatalf("run admin: %v", err)
	}
}
