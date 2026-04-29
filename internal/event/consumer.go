package event

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/pkg/logger"
)

// HandlerFunc 单条 Stream 消息的处理函数；返回 error 时不 ACK，且 Run 会立即返回该错误（由进程托管策略决定是否重启）。
type HandlerFunc func(ctx context.Context, msg redis.XMessage) error

// Consumer 基于 Redis 消费组 XREADGROUP 阻塞拉取多个 topic 对应 Stream，并分发到已注册的 HandlerFunc。
type Consumer struct {
	rdb          *redis.Client
	cfg          config.EventConfig
	group        string
	consumerName string
	handlers     map[string]HandlerFunc // topic -> handler
}

// NewConsumer 创建消费者；需后续 Register topic 后再 Run。
// group 和 consumerName 由调用方指定，支持灵活的消费组配置。
func NewConsumer(rdb *redis.Client, cfg config.EventConfig, group, consumerName string) *Consumer {
	return &Consumer{
		rdb:          rdb,
		cfg:          cfg,
		group:        group,
		consumerName: consumerName,
		handlers:     make(map[string]HandlerFunc),
	}
}

// Register 为 topic 注册处理函数；Run 时会为 prefix:topic 创建消费组（若不存在）。
func (c *Consumer) Register(topic string, fn HandlerFunc) {
	c.handlers[topic] = fn
}

// Run 阻塞拉取所有已注册 topic 的 Stream 消息并分发到对应 HandlerFunc。
// 优雅退出：ctx 取消后不再拉取新消息，但会在 ShutdownTimeout 内让当前批次的 handler 执行完毕并 ACK，
// 避免因 context 取消导致 handler 内的 DB/Redis 操作中途失败。
func (c *Consumer) Run(ctx context.Context) error {
	streamKeys := make([]string, 0, len(c.handlers))
	for topic := range c.handlers {
		stream := StreamKey(c.cfg, topic)
		c.ensureGroup(ctx, stream)
		streamKeys = append(streamKeys, stream)
	}

	log := logger.WithCtx(ctx)
	log.Info("event consumer starting", zap.Strings("streams", streamKeys), zap.String("group", c.group), zap.String("consumer", c.consumerName))

	// 第一步：先消费所有 Pending 未 ACK 消息
	if err := c.processPendingMessages(ctx, log, streamKeys); err != nil {
		return err
	}

	// 第二步：消费新消息
	return c.processNewMessages(ctx, log, streamKeys)
}

// processPendingMessages 循环处理 Pending 消息（ID=0），直到耗尽
func (c *Consumer) processPendingMessages(ctx context.Context, log *zap.Logger, streamKeys []string) error {
	log.Debug("processPendingMessages start")
	for {
		select {
		case <-ctx.Done():
			log.Info("event consumer stopped")
			return nil
		default:
		}

		pending, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.group,
			Consumer: c.consumerName,
			Streams:  c.xreadStreams(streamKeys, "0"),
			Count:    c.cfg.BatchSize,
			Block:    1000 * time.Millisecond, // 给一个短超时避免阻塞
		}).Result()

		if err != nil {
			if ctx.Err() != nil {
				log.Info("event consumer stopped")
				return nil
			}
			if err == redis.Nil {
				log.Info("no pending messages, switch to new messages")
				break
			}
			log.Error("xreadgroup pending", zap.Error(err))
			continue
		}

		if !c.xstreamsHasMessages(pending) {
			log.Info("all pending msg processed")
			break
		}

		if err := c.processMessages(ctx, log, pending); err != nil {
			return err
		}

		if ctx.Err() != nil {
			log.Info("event consumer drained in-flight pending messages, stopped")
			return nil
		}
	}
	return nil
}

// processNewMessages 循环阻塞消费新消息（ID=>）
func (c *Consumer) processNewMessages(ctx context.Context, log *zap.Logger, streamKeys []string) error {
	log.Debug("processNewMessages start")
	for {
		select {
		case <-ctx.Done():
			log.Info("event consumer stopped")
			return nil
		default:
		}

		msgs, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.group,
			Consumer: c.consumerName,
			Streams:  c.xreadStreams(streamKeys, ">"),
			Count:    c.cfg.BatchSize,
			// Block超时了会返回 err == redis.Nil，需要继续调用 XReadGroup，不代表消息消费延时，只要超时之前有消息进来，XReadGroup会立马返回
			Block: c.cfg.BlockTime,
		}).Result()

		//这里的 error 绝大多数是 网络闪断 Redis 暂时不可达 超时之类的错误，遇到了就只是continue
		if err != nil {
			if ctx.Err() != nil {
				log.Info("event consumer stopped")
				return nil
			}
			if err == redis.Nil {
				continue
			}
			log.Error("xreadgroup error", zap.Error(err))
			continue
		}

		if err := c.processMessages(ctx, log, msgs); err != nil {
			return err
		}

		if ctx.Err() != nil {
			log.Info("event consumer drained in-flight messages, stopped")
			return nil
		}
	}
}

