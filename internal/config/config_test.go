package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAppConfig_IsProduction(t *testing.T) {
	assert.True(t, AppConfig{Env: "production"}.IsProduction())
	assert.False(t, AppConfig{Env: "development"}.IsProduction())
	assert.False(t, AppConfig{Env: ""}.IsProduction())
}

func TestServerEntry_Addr(t *testing.T) {
	assert.Equal(t, ":8080", ServerEntry{Port: 8080}.Addr())
	assert.Equal(t, ":3000", ServerEntry{Port: 3000}.Addr())
	assert.Equal(t, ":0", ServerEntry{Port: 0}.Addr())
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "secret",
		DBName:   "go_base_skeleton",
		Charset:  "utf8mb4",
	}
	expected := "root:secret@tcp(localhost:3306)/go_base_skeleton?charset=utf8mb4&parseTime=True&loc=Local"
	assert.Equal(t, expected, cfg.DSN())
}

func TestRedisConfig_Addr(t *testing.T) {
	assert.Equal(t, "localhost:6379", RedisConfig{Host: "localhost", Port: 6379}.Addr())
	assert.Equal(t, "10.0.0.1:6380", RedisConfig{Host: "10.0.0.1", Port: 6380}.Addr())
}

func TestConfig_validate_Alert(t *testing.T) {
	t.Run("telegram enabled 缺 token 应报错", func(t *testing.T) {
		cfg := &Config{
			JWT: JWTConfig{Secret: "x"},
			Alert: AlertConfig{
				Telegram: AlertTelegramConfig{Enabled: true, ChatID: "1"},
			},
		}
		assert.Error(t, cfg.validate())
	})

	t.Run("telegram enabled 缺 chat_id 应报错", func(t *testing.T) {
		cfg := &Config{
			JWT: JWTConfig{Secret: "x"},
			Alert: AlertConfig{
				Telegram: AlertTelegramConfig{Enabled: true, BotToken: "t"},
			},
		}
		assert.Error(t, cfg.validate())
	})

	t.Run("webhook enabled 非 http(s) 应报错", func(t *testing.T) {
		cfg := &Config{
			JWT: JWTConfig{Secret: "x"},
			Alert: AlertConfig{
				Webhook: AlertWebhookConfig{Enabled: true, URL: "ftp://x"},
			},
		}
		assert.Error(t, cfg.validate())
	})

	t.Run("子渠道全关应通过", func(t *testing.T) {
		cfg := &Config{
			JWT:   JWTConfig{Secret: "x"},
			Alert: AlertConfig{},
		}
		assert.NoError(t, cfg.validate())
	})
}

func TestConfig_validate(t *testing.T) {
	t.Run("production环境未设置jwt.secret应报错", func(t *testing.T) {
		cfg := &Config{
			App: AppConfig{Env: "production"},
			JWT: JWTConfig{Secret: ""},
		}
		assert.Error(t, cfg.validate())
	})

	t.Run("production环境已设置jwt.secret应通过", func(t *testing.T) {
		cfg := &Config{
			App: AppConfig{Env: "production"},
			JWT: JWTConfig{Secret: "my-secret"},
		}
		assert.NoError(t, cfg.validate())
	})

	t.Run("非production环境无jwt.secret也应通过", func(t *testing.T) {
		cfg := &Config{
			App: AppConfig{Env: "development"},
			JWT: JWTConfig{Secret: ""},
		}
		assert.NoError(t, cfg.validate())
	})
}

func TestLoad_DatabaseMaxIdletime(t *testing.T) {
	configContent := `app:
  name: go_base_skeleton
  env: development
server:
  api:
    port: 8080
    read_timeout: 10s
    write_timeout: 10s
  admin:
    port: 8081
    read_timeout: 10s
    write_timeout: 10s
database:
  host: 127.0.0.1
  port: 3306
  user: root
  password: secret
  dbname: go_base_skeleton
  charset: utf8mb4
  max_open_conns: 50
  max_idle_conns: 10
  max_lifetime: 3600s
  max_idletime: 600s
redis:
  host: 127.0.0.1
  port: 6379
  password: ""
  db: 0
  pool_size: 100
jwt:
  secret: ""
  expire: 7200
  issuer: go_base_skeleton
log:
  level: info
  dir: ./log
ratelimit:
  enabled: false
  rate: 100
  window: 1m
  burst: 10
event:
  redis:
    host: 127.0.0.1
    port: 6379
    password: ""
    db: 0
    pool_size: 100
  stream_prefix: go_base_skeleton
  consumer_group: go_base_skeleton-group
  consumer_name: go_base_skeleton-consumer
  batch_size: 10
  block_time: 1s
  shutdown_timeout: 5s
alert:
  enabled: false
  dedup_enabled: false
  telegram:
    enabled: false
  webhook:
    enabled: false
`

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	assert.NoError(t, err)

	cfg, err := Load(configPath)
	assert.NoError(t, err)
	assert.Equal(t, 10*time.Minute, cfg.Database.MaxIdletime)
}
