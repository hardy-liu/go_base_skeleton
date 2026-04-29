package cache

import (
	"math/rand/v2"
	"time"
)

const (
	// TTLShort 适用于高频变化、允许短时缓存的数据。
	TTLShort = 1 * time.Minute
	// TTLMedium 适用于相对稳定但仍可能变化的配置类数据。
	TTLMedium = 10 * time.Minute
	// TTLLong 适用于低频变化或接近只读的映射型数据。
	TTLLong = 1 * time.Hour
)

const jitterDivisor = 10

// JitterTTL 为基础 TTL 追加一个较小的正向随机抖动，用于分散同批 key 的过期时间。
func JitterTTL(base time.Duration) time.Duration {
	if base <= 0 {
		return time.Millisecond
	}

	maxJitter := base / jitterDivisor
	if maxJitter <= 0 {
		return base
	}

	return base + time.Duration(rand.Int64N(int64(maxJitter)+1))
}