// xreadStreams 构造 XReadGroup 的 Streams：前一半为各 stream 键，后一半为每条流对应的起读 ID（"0" 表示本 consumer 的 PEL，">" 仅新消息）。
func (c *Consumer) xreadStreams(streamKeys []string, startID string) []string {
	out := make([]string, 0, len(streamKeys)*2)
	for _, s := range streamKeys {
		out = append(out, s)
	}
	for range streamKeys {
		out = append(out, startID)
	}
	return out
}

// xstreamsHasMessages 判断 XReadGroup 结果中是否含至少一条消息（空流或全空时用于结束 pending 耗尽循环）。
func (c *Consumer) xstreamsHasMessages(msgs []redis.XStream) bool {
	for i := range msgs {
		if len(msgs[i].Messages) > 0 {
			return true
		}
	}
	return false
}

// processMessages 批量处理消息：独立超时上下文，优雅兜底
func (c *Consumer) processMessages(ctx context.Context, log *zap.Logger, msgs []redis.XStream) error {
	shutdownTimeout := c.cfg.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 15 * time.Second
	}

	// 父ctx取消不终止正在执行的业务，超时强制收尾
	handlerCtx := context.WithoutCancel(ctx)
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		handlerCtx, cancel = context.WithTimeout(handlerCtx, shutdownTimeout)
		defer cancel()
		log.Info("draining in-flight messages", zap.Duration("timeout", shutdownTimeout))
	}

	for _, stream := range msgs {
		topic := c.topicFromStream(stream.Stream)
		log.Debug("processMessages topic: " + topic + " len: " + strconv.Itoa(len(stream.Messages)))
		handler, ok := c.handlers[topic]
		if !ok {
			log.Error("topic no register handler", zap.String("topic", topic))
			continue
		}
		for _, msg := range stream.Messages {
			log.Debug("process event message",
				zap.String("stream", stream.Stream),
				zap.String("msg_id", msg.ID),
				zap.Any("payload", msg.Values),
			)
			//todo 这里检查下 msgID， 如果时间戳离当前时间太远，说明消费者有延迟，需要告警处理，（告警使用异步队列）
			//handler遇到错误直接返回error退出，不再处理接下来的消息，等待进程重启，继续拉取pending的消息继续处理
			if err := handler(handlerCtx, msg); err != nil {
				log.Error("event handler error", zap.String("topic", topic), zap.String("msg_id", msg.ID), zap.Error(err))
				return fmt.Errorf("event handler topic=%s msg_id=%s: %w", topic, msg.ID, err)
			}
			//XACK 失败也返回error不再处理接下来的消息，handler 要做幂等，不然可能会重复消费peding数据
			if err := c.rdb.XAck(handlerCtx, stream.Stream, c.group, msg.ID).Err(); err != nil {
				log.Error("xack error", zap.String("topic", topic), zap.String("msg_id", msg.ID), zap.Error(err))
				return fmt.Errorf("xack topic=%s msg_id=%s: %w", topic, msg.ID, err)
			}
		}
	}
	return nil
}

// ensureGroup 对 stream 执行 XGROUP CREATE MKSTREAM；若组已存在则忽略 BUSYGROUP 类错误
func (c *Consumer) ensureGroup(ctx context.Context, stream string) {
	//0 - 历史全要（对新组而言）；$ - 只从建组之后新产生的消息开始，历史跳过，当前默认使用 $
	err := c.rdb.XGroupCreateMkStream(ctx, stream, c.group, "$").Err()
	// 组已存在忽略，其他异常打告警
	if err != nil && !redis.HasErrorPrefix(err, "BUSYGROUP") {
		logger.WithCtx(ctx).Warn("create stream consumer group fail",
			zap.String("stream", stream),
			zap.Error(err),
		)
	}
}

// topicFromStream 从完整 Stream 键中去掉配置的前缀与冒号，得到 topic 名。
func (c *Consumer) topicFromStream(stream string) string {
	prefix := c.cfg.StreamPrefix + ":"
	if len(stream) > len(prefix) {
		return stream[len(prefix):]
	}
	return stream
}

// StreamKey 返回某 topic 对应的完整 Redis Stream 键，测试或运维脚本可复用。
func StreamKey(cfg config.EventConfig, topic string) string {
	return fmt.Sprintf("%s:%s", cfg.StreamPrefix, topic)
}
