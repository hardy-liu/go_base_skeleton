package alert

import "context"

// 渠道在日志/指标中的稳定标识，全包统一引用避免拼写漂移。
const (
	SinkTelegramName = "telegram"
	SinkWebhookName  = "webhook"
)

// Sink 单一投递渠道；由静态组合在 Fanout 中并发调用。
type Sink interface {
	Name() string
	Send(ctx context.Context, a Alert) error
}

// SinkSendResult 为 Fanout 对单路 Send 的汇总，供 Notifier 打日志等使用。
type SinkSendResult struct {
	Name string
	Err  error
}
