package database

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/pkg/trace"
)

// New 使用 GORM 打开 MySQL，并应用连接池参数；日志通过自定义 gormZapLogger 输出到传入的 zapLogger。
func New(cfg config.DatabaseConfig, zapLogger *zap.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: newGormLogger(zapLogger),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.MaxIdletime)

	return db, nil
}

// Ping 对底层 sql.DB 执行 Ping，供健康检查使用；db 为 nil 时会 panic（由上层保证或 Recovery 捕获）。
func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// gormZapLogger 实现 gorm/logger.Interface：SQL 轨迹、慢查询与错误均带 trace_id（来自 ctx）。
type gormZapLogger struct {
	zap           *zap.Logger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

// newGormLogger 默认日志级别为 Warn，慢查询阈值 200ms；Trace 中根据错误/耗时/级别分支记录。
func newGormLogger(z *zap.Logger) gormlogger.Interface {
	return &gormZapLogger{
		zap:           z.Named("gorm"),
		level:         gormlogger.Info, //默认日志级别
		slowThreshold: 200 * time.Millisecond,
	}
}

func (l *gormZapLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	cp := *l
	cp.level = level
	return &cp
}

func (l *gormZapLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Info {
		l.zap.Sugar().With("trace_id", trace.FromCtx(ctx)).Infof(msg, args...)
	}
}

func (l *gormZapLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Warn {
		l.zap.Sugar().With("trace_id", trace.FromCtx(ctx)).Warnf(msg, args...)
	}
}

func (l *gormZapLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Error {
		l.zap.Sugar().With("trace_id", trace.FromCtx(ctx)).Errorf(msg, args...)
	}
}

func (l *gormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	fields := []zap.Field{
		zap.String("trace_id", trace.FromCtx(ctx)),
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil && l.level >= gormlogger.Error:
		l.zap.Error("query error", append(fields, zap.Error(err))...)
	case elapsed > l.slowThreshold && l.level >= gormlogger.Warn:
		l.zap.Warn("slow query", fields...)
	case l.level >= gormlogger.Info:
		l.zap.Info("query", fields...)
	}
}
