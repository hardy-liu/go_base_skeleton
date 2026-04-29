package alert

import (
	"context"
	"sync"
	"time"
)

const defaultPerSinkTimeout = 5 * time.Second

// Fanout 对多个 Sink 并发 Send，每路带独立子 ctx 超时；不记录日志，错误由调用方处理。
type Fanout struct {
	Sinks []Sink
	// PerSinkTimeout 为单路 Send 的 context 超时时长；0 时 SendAll 内使用 defaultPerSinkTimeout。
	PerSinkTimeout time.Duration
}

// SendAll 在 ctx 取消或子超时下尽可能投递所有路；对每路 error 单独收集。ctx 为父级（如 Background），非 HTTP 请求 ctx。
func (f *Fanout) SendAll(ctx context.Context, a Alert) []SinkSendResult {
	if f == nil || len(f.Sinks) == 0 {
		return nil
	}
	timeout := defaultPerSinkTimeout
	if t := f.PerSinkTimeout; t > 0 {
		timeout = t
	}
	out := make([]SinkSendResult, len(f.Sinks))
	var wg sync.WaitGroup
	for i, s := range f.Sinks {
		i, s := i, s //避坑写法，goroutine 延迟执行，会导致所有 goroutine 读到的都是最后一轮的 i 和 s。重新声明局部变量避免之
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			err := s.Send(cctx, a)
			if s != nil {
				out[i].Name = s.Name()
			}
			out[i].Err = err
		}()
	}
	wg.Wait()
	return out
}
