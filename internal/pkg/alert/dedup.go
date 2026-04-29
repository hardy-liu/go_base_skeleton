package alert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultDedupWindow = 60 * time.Second

// Deduplicator 可关闭的内存去重/冷却。Try 只读、Commit 在**至少一次成功投递**后写入，避免「全失败仍占满窗口」。
type Deduplicator struct {
	on     bool
	window time.Duration
	mu     sync.RWMutex
	last   map[string]int64
}

// NewDeduplicator on=false 时等价于关闭去重；on=true 且 window<=0 使用 defaultDedupWindow（60s）。
func NewDeduplicator(on bool, window time.Duration) *Deduplicator {
	var w time.Duration
	if on {
		if window > 0 {
			w = window
		} else {
			w = defaultDedupWindow
		}
	}
	return &Deduplicator{on: on, window: w, last: make(map[string]int64)}
}

// BuildDedupKey 与 Notifier 投递前使用的键规则一致，便于单测与文档对齐。
// 这里构建的 key 只用到了 Level、Title、Message、Extra["dedup_key"]，没有用到 TraceID。
func BuildDedupKey(a Alert) string {
	var b strings.Builder
	b.WriteString(string(a.Level))
	b.WriteByte(0)
	b.WriteString(a.Title)
	b.WriteByte(0)
	b.WriteString(a.Message)
	b.WriteByte(0)
	if a.Extra != nil {
		if v, ok := a.Extra["dedup_key"]; ok {
			if s, ok2 := v.(string); ok2 {
				b.WriteString(s)
			} else {
				//使用 json.Marshal(v) 是为了保证序列化结果的稳定性，从而确保同一个 dedup_key 值始终生成相同的去重键
				if bb, err := json.Marshal(v); err == nil {
					b.Write(bb)
				} else {
					b.WriteString(fmt.Sprint(v))
				}
			}
		}
	}
	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}

// Try 在不上写 last 的前提下，判断本次是否**允许尝试发送**：距上次 Commit 已超出 window 为 true，否则为 false（处于冷却、应跳过）。
// 去重关闭时恒为 true。
func (d *Deduplicator) Try(key string) bool {
	if d == nil || !d.on {
		return true
	}
	now := time.Now().UnixNano()
	d.mu.RLock()
	prev, ok := d.last[key]
	d.mu.RUnlock()
	if ok {
		elapsed := time.Duration(now - prev)
		if elapsed < d.window {
			return false
		}
	}
	return true
}

// Commit 将 key 的「上次成功投递」时间记为当前时刻；在至少一路 Sink 成功时由 Notifier 调用。去重关闭时 no-op。
func (d *Deduplicator) Commit(key string) {
	if d == nil || !d.on {
		return
	}
	now := time.Now().UnixNano()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.last[key] = now

	// 惰性清理：删除窗口期外的旧条目，防止无界增长
	cutoff := now - int64(d.window)
	for k, v := range d.last {
		if v < cutoff {
			delete(d.last, k)
		}
	}
}
