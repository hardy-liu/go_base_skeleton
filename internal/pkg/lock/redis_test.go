package lock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go_base_skeleton/test/testhelper"
)

// TestRedisLocker_TryWithLock 获取未占用的锁时应执行回调，并在退出后释放锁。
func TestRedisLocker_TryWithLock(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	locker := NewRedisLocker(rdb)

	called := 0
	acquired, err := locker.TryWithLock(ctx, "lock:test:try", time.Second, func(context.Context) error {
		called++
		return nil
	})
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, 1, called)

	acquired, err = locker.TryWithLock(ctx, "lock:test:try", time.Second, func(context.Context) error {
		called++
		return nil
	})
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, 2, called)
}

// TestRedisLocker_TryWithLock_NotAcquired 同一把锁被其他持有者占用时，应返回未获取成功而不是系统错误。
func TestRedisLocker_TryWithLock_NotAcquired(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	locker := NewRedisLocker(rdb)

	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	lockDone := make(chan struct{})
	go func() {
		defer close(lockDone)
		acquired, err := locker.TryWithLock(ctx, "lock:test:conflict", time.Second, func(context.Context) error {
			close(lockHeld)
			<-releaseLock
			return nil
		})
		require.NoError(t, err)
		require.True(t, acquired)
	}()

	<-lockHeld

	called := false
	acquired, err := locker.TryWithLock(ctx, "lock:test:conflict", time.Second, func(context.Context) error {
		called = true
		return nil
	})
	require.NoError(t, err)
	require.False(t, acquired)
	require.False(t, called)

	close(releaseLock)
	<-lockDone
}

// TestRedisLocker_TryWithLock_HandlerError 业务回调报错时，应原样向上透传。
func TestRedisLocker_TryWithLock_HandlerError(t *testing.T) {
	ctx := context.Background()
	_, rdb := testhelper.NewMiniRedis(t)
	locker := NewRedisLocker(rdb)
	wantErr := errors.New("handler failed")

	acquired, err := locker.TryWithLock(ctx, "lock:test:handler_error", time.Second, func(context.Context) error {
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	require.True(t, acquired)
}

// TestRedisLocker_TryWithLock_DoesNotDeleteOtherOwner 当锁所有者发生变化时，释放阶段不应误删新的持有者。
func TestRedisLocker_TryWithLock_DoesNotDeleteOtherOwner(t *testing.T) {
	ctx := context.Background()
	mr, rdb := testhelper.NewMiniRedis(t)
	locker := NewRedisLocker(rdb)

	acquired, err := locker.TryWithLock(ctx, "lock:test:owner", time.Second, func(context.Context) error {
		// 直接改写底层值，模拟锁过期后已被其他持有者重新抢到。
		mr.Set("lock:test:owner", "another-owner")
		return nil
	})
	require.NoError(t, err)
	require.True(t, acquired)

	got, err := mr.Get("lock:test:owner")
	require.NoError(t, err)
	require.Equal(t, "another-owner", got)
}
