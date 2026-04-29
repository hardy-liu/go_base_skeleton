package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go_base_skeleton/test/testhelper"
)

type sampleValue struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TestKey 应按统一约定拼接缓存 key，并自动补上系统前缀。
func TestKey(t *testing.T) {
	got := Key("sample", "config", "123")
	require.Equal(t, "go_base_skeleton:sample:config:123", got)
}

// TestJitterTTL 应返回一个正数 TTL，且只在基础 TTL 附近做小幅抖动。
func TestJitterTTL(t *testing.T) {
	base := 10 * time.Minute
	got := JitterTTL(base)

	require.Greater(t, got, time.Duration(0))
	require.GreaterOrEqual(t, got, base)
	require.LessOrEqual(t, got, base+base/10)
}

// TestCache_Get_Miss 当 key 不存在时，应返回明确的缓存未命中错误。
func TestCache_Get_Miss(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	cache := New(rdb)

	var got sampleValue
	err := cache.Get(ctx, Key("sample", "config", "404"), &got)
	require.ErrorIs(t, err, ErrCacheMiss)
}

// TestCache_SetGetDelete 应支持对象写入、读取和删除。
func TestCache_SetGetDelete(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	cache := New(rdb)
	key := Key("sample", "config", "123")
	want := sampleValue{ID: 123, Name: "alice"}

	err := cache.Set(ctx, key, want, TTLMedium)
	require.NoError(t, err)

	var got sampleValue
	err = cache.Get(ctx, key, &got)
	require.NoError(t, err)
	require.Equal(t, want, got)

	err = cache.Delete(ctx, key)
	require.NoError(t, err)

	err = cache.Get(ctx, key, &got)
	require.ErrorIs(t, err, ErrCacheMiss)
}

// TestCache_Get_InvalidJSON 当缓存值已损坏时，应直接返回反序列化错误。
func TestCache_Get_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	mr, rdb := testhelper.NewMiniRedis(t)
	cache := New(rdb)
	key := Key("sample", "config", "broken")
	mr.Set(key, "{bad-json")

	var got sampleValue
	err := cache.Get(ctx, key, &got)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrCacheMiss))
}

// TestCache_Remember_Hit 命中缓存时，不应重复执行 loader。
func TestCache_Remember_Hit(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	cache := New(rdb)
	key := Key("sample", "config", "hit")
	want := sampleValue{ID: 1, Name: "cached"}

	err := cache.Set(ctx, key, want, TTLMedium)
	require.NoError(t, err)

	var loaderCalls int32
	var got sampleValue
	err = cache.Remember(ctx, key, TTLMedium, &got, func(context.Context) (any, error) {
		atomic.AddInt32(&loaderCalls, 1)
		return sampleValue{ID: 2, Name: "db"}, nil
	})
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Zero(t, atomic.LoadInt32(&loaderCalls))
}

// TestCache_Remember_MissAndBackfill 未命中时，应执行回源并完成回填。
func TestCache_Remember_MissAndBackfill(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	cache := New(rdb)
	key := Key("sample", "config", "miss")
	want := sampleValue{ID: 9, Name: "loaded"}

	var got sampleValue
	err := cache.Remember(ctx, key, TTLMedium, &got, func(context.Context) (any, error) {
		return want, nil
	})
	require.NoError(t, err)
	require.Equal(t, want, got)

	var cached sampleValue
	err = cache.Get(ctx, key, &cached)
	require.NoError(t, err)
	require.Equal(t, want, cached)
}

// TestCache_Remember_LoaderError 回源失败时，不应写入缓存。
func TestCache_Remember_LoaderError(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	cache := New(rdb)
	key := Key("sample", "config", "loader-error")
	wantErr := errors.New("loader failed")

	var got sampleValue
	err := cache.Remember(ctx, key, TTLMedium, &got, func(context.Context) (any, error) {
		return nil, wantErr
	})
	require.ErrorIs(t, err, wantErr)

	err = cache.Get(ctx, key, &got)
	require.ErrorIs(t, err, ErrCacheMiss)
}

// TestCache_Remember_Singleflight 同一个热点 key 并发 miss 时，应只执行一次 loader。
func TestCache_Remember_Singleflight(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	cache := New(rdb)
	key := Key("sample", "config", "hot")
	want := sampleValue{ID: 88, Name: "hot"}

	var loaderCalls int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]sampleValue, 8)
	errs := make([]error, 8)

	for i := range 8 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = cache.Remember(ctx, key, TTLMedium, &results[idx], func(context.Context) (any, error) {
				atomic.AddInt32(&loaderCalls, 1)
				time.Sleep(20 * time.Millisecond)
				return want, nil
			})
		}(i)
	}

	close(start)
	wg.Wait()

	for i := range errs {
		require.NoError(t, errs[i])
		require.Equal(t, want, results[i])
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&loaderCalls))
}

// TestCache_Set_WithExpiry 写入带 TTL 的缓存后，到期应返回未命中。
func TestCache_Set_WithExpiry(t *testing.T) {
	ctx := context.Background()
	mr, rdb := testhelper.NewMiniRedis(t)
	cache := New(rdb)
	key := Key("sample", "config", "ttl")
	want := sampleValue{ID: 7, Name: "ttl"}

	err := cache.Set(ctx, key, want, time.Second)
	require.NoError(t, err)

	mr.FastForward(2 * time.Second)

	var got sampleValue
	err = cache.Get(ctx, key, &got)
	require.ErrorIs(t, err, ErrCacheMiss)
}
