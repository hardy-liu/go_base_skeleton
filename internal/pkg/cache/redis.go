package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// ErrCacheMiss 表示缓存中不存在目标 key。
var ErrCacheMiss = errors.New("cache miss")

// Cache 对 go-redis 提供最小可用的 JSON 缓存封装。
type Cache struct {
	rdb   *redis.Client
	group singleflight.Group
}

// New 基于已有 Redis 客户端创建缓存实例。
func New(rdb *redis.Client) *Cache {
	return &Cache{rdb: rdb}
}

// Get 读取缓存并将 JSON 内容反序列化到目标对象。
func (c *Cache) Get(ctx context.Context, key string, dest any) error {
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return fmt.Errorf("get cache key %q: %w", key, err)
	}

	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("unmarshal cache key %q: %w", key, err)
	}
	return nil
}

// Set 将对象序列化后写入 Redis，并设置过期时间。
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal cache key %q: %w", key, err)
	}

	if err := c.rdb.Set(ctx, key, raw, ttl).Err(); err != nil {
		return fmt.Errorf("set cache key %q: %w", key, err)
	}
	return nil
}

// Delete 删除指定缓存 key。
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete cache key %q: %w", key, err)
	}
	return nil
}

// Remember 按 cache-aside 模式读取缓存（命中缓存直接返回，否则使用loader方法获取到数据后回填缓存），并在 miss 时合并并发回源。
func (c *Cache) Remember(
	ctx context.Context,
	key string,
	ttl time.Duration,
	dest any,
	loader func(context.Context) (any, error),
) error {
	if err := c.Get(ctx, key, dest); err == nil {
		return nil
	} else if !errors.Is(err, ErrCacheMiss) {
		return err
	}

	value, err, _ := c.group.Do(key, func() (any, error) {
		loaded, loadErr := loader(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		if setErr := c.Set(ctx, key, loaded, ttl); setErr != nil {
			return nil, setErr
		}
		return loaded, nil
	})
	if err != nil {
		return err
	}

	return decodeInto(value, dest)
}

// decodeInto 统一复用 JSON 编解码路径，避免共享结果时出现类型断言分支。
func decodeInto(value any, dest any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal remember result: %w", err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("unmarshal remember result: %w", err)
	}
	return nil
}
