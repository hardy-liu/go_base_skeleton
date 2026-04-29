// [Publisher] 测试：miniredis 模拟 Redis Stream，覆盖成功路径、各 [ErrEmptyPayload] / [ErrPublisherNotConfigured] /
// [ErrTopicNoWriter] 分支，以及审计文件与 payload 不原地修改等行为。
package event

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/constant"
	"go_base_skeleton/internal/pkg/trace"
	"go_base_skeleton/test/testhelper"
)

// TestPublisher_Ping_NotConfigured 未配置发布器（nil 接收方或 rdb 未注入）时 Ping 返回 [ErrPublisherNotConfigured]。
func TestPublisher_Ping_NotConfigured(t *testing.T) {
	ctx := context.Background()
	var pub *Publisher
	err := pub.Ping(ctx)
	require.ErrorIs(t, err, ErrPublisherNotConfigured)

	pub2 := &Publisher{rdb: nil}
	err = pub2.Ping(ctx)
	require.ErrorIs(t, err, ErrPublisherNotConfigured)
}

// TestPublisher_Publish_NotConfigured 同上，Publish 在首段校验与 Ping 一致。
func TestPublisher_Publish_NotConfigured(t *testing.T) {
	ctx := context.Background()
	topic := TopicDebug
	payload := map[string]any{"k": 1}

	var p *Publisher
	err := p.Publish(ctx, topic, payload)
	require.ErrorIs(t, err, ErrPublisherNotConfigured)

	p2 := &Publisher{rdb: nil, cfg: config.EventConfig{}}
	err = p2.Publish(ctx, topic, payload)
	require.ErrorIs(t, err, ErrPublisherNotConfigured)
}

// TestPublisher_Publish_UnknownTopic topic 不在 [NewPublisher] 预置写盘器内时，返回可 [errors.Is] 为 [ErrTopicNoWriter] 的错误，且附 topic 名。
func TestPublisher_Publish_UnknownTopic(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := config.EventConfig{StreamPrefix: "test_stream"}
	pub, err := NewPublisher(rdb, cfg, filepath.Join(t.TempDir(), "event"), config.LogConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	ctx := context.Background()
	unknown := "not-a-registered-event-topic-xyz-123"
	err = pub.Publish(ctx, unknown, map[string]any{"k": "v"})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrTopicNoWriter)
	assert.Contains(t, err.Error(), unknown, "消息中应带上 topic 名便于排查")
}

