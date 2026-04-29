package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/constant"
	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/pkg/trace"
	"go_base_skeleton/internal/pkg/util"
)

// Publisher 向 Redis Stream 写入事件；key为 cfg.StreamPrefix + ":" + topic。
type Publisher struct {
	rdb     *redis.Client
	cfg     config.EventConfig
	writers map[string]*logger.DailyLumberjackWriter
}

type auditPayload struct {
	Topic string `json:"topic"`
	Data  string `json:"data"`
}

type auditLine struct {
	MsgID   string       `json:"msg_id"`
	Payload auditPayload `json:"payload"`
}

// ErrEmptyPayload 表示 Publish 的 payload 为 nil 或空 map（不含任何键）。
var ErrEmptyPayload = errors.New("event payload is empty")

// ErrPublisherNotConfigured 表示 Publisher 未初始化、未注入 Redis 客户端，或 [Publisher.Ping] 在同样条件下失败。
var ErrPublisherNotConfigured = errors.New("event publisher: not configured")

// ErrTopicNoWriter 表示该 topic 在发布器内没有对应的审计写盘器（通常不是 [PublishTopics] 中的合法 topic）。可与 fmt.Errorf 包装，链上具体 topic 名供排查。
var ErrTopicNoWriter = errors.New("event topic has no writer")

// NewPublisher 创建发布器，并按 PublishTopics 初始化对应的审计写盘器。
func NewPublisher(rdb *redis.Client, cfg config.EventConfig, eventLogDir string, logCfg config.LogConfig) (*Publisher, error) {
	writers := make(map[string]*logger.DailyLumberjackWriter, len(PublishTopics()))
	for _, topic := range PublishTopics() {
		w, err := logger.NewDailyLumberjackWriter(eventLogDir, topic, logCfg)
		if err != nil {
			for _, created := range writers {
				_ = created.Close()
			}
			return nil, fmt.Errorf("init event writer for topic %s: %w", topic, err)
		}
		writers[topic] = w
	}
	return &Publisher{rdb: rdb, cfg: cfg, writers: writers}, nil
}

// Ping 检测事件用 Redis 连接；p 为 nil 或 rdb 未设置时返回 [ErrPublisherNotConfigured]（与 [Publisher.Publish] 同类情况一致）。
func (p *Publisher) Ping(ctx context.Context) error {
	if p == nil || p.rdb == nil {
		return ErrPublisherNotConfigured
	}
	return p.rdb.Ping(ctx).Err()
}

// Publish 将 payload JSON 序列化后写入 Stream，消息字段含 topic 与 data（字符串形式的 JSON）。未配置时返回 [ErrPublisherNotConfigured]；payload 须非空，否则 [ErrEmptyPayload]；topic 无对应写盘器时返回可 [errors.Is] 为 [ErrTopicNoWriter] 的包装错误。
func (p *Publisher) Publish(ctx context.Context, topic string, payload map[string]any) error {
	if p == nil || p.rdb == nil {
		return ErrPublisherNotConfigured
	}
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	writer, ok := p.writers[topic]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTopicNoWriter, topic)
	}

	dataStr, err := buildEventData(ctx, payload)
	if err != nil {
		return fmt.Errorf("build event data: %w", err)
	}

	stream := StreamKey(p.cfg, topic)
	args := &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{
			"topic": topic,
			"data":  dataStr,
		},
	}

	id, err := p.rdb.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("xadd %s: %w", stream, err)
	}
	if err := writeAuditLine(writer, id, topic, dataStr); err != nil {
		logger.WithCtx(ctx).Error("write event audit log failed",
			zap.String("topic", topic),
			zap.String("msg_id", id),
			zap.Error(err),
		)
	}

	logger.WithCtx(ctx).Info("event published",
		zap.String("stream", stream),
		zap.String("topic", topic),
		zap.String("msg_id", id),
		zap.String("data", dataStr),
	)
	return nil
}

// Close 关闭 publisher 内部持有的审计日志写盘器。
func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	var errs []string
	for topic, writer := range p.writers {
		if writer == nil {
			continue
		}
		if err := writer.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", topic, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close event writers: %s", strings.Join(errs, "; "))
	}
	return nil
}

// 把业务 payload 转成事件 data：先拷贝到独立 map，再自动补上 time/client_ip/trace_id 等字段；若已存在这些字段，则不覆盖。不修改传入的 payload。
func buildEventData(ctx context.Context, payload map[string]any) (string, error) {
	merged := make(map[string]any, len(payload)+3)
	maps.Copy(merged, payload)

	// 自动补全标准字段（不存在才添加）
	util.MapSetIfNotExist(merged, "time", time.Now().Unix())
	util.MapSetIfNotExist(merged, "client_ip", clientIPFromContext(ctx))
	util.MapSetIfNotExist(merged, "trace_id", trace.FromCtx(ctx))

	dataBytes, err := json.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshal merged data: %w", err)
	}
	return string(dataBytes), nil
}

func clientIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(constant.ContextKeyClientIP).(string); ok {
		return v
	}
	return ""
}

func writeAuditLine(writer *logger.DailyLumberjackWriter, msgID, topic, data string) error {
	line := auditLine{
		MsgID: msgID,
		Payload: auditPayload{
			Topic: topic,
			Data:  data,
		},
	}
	b, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("marshal audit line: %w", err)
	}
	b = append(b, '\n')
	if _, err := writer.Write(b); err != nil {
		return fmt.Errorf("append audit line: %w", err)
	}
	return nil
}
