package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go_base_skeleton/internal/pkg/httpclient"
	"go_base_skeleton/internal/pkg/util"
)

// TelegramSink 使用 Bot API sendMessage，不在日志中暴露 token（由调用方保证）。
type TelegramSink struct {
	BotToken string
	ChatID   string
	Client   httpclient.Client
}

const maxTelegramText = 3500

// Name 实现 Sink，供日志等使用。
func (t *TelegramSink) Name() string { return SinkTelegramName }

// Send 实现 Sink。
func (t *TelegramSink) Send(ctx context.Context, a Alert) error {
	if t == nil || t.BotToken == "" || t.ChatID == "" {
		return fmt.Errorf("telegram sink misconfigured")
	}
	text := buildTelegramText(a)
	if len(text) > maxTelegramText {
		text = string(util.TruncateUTF8([]byte(text), maxTelegramText)) + "..."
	}
	if t.Client == nil {
		return fmt.Errorf("telegram sink: no http client")
	}
	u := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	var res tgResponse
	if err := t.Client.PostForm(ctx, u, map[string]string{
		"chat_id": t.ChatID,
		"text":    text,
	}, &res); err != nil {
		return err
	}
	if !res.OK {
		return fmt.Errorf("telegram api: %s", res.Description)
	}
	return nil
}

type tgResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func buildTelegramText(a Alert) string {
	var b strings.Builder
	title := a.Title
	if title == "" {
		title = "(no title)"
	}
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(a.Message)
	if a.TraceID != "" {
		b.WriteString("\n\nTraceID: ")
		b.WriteString(a.TraceID)
	}
	if len(a.Extra) > 0 {
		enc, err := json.Marshal(a.Extra)
		if err == nil && len(enc) > 0 {
			b.WriteString("\n\nExtra: ")
			b.Write(enc)
		}
	}
	if b.Len() == 0 {
		return "(empty alert)"
	}
	return b.String()
}
