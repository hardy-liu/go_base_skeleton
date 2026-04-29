package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 为应用级配置的根结构体，字段与 config.yaml 及环境变量（通过 Viper）对应。
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Log       LogConfig       `mapstructure:"log"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
	Event     EventConfig     `mapstructure:"event"`
	Alert     AlertConfig     `mapstructure:"alert"`
}

// AppConfig 描述应用名称与运行环境（如 development / production）。
type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

// IsProduction 当 env 为 production 时为 true，用于开启更严格的校验（如 JWT 密钥必填）。
func (c AppConfig) IsProduction() bool {
	return c.Env == "production"
}

// ServerConfig 分别配置业务 API 与管理后台 API 的监听端口与读写超时。
type ServerConfig struct {
	API   ServerEntry `mapstructure:"api"`
	Admin ServerEntry `mapstructure:"admin"`
}

// ServerEntry 为单个 HTTP 服务的监听与超时设置。
type ServerEntry struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// Addr 返回 Gin/http.Server 使用的监听地址，形如 ":8080"。
func (s ServerEntry) Addr() string {
	return fmt.Sprintf(":%d", s.Port)
}

// DatabaseConfig MySQL 连接与连接池参数；DSN 由 DSN() 拼接。
type DatabaseConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	User         string        `mapstructure:"user"`
	Password     string        `mapstructure:"password"`
	DBName       string        `mapstructure:"dbname"`
	Charset      string        `mapstructure:"charset"`
	MaxOpenConns int           `mapstructure:"max_open_conns"`
	MaxIdleConns int           `mapstructure:"max_idle_conns"`
	MaxLifetime  time.Duration `mapstructure:"max_lifetime"`
	MaxIdletime  time.Duration `mapstructure:"max_idletime"`
}

// DSN 生成 GORM MySQL DSN 字符串（含 parseTime、loc=Local）。
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.Charset)
}

// RedisConfig 单机 Redis 地址、库号与连接池大小。
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// Addr 返回 host:port，供 go-redis 使用。
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// JWTConfig 签发与校验 JWT 所需的密钥、过期时间与签发方（issuer）。
// Expire 在 Load 中若 YAML 为整型秒数且反序列化为 0，会由 jwt.expire 再次转为 time.Duration。
type JWTConfig struct {
	Secret string        `mapstructure:"secret"`
	Expire time.Duration `mapstructure:"expire"`
	Issuer string        `mapstructure:"issuer"`
}

// LogConfig 日志级别、根目录与文件滚动参数（实际文件路径会再拼接子目录，见 app.New）。
// MaxSize/MaxBackups/MaxAge 供 lumberjack 使用；任一项 <=0 时由 logger 包在创建 DailyLumberjackWriter 时采用内置默认值。
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Dir        string `mapstructure:"dir"`
	MaxSize    int    `mapstructure:"max_size"`    // 单文件上限（MB）
	MaxBackups int    `mapstructure:"max_backups"` // 保留的压缩/轮转文件个数
	MaxAge     int    `mapstructure:"max_age"`     // 保留天数
}

// RateLimitConfig 进程内按 IP 的令牌桶限流（golang.org/x/time/rate）：平均速率为 Rate/Window；Burst 为桶容量（突发允许的最大并发请求数），未配置或 <=0 时中间件内默认为 10。多实例时各进程独立计数。
type RateLimitConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	Rate    int           `mapstructure:"rate"`
	Window  time.Duration `mapstructure:"window"`
	Burst   int           `mapstructure:"burst"`
}

// EventConfig Redis Stream 事件：独立 Redis 连接（与顶层 redis 隔离）、流名前缀、每次拉取条数、阻塞读等待时间。
// ConsumerGroup 和 ConsumerName 由各 Consumer 命令独立配置，通过 NewConsumer 参数传入。
type EventConfig struct {
	Redis           RedisConfig   `mapstructure:"redis"`
	StreamPrefix    string        `mapstructure:"stream_prefix"`
	BatchSize       int64         `mapstructure:"batch_size"`
	BlockTime       time.Duration `mapstructure:"block_time"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// AlertConfig 多渠道路径告警：全模块开关、去重与静态组合的 Telegram/Webhook；密钥见 bindEnv 对应环境变量。
