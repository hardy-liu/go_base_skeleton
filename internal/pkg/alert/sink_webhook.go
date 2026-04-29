package alert

import (
	"context"
	"fmt"

	"go_base_skeleton/internal/pkg/httpclient"
)

// WebhookSink 向配置 URL 投递 JSON。
type WebhookSink struct {
	Client httpclient.Client
	URL    string
}

// Name 实现 Sink，供日志等使用。
func (w *WebhookSink) Name() string { return SinkWebhookName }

// payload 与 SPEC 的 JSON 形状一致，便于对端解析。
type webhookPayload struct {
	Level   Level          `json:"level"`
	Title   string         `json:"title"`
	Message string         `json:"message"`
	TraceID string         `json:"trace_id"`
	Extra   map[string]any `json:"extra"`
}

// Send 实现 Sink。
func (w *WebhookSink) Send(ctx context.Context, a Alert) error {
	if w == nil || w.URL == "" {
		return fmt.Errorf("webhook sink misconfigured")
	}
	if w.Client == nil {
		return fmt.Errorf("webhook sink: no http client")
	}
	ex := a.Extra
	if ex == nil {
		ex = map[string]any{}
	}
	body := webhookPayload{
		Level:   a.Level,
		Title:   a.Title,
		Message: a.Message,
		TraceID: a.TraceID,
		Extra:   ex,
	}
	return w.Client.Post(ctx, w.URL, body, &map[string]any{})
}
