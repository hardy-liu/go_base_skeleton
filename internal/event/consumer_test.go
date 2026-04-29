package event

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/test/testhelper"
)

func TestStreamKey(t *testing.T) {
	cfg := config.EventConfig{StreamPrefix: "go_base_skeleton_stream"}
	assert.Equal(t, "go_base_skeleton_stream:sample.task.start", StreamKey(cfg, "sample.task.start"))
	assert.Equal(t, "go_base_skeleton_stream:example", StreamKey(cfg, "example"))
}

func TestConsumer_topicFromStream(t *testing.T) {
	c := &Consumer{cfg: config.EventConfig{StreamPrefix: "go_base_skeleton_stream"}}
	assert.Equal(t, "sample.task.start", c.topicFromStream("go_base_skeleton_stream:sample.task.start"))
	assert.Equal(t, "example", c.topicFromStream("go_base_skeleton_stream:example"))
}

func TestConsumer_Register(t *testing.T) {
	c := NewConsumer(nil, config.EventConfig{}, "test_group", "test_consumer")
	handler := func(_ context.Context, _ redis.XMessage) error { return nil }
	c.Register("test.topic", handler)
	_, ok := c.handlers["test.topic"]
	assert.True(t, ok, "handler should be registered for test.topic")
}

func TestConsumer_processPendingMessages_DrainAndAck(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := testEventConfig()
	group, consumer := "test_group", "test_consumer"
	c := NewConsumer(rdb, cfg, group, consumer)
	ctx := context.Background()

	topic := "test.topic.pending"
	stream := StreamKey(cfg, topic)
	c.ensureGroup(ctx, stream)

	addedID, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"foo": "bar"},
	}).Result()
	require.NoError(t, err)

	// 先读一次新消息但不 ACK，让消息进入当前 consumer 的 pending 列表
	_, err = rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	require.NoError(t, err)

	var handled int32
	c.Register(topic, func(_ context.Context, msg redis.XMessage) error {
		atomic.AddInt32(&handled, 1)
		assert.Equal(t, addedID, msg.ID)
		return nil
	})

	err = c.processPendingMessages(ctx, logger.WithCtx(ctx), []string{stream})
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&handled))

	pending, err := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, err)
	assert.Len(t, pending, 0, "pending messages should be fully ACKed")
}

func TestConsumer_processPendingMessages_HandlerErrorKeepsPending(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := testEventConfig()
	group, consumer := "test_group", "test_consumer"
	c := NewConsumer(rdb, cfg, group, consumer)
	ctx := context.Background()

	topic := "test.topic.pending.err"
	stream := StreamKey(cfg, topic)
	c.ensureGroup(ctx, stream)

	_, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"foo": "bar"},
	}).Result()
	require.NoError(t, err)

	// 先消费为 pending，模拟重启后从 ID=0 回捞处理
	_, err = rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	require.NoError(t, err)

	handlerErr := errors.New("pending handler failed")
	c.Register(topic, func(_ context.Context, _ redis.XMessage) error {
		return handlerErr
	})

	err = c.processPendingMessages(ctx, logger.WithCtx(ctx), []string{stream})
	require.Error(t, err)
	assert.ErrorIs(t, err, handlerErr)

	pending, pendErr := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, pendErr)
	require.Len(t, pending, 1, "failed pending message should remain pending")
}

func TestConsumer_processNewMessages_ConsumeThenStop(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := testEventConfig()
	group, consumer := "test_group", "test_consumer"
	c := NewConsumer(rdb, cfg, group, consumer)
	baseCtx := context.Background()

	topic := "test.topic.new"
	stream := StreamKey(cfg, topic)
	c.ensureGroup(baseCtx, stream)

	ctx, cancel := context.WithCancel(baseCtx)
	defer cancel()

	done := make(chan struct{}, 1)
	c.Register(topic, func(_ context.Context, _ redis.XMessage) error {
		done <- struct{}{}
		cancel()
		return nil
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.processNewMessages(ctx, logger.WithCtx(baseCtx), []string{stream})
	}()

	_, err := rdb.XAdd(baseCtx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"foo": "new"},
	}).Result()
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("message was not consumed in time")
	}

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("processNewMessages did not stop in time")
	}
}

func TestConsumer_Run_HandlerErrorStopsAndKeepsPending(t *testing.T) {
	_, rdb := testhelper.NewMiniRedis(t)
	cfg := testEventConfig()
	group, consumer := "test_group", "test_consumer"
	c := NewConsumer(rdb, cfg, group, consumer)
	ctx := context.Background()

	topic := "test.topic.err"
	stream := StreamKey(cfg, topic)

	// 先建组（$）再写入消息，确保该消息属于“新消息”可被 Run 消费到。
	c.ensureGroup(ctx, stream)

	_, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"foo": "err"},
	}).Result()
	require.NoError(t, err)

	handlerErr := errors.New("boom")
	c.Register(topic, func(_ context.Context, _ redis.XMessage) error {
		return handlerErr
	})

	err = c.Run(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, handlerErr)

	pending, pendErr := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, pendErr)
	require.Len(t, pending, 1, "failed message should remain pending due to no ACK")
}

func testEventConfig() config.EventConfig {
	return config.EventConfig{
		StreamPrefix: "test_stream",
		BatchSize:    10,
		BlockTime:    20 * time.Millisecond,
	}
}
