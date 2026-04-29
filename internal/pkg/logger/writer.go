package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"go_base_skeleton/internal/config"
)

// DailyLumberjackWriter 按自然日切换日志路径 {dir}/{prefix}-YYYY-MM-DD.log；同一天内由 lumberjack 按大小与保留策略滚动。
type DailyLumberjackWriter struct {
	dir        string
	prefix     string
	maxSize    int
	maxBackups int
	maxAge     int

	mu      sync.Mutex
	curDate string
	lj      *lumberjack.Logger
}

// lumberjackRotateLimits 从 LogConfig 取出 lumberjack 参数；MaxSize/MaxBackups/MaxAge 任一项 <=0 时分别回退为 100（MB）、7、30（天）。
// 后续若在 LogConfig 增加 Compress、LocalTime 等字段，可只改此处与 newLumberjackForDay。
func lumberjackRotateLimits(cfg config.LogConfig) (maxSize, maxBackups, maxAge int) {
	maxSize = cfg.MaxSize
	if maxSize <= 0 {
		maxSize = 100
	}
	maxBackups = cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 7
	}
	maxAge = cfg.MaxAge
	if maxAge <= 0 {
		maxAge = 30
	}
	return maxSize, maxBackups, maxAge
}

func newLumberjackForDay(dir, prefix, date string, maxSize, maxBackups, maxAge int) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   filepath.Join(dir, fmt.Sprintf("%s-%s.log", prefix, date)),
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   true,
		LocalTime:  true,
	}
}

// NewDailyLumberjackWriter 创建写入器并确保 dir 存在，并绑定「当天」的 lumberjack 实例。
// logCfg 中与文件滚动相关的字段会在此解析（含默认值）；Level、Dir 等由调用方另作处理。
func NewDailyLumberjackWriter(dir, prefix string, logCfg config.LogConfig) (*DailyLumberjackWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir %s: %w", dir, err)
	}
	maxSize, maxBackups, maxAge := lumberjackRotateLimits(logCfg)
	today := time.Now().Format("2006-01-02")
	return &DailyLumberjackWriter{
		dir:        dir,
		prefix:     prefix,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		maxAge:     maxAge,
		curDate:    today,
		lj:         newLumberjackForDay(dir, prefix, today, maxSize, maxBackups, maxAge),
	}, nil
}

// Write 实现 io.Writer；跨日时关闭当日 lumberjack 并切换到新日期的 Filename。
func (w *DailyLumberjackWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if today != w.curDate {
		if w.lj != nil {
			_ = w.lj.Close()
		}
		w.lj = newLumberjackForDay(w.dir, w.prefix, today, w.maxSize, w.maxBackups, w.maxAge)
		w.curDate = today
	}
	return w.lj.Write(p)
}

// Close 关闭当前日的 lumberjack。
func (w *DailyLumberjackWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.lj != nil {
		return w.lj.Close()
	}
	return nil
}