type AlertConfig struct {
	Enabled            bool                `mapstructure:"enabled"`
	DedupEnabled       bool                `mapstructure:"dedup_enabled"`
	DedupWindow        time.Duration       `mapstructure:"dedup_window"`
	DefaultSinkTimeout time.Duration       `mapstructure:"default_sink_timeout"`
	Telegram           AlertTelegramConfig `mapstructure:"telegram"`
	Webhook            AlertWebhookConfig  `mapstructure:"webhook"`
}

// AlertTelegramConfig Telegram Bot 投递参数。
type AlertTelegramConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
	ChatID   string `mapstructure:"chat_id"`
}

// AlertWebhookConfig HTTP Webhook 投递目标。
type AlertWebhookConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
}

// Load 先尝试加载当前目录 .env，再读取 configPath 的 YAML，并启用环境变量覆盖（键中的 . 替换为 _）。
// 部分敏感项通过 bindEnvOverrides 显式绑定到 DB_PASSWORD 等环境变量。
func Load(configPath string) (*Config, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.SetConfigFile(configPath)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	bindEnvOverrides(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// YAML 中 jwt.expire 常为整型「秒」；若 Unmarshal 后 Expire 仍为 0，则按秒转为 time.Duration。
	if cfg.JWT.Expire == 0 {
		secs := v.GetInt("jwt.expire")
		if secs > 0 {
			cfg.JWT.Expire = time.Duration(secs) * time.Second
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// bindEnvOverrides 将若干配置项与固定环境变量名绑定，便于在部署时只改环境变量而不改 YAML。
func bindEnvOverrides(v *viper.Viper) {
	_ = v.BindEnv("app.env", "APP_ENV")
	_ = v.BindEnv("database.host", "DB_HOST")
	_ = v.BindEnv("database.user", "DB_USER")
	_ = v.BindEnv("database.dbname", "DB_NAME")
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("redis.host", "REDIS_HOST")
	_ = v.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = v.BindEnv("event.redis.host", "EVENT_REDIS_HOST")
	_ = v.BindEnv("event.redis.password", "EVENT_REDIS_PASSWORD")
	_ = v.BindEnv("event.redis.db", "EVENT_REDIS_DB")
	_ = v.BindEnv("jwt.secret", "JWT_SECRET")
	_ = v.BindEnv("alert.telegram.bot_token", "ALERT_TELEGRAM_BOT_TOKEN")
	_ = v.BindEnv("alert.telegram.chat_id", "ALERT_TELEGRAM_CHAT_ID")
	_ = v.BindEnv("alert.webhook.url", "ALERT_WEBHOOK_URL")
}

// validate 在加载后做必要校验；生产环境必须配置 jwt.secret，避免无密钥签发。
func (c *Config) validate() error {
	if c.App.IsProduction() && c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret must be set in production")
	}
	if c.Alert.Telegram.Enabled {
		if strings.TrimSpace(c.Alert.Telegram.BotToken) == "" {
			return fmt.Errorf("alert.telegram.bot_token is required when alert.telegram.enabled is true")
		}
		if strings.TrimSpace(c.Alert.Telegram.ChatID) == "" {
			return fmt.Errorf("alert.telegram.chat_id is required when alert.telegram.enabled is true")
		}
	}
	if c.Alert.Webhook.Enabled {
		u := strings.TrimSpace(c.Alert.Webhook.URL)
		if u == "" {
			return fmt.Errorf("alert.webhook.url is required when alert.webhook.enabled is true")
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return fmt.Errorf("alert.webhook.url must start with http:// or https://")
		}
	}
	return nil
}
