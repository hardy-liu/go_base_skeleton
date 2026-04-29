package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
)

// Locker 定义高层锁执行器接口，业务只关心是否拿到锁，以及锁内逻辑是否执行成功。
type Locker interface {
	// TryWithLock 尝试获取锁并执行 fn；退出时尽力释放锁（lease 仍有效时）。
	//
	// 返回值约定：
	//   - acquired=false, err=nil      — 未抢到锁（如已被其他持有者占用），非系统错误，调用方可跳过本轮。
	//   - acquired=false, err!=nil     — 抢锁阶段失败（Redis/网络/ctx 取消等），fn 未执行。
	//   - acquired=true,  err=nil      — 已持有锁且 fn 与释放均成功。
	//   - acquired=true,  err!=nil     — 曾持有锁且 fn 已执行；err 可能来自 fn、来自释放锁、或二者（errors.Join）。
	TryWithLock(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) (acquired bool, err error)
}

// RedisLocker 基于 Redis 分布式锁实现高层执行器。
type RedisLocker struct {
	client *redislock.Client
}

// NewRedisLocker 创建 Redis 锁执行器。
func NewRedisLocker(rdb *redis.Client) *RedisLocker {
	return &RedisLocker{client: redislock.New(rdb)}
}

// TryWithLock 实现 [Locker.TryWithLock] 的语义，返回值情况见接口注释。
func (l *RedisLocker) TryWithLock(ctx context.Context, key string, ttl time.Duration, fn func(context.Context) error) (bool, error) {
	lease, err := l.client.Obtain(ctx, key, ttl, nil)
	if err != nil {
		if errors.Is(err, redislock.ErrNotObtained) {
			return false, nil
		}
		return false, fmt.Errorf("obtain lock %q: %w", key, err)
	}

	runErr := fn(ctx)
	releaseErr := releaseLease(ctx, lease)
	if runErr != nil || releaseErr != nil {
		return true, errors.Join(runErr, releaseErr)
	}
	return true, nil
}

// releaseLease 将上游“锁已不再由自己持有”的情况视为幂等释放成功，避免业务层处理噪音。
func releaseLease(ctx context.Context, lease *redislock.Lock) error {
	if lease == nil {
		return nil
	}

	err := lease.Release(ctx)
	if errors.Is(err, redislock.ErrLockNotHeld) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}
