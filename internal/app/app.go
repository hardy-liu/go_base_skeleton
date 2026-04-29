package app

import (
	"fmt"
	"log"
	"path/filepath"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/event"
	"go_base_skeleton/internal/pkg/alert"
	"go_base_skeleton/internal/pkg/cache"
	"go_base_skeleton/internal/pkg/database"
	"go_base_skeleton/internal/pkg/httpclient"
	"go_base_skeleton/internal/pkg/lock"
	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/pkg/redis"
)

// App 聚合进程级依赖：配置、日志、MySQL（GORM）、业务 Redis、JSON 缓存、事件专用 Redis、事件发布器、分布式锁、多路告警。
// Publisher 使用 EventRedis；业务缓存/限流/分布式锁等使用 Redis。Cache 与 Locker 共用同一业务 Redis 客户端，进程内单例。Close 时两者均关闭。 AlertNotifier 无长连接、无需在 Close 中显式释放。
// logCleanup 经 closeResources 关闭（Sync 日志并关闭文件写入器）；New 失败时在内部按已分配资源传入 closeResources。
type App struct {
	Config        *config.Config
	Logger        *zap.Logger
	DB            *gorm.DB
	Redis         *goredis.Client
	EventRedis    *goredis.Client
	Publisher     *event.Publisher
	Cache         *cache.Cache
	Locker        lock.Locker
	HTTPClient    httpclient.Client
	AlertNotifier *alert.Notifier

	logCleanup func()
}

// New 按顺序加载配置、初始化日志（目录为 cfg.Log.Dir/logSubDir，文件前缀 logPrefix）、连接 MySQL 与 Redis。
// logSubDir 用于区分 api / admin / cli 等日志子目录；logPrefix 为日志文件名前缀（如 api、consume）。
// 日志初始化成功之后的任一步失败时，按当前已创建资源调用 closeResources 再返回 (nil, error)。
func New(configPath, logSubDir, logPrefix string) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logDir := filepath.Join(cfg.Log.Dir, logSubDir)
	zapLogger, logCleanup, err := logger.Init(cfg.Log, logDir, logPrefix)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	db, err := database.New(cfg.Database, zapLogger)
	if err != nil {
		closeResources(logCleanup, nil, nil, nil, nil)
		return nil, fmt.Errorf("init database: %w", err)
	}

	rdb, err := redis.New(cfg.Redis)
	if err != nil {
		closeResources(logCleanup, db, nil, nil, nil)
		return nil, fmt.Errorf("init redis: %w", err)
	}

	eventRdb, err := redis.New(cfg.Event.Redis)
	if err != nil {
		closeResources(logCleanup, db, rdb, nil, nil)
		return nil, fmt.Errorf("init event redis: %w", err)
	}

	//event发射的事件，单独记录在 event 目录下
	publisher, err := event.NewPublisher(eventRdb, cfg.Event, filepath.Join(cfg.Log.Dir, "event"), cfg.Log)
	if err != nil {
		closeResources(logCleanup, db, rdb, eventRdb, nil)
		return nil, fmt.Errorf("init event publisher: %w", err)
	}

	// 外部 HTTP 调用统一复用一个进程级 client，先采用包内默认配置，后续如有需要再提升到配置文件。
	httpClient := httpclient.NewRestyClient(httpclient.DefaultConfig, zapLogger, nil)

	return &App{
		Config:        cfg,
		Logger:        zapLogger,
		DB:            db,
		Redis:         rdb,
		EventRedis:    eventRdb,
		Publisher:     publisher,
		Cache:         cache.New(rdb),
		Locker:        lock.NewRedisLocker(rdb),
		HTTPClient:    httpClient,
		AlertNotifier: alert.New(zapLogger, &cfg.Alert, httpClient),
		logCleanup:    logCleanup,
	}, nil
}

// Close 释放 App 持有的连接与日志资源；可对同一实例多次调用（幂等）。
func (a *App) Close() {
	log.Printf("closing app")
	if a == nil {
		return
	}
	closeResources(a.logCleanup, a.DB, a.Redis, a.EventRedis, a.Publisher)
}

// New 失败时只传入已成功创建的部分；Close 传入 App 内字段。失败时错误被忽略，与常见 defer 清理一致。
func closeResources(logCleanup func(), db *gorm.DB, rdb, eventRdb *goredis.Client, publisher *event.Publisher) {
	if logCleanup != nil {
		log.Printf("closing logger")
		logCleanup()
	}
	if db != nil {
		if sqlDB, err := db.DB(); err == nil {
			log.Printf("closing database")
			_ = sqlDB.Close()
		}
	}
	if rdb != nil {
		log.Printf("closing redis")
		_ = rdb.Close()
	}
	if eventRdb != nil {
		log.Printf("closing eventRedis")
		_ = eventRdb.Close()
	}
	if publisher != nil {
		log.Printf("closing event publisher")
		_ = publisher.Close()
	}
}
