package alert

import (
	"context"

	"go.uber.org/zap"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/pkg/httpclient"
	"go_base_skeleton/internal/pkg/trace"
)

// Notifier 对业务侧唯一入口；失败不冒泡，由 zap 英文明示。
type Notifier struct {
	log   *zap.Logger
	dedup *Deduplicator
	fo    *Fanout
	ac    *config.AlertConfig
}

// New 从配置装配 Sink；ac 为 nil 时按全关处理。!Enabled 时 Send 为轻量 no-op。
func New(log *zap.Logger, ac *config.AlertConfig, client httpclient.Client) *Notifier {
	if log == nil {
		log = zap.NewNop()
	}
	if ac == nil {
		empty := config.AlertConfig{}
		ac = &empty
	}
	dedup := NewDeduplicator(ac.DedupEnabled, ac.DedupWindow)
	var sinks []Sink
	timeout := ac.DefaultSinkTimeout
	if timeout <= 0 {
		timeout = defaultPerSinkTimeout
	}
	if ac.Enabled {
		if ac.Telegram.Enabled {
			sinks = append(sinks, &TelegramSink{
				BotToken: ac.Telegram.BotToken,
				ChatID:   ac.Telegram.ChatID,
				Client:   client,
			})
		}
		if ac.Webhook.Enabled {
			sinks = append(sinks, &WebhookSink{
				Client: client,
				URL:    ac.Webhook.URL,
			})
		}
	}
	fo := &Fanout{
		Sinks:          sinks,
		PerSinkTimeout: timeout,
	}
	n := &Notifier{log: log, dedup: dedup, fo: fo, ac: ac}
	if ac.Enabled && len(sinks) == 0 {
		log.Info("alert module enabled but no channel sinks are enabled; Send is a no-op")
	}
	return n
}

// Send 补全 TraceID、去重后异步投递；不因渠道失败而向调用方返回错误。
func (n *Notifier) Send(ctx context.Context, a Alert) {
	if n == nil || n.ac == nil || !n.ac.Enabled {
		return
	}
	out := a
	if out.TraceID == "" && ctx != nil {
		out.TraceID = trace.FromCtx(ctx)
	}
	key := ""
	if n.dedup != nil {
		key = BuildDedupKey(out)
		if !n.dedup.Try(key) {
			n.log.Info("alert throttled", zap.String("dedup_key", key))
			return
		}
	}
	go n.deliver(context.Background(), out, key)
}

func (n *Notifier) deliver(ctx context.Context, a Alert, key string) {
	if n.fo == nil {
		return
	}
	results := n.fo.SendAll(ctx, a)
	anyOK := false
	for _, r := range results {
		if r.Err == nil {
			anyOK = true
		} else {
			nm := r.Name
			if nm == "" {
				nm = "unknown"
			}
			n.log.Warn("alert sink failed", zap.String("sink", nm), zap.Error(r.Err))
		}
	}
	if anyOK && n.dedup != nil && key != "" {
		n.dedup.Commit(key)
	}
}
