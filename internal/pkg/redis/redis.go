package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"go_base_skeleton/internal/config"
)

// New 根据配置创建 go-redis 客户端并立即 Ping；失败则返回错误，避免启动后才发现连不上。
func New(cfg config.RedisConfig) (*goredis.Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return rdb, nil
}

// Ping 对 Redis 执行 PING
func Ping(ctx context.Context, rdb *goredis.Client) error {
	if rdb == nil {
		return fmt.Errorf("redis client is nil")
	}
	return rdb.Ping(ctx).Err()
}
