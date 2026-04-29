package debug

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go_base_skeleton/internal/app"
	"go_base_skeleton/internal/command"
	"go_base_skeleton/internal/pkg/logger"
)

var httpclientCmd = &cobra.Command{
	Use:   "httpclient",
	Short: "httpclint debugs",
	RunE:  command.BuildRunE(httpclientCmdRun),
}

func httpclientCmdRun(ctx context.Context, cancel context.CancelFunc, a *app.App, args []string) error {
	log := logger.WithCtx(ctx)

	newCtx, newCancel := context.WithTimeout(ctx, 10*time.Second)
	defer newCancel()

	resp := make(map[string]any)
	url := "http://127.0.0.1:8080/debug"
	query := map[string]string{
		"qa": "qa",
		"qb": "aaa",
	}
	err := a.HTTPClient.Get(newCtx, url, query, &resp)
	if err != nil {
		return err
	}
	log.Info("resp", zap.Any("resp", resp))

	urlPost := "http://127.0.0.1:8080/debug"
	reqBody := map[string]any{
		"name": "test",
		"age":  18,
	}
	respPost := make(map[string]any)
	err = a.HTTPClient.Post(newCtx, urlPost, reqBody, &respPost)
	if err != nil {
		return err
	}
	log.Info("respPost", zap.Any("respPost", resp))

	reqFormData := map[string]string{
		"formname": "test",
		"formage":  "111",
	}
	respPostForm := make(map[string]any)
	err = a.HTTPClient.PostForm(newCtx, urlPost, reqFormData, &respPostForm)
	if err != nil {
		return err
	}
	log.Info("respPostForm", zap.Any("respPostForm", respPostForm))

	return nil
}

func init() {
	debugCmd.AddCommand(httpclientCmd)
}