// TestPublisher_Publish_PayloadNotMutated 发布成功后，不应向调用方传入的 map 追加 trace_id、client_ip（由 buildEventData 内部拷贝再补全）。
func TestPublisher_Publish_PayloadNotMutated(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := config.EventConfig{StreamPrefix: "test_stream"}
	pub, err := NewPublisher(rdb, cfg, filepath.Join(t.TempDir(), "event"), config.LogConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	ctx := context.Background()
	topic := TopicDebug
	payload := map[string]any{
		"order_id": "o1",
		"time":     int64(100),
	}
	orig := len(payload)
	require.NoError(t, pub.Publish(ctx, topic, payload))
	assert.Len(t, payload, orig)
	_, hasTrace := payload["trace_id"]
	_, hasIP := payload["client_ip"]
	assert.False(t, hasTrace, "Publish 不应往调用方 map 上写入 trace_id")
	assert.False(t, hasIP, "Publish 不应往调用方 map 上写入 client_ip")
}

// TestPublisher_Publish_EmptyPayload payload 为 nil 或 len 为 0 时拒绝发布，返回 [ErrEmptyPayload]。
func TestPublisher_Publish_EmptyPayload(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := config.EventConfig{StreamPrefix: "test_stream"}
	pub, err := NewPublisher(rdb, cfg, filepath.Join(t.TempDir(), "event"), config.LogConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	ctx := context.Background()
	topic := TopicDebug
	var nilPayload map[string]any
	err = pub.Publish(ctx, topic, nilPayload)
	require.ErrorIs(t, err, ErrEmptyPayload)
	err = pub.Publish(ctx, topic, map[string]any{})
	require.ErrorIs(t, err, ErrEmptyPayload)
}

// TestPublisher_Publish 成功写入 Stream，data 为 JSON：业务字段保留；自动补全 client_ip、trace_id；payload 中已有 time 则不被覆盖。
func TestPublisher_Publish(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := config.EventConfig{StreamPrefix: "test_stream"}
	pub, err := NewPublisher(rdb, cfg, filepath.Join(t.TempDir(), "event"), config.LogConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	ctx := trace.WithCtx(context.Background(), "trace-001")
	ctx = context.WithValue(ctx, constant.ContextKeyClientIP, "1.2.3.4")
	topic := TopicDebug
	err = pub.Publish(ctx, topic, map[string]any{
		"message": "debug event",
		"time":    123,
	})
	require.NoError(t, err)

	streamKey := "test_stream:" + topic
	msgs, err := rdb.XRange(ctx, streamKey, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	assert.Equal(t, topic, msgs[0].Values["topic"])
	dataStr, ok := msgs[0].Values["data"].(string)
	require.True(t, ok)
	var dataMap map[string]any
	require.NoError(t, json.Unmarshal([]byte(dataStr), &dataMap))
	assert.Equal(t, "debug event", dataMap["message"])
	assert.Equal(t, "1.2.3.4", dataMap["client_ip"])
	assert.Equal(t, "trace-001", dataMap["trace_id"])
	assert.Equal(t, float64(123), dataMap["time"])

}

// TestPublisher_Publish_AuditLog 同一条发布在落盘审计日志中的 msg_id、topic 与 data 与 Stream 一致。
func TestPublisher_Publish_AuditLog(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := config.EventConfig{StreamPrefix: "test_stream"}
	logRoot := filepath.Join(t.TempDir(), "event")
	pub, err := NewPublisher(rdb, cfg, logRoot, config.LogConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	topic := TopicExample
	ctx := context.Background()
	require.NoError(t, pub.Publish(ctx, topic, map[string]any{"k": "v1"}))

	streamKey := "test_stream:" + topic
	msgs, err := rdb.XRange(ctx, streamKey, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	dataStr := msgs[0].Values["data"].(string)

	pattern := filepath.Join(logRoot, topic+"-"+time.Now().Format("2006-01-02")+".log")
	logBytes, err := os.ReadFile(pattern)
	require.NoError(t, err)
	lines := splitNonEmptyLines(string(logBytes))
	require.NotEmpty(t, lines)

	var line map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &line))
	assert.Equal(t, msgs[0].ID, line["msg_id"])
	payload, ok := line["payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, topic, payload["topic"])
	assert.Equal(t, dataStr, payload["data"])
}

// TestPublisher_Publish_MultipleTopic 不同 [PublishTopics] 对应不同 Stream，各自仅一条，互不串流。
func TestPublisher_Publish_MultipleTopic(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := config.EventConfig{StreamPrefix: "test_stream"}
	pub, err := NewPublisher(rdb, cfg, filepath.Join(t.TempDir(), "event"), config.LogConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	topics := PublishTopics()
	require.GreaterOrEqual(t, len(topics), 2)

	ctx := context.Background()
	require.NoError(t, pub.Publish(ctx, topics[0], map[string]any{"k": "v1"}))
	require.NoError(t, pub.Publish(ctx, topics[1], map[string]any{"k": "v2"}))

	lenA, err := rdb.XLen(ctx, "test_stream:"+topics[0]).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), lenA)

	lenB, err := rdb.XLen(ctx, "test_stream:"+topics[1]).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), lenB)
}

// splitNonEmptyLines 将内容按行拆成非空行（审计文件读入后去空行，便于取最后一行 JSON）。
func splitNonEmptyLines(content string) []string {
	lines := make([]string, 0)
	start := 0
	for i := 0; i <= len(content); i++ {
		if i == len(content) || content[i] == '\n' {
			if i > start {
				lines = append(lines, content[start:i])
			}
			start = i + 1
		}
	}
	return lines
}
