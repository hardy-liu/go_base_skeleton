package logger

import (
	"context"
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/pkg/trace"
)

type ctxLoggerKey struct{}

var defaultLogger *zap.Logger

// Init 初始化 Zap：同时输出到标准输出与 logDir 下文件日志；文件名为 {service}-YYYY-MM-DD.log，按自然日切换，同一天内由 lumberjack 滚动。
// 文件滚动相关默认值见 NewDailyLumberjackWriter（lumberjackRotateLimits）。
// 返回的 cleanup 应 defer 调用，用于 Sync 与关闭文件；level 解析失败时回退为 Debug。
func Init(logCfg config.LogConfig, logDir, service string) (*zap.Logger, func(), error) {
	lvl, err := zapcore.ParseLevel(logCfg.Level)
	if err != nil {
		lvl = zapcore.DebugLevel
	}

	fileWriter, err := NewDailyLumberjackWriter(logDir, service, logCfg)
	if err != nil {
		return nil, nil, err
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	consoleSyncer := zapcore.AddSync(os.Stdout)
	fileSyncer := zapcore.AddSync(fileWriter)
	multiSyncer := zapcore.NewMultiWriteSyncer(consoleSyncer, fileSyncer)

	encoder := zapcore.NewConsoleEncoder(encoderCfg)
	core := zapcore.NewCore(encoder, multiSyncer, lvl)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0))
	defaultLogger = logger

	cleanup := func() {
		_ = logger.Sync()
		_ = fileWriter.Close()
	}
	return logger, cleanup, nil
}

// Default 返回 Init 设置的全局 logger；若尚未 Init 则懒创建 Development 配置（仅兜底，生产应始终 Init）。
func Default() *zap.Logger {
	if defaultLogger == nil {
		defaultLogger, _ = zap.NewDevelopment()
	}
	return defaultLogger
}

// WithCtx 从 ctx 读取 trace_id 并作为字段注入 logger，便于与 AccessLog、GORM 等同链路透传。
func WithCtx(ctx context.Context) *zap.Logger {
	l := fromCtx(ctx)
	if traceID := trace.FromCtx(ctx); traceID != "" {
		l = l.With(zap.String("trace_id", traceID))
	}
	return l
}

// NewCtx 将指定 logger 存入 context（可选能力；当前业务路径以 trace_id 为主）。
func NewCtx(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey{}, l)
}

func fromCtx(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Default()
	}
	if l, ok := ctx.Value(ctxLoggerKey{}).(*zap.Logger); ok {
		return l
	}
	return Default()
}

// NewWriter 将 Zap 包装为 io.Writer，便于对接仅支持 Writer 的第三方库。
func NewWriter(l *zap.Logger, level zapcore.Level) io.Writer {
	return &logWriter{logger: l, level: level}
}

type logWriter struct {
	logger *zap.Logger
	level  zapcore.Level
}

func (w *logWriter) Write(p []byte) (int, error) {
	if ce := w.logger.Check(w.level, string(p)); ce != nil {
		ce.Write()
	}
	return len(p), nil
}
