package main

import (
	"log"

	"go_base_skeleton/internal/app"
)

// main 启动业务 API：配置文件固定为 config/config.yaml，日志子目录与前缀均为 api。
func main() {
	a, err := app.New("config/config.yaml", "api", "api")
	if err != nil {
		log.Fatalf("init app: %v", err)
	}
	defer a.Close()

	if err := app.RunAPI(a); err != nil {
		log.Fatalf("run api: %v", err)
	}
}
