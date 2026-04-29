package middleware

import (
	"math"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/pkg/errcode"
	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/pkg/response"
)

// RateLimit 基于 golang.org/x/time/rate 的 per-IP 令牌桶限流中间件（进程内）。
//
// 说明：
// - 限流状态保存在当前进程内存中：多实例部署时，每个实例都会独立限流，无法做到「全局一致」。
// - 平均速率为 cfg.Rate / cfg.Window（requests per second）；Burst 为令牌桶容量，可由配置 ratelimit.burst 指定，未配置或 <=0 时默认为 10。
// - 超限返回 429 并设置 Retry-After（建议客户端按该秒数等待后重试）。
func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	// 与 Gin 引擎同生命周期：所有 HTTP 请求共享这一张表，不是「每个请求一份」。
	// 每个出现过的 ClientIP 会对应一个长期存活的 *rate.Limiter，条目不会随请求结束而删除，陌生 IP 很多时内存会持续增长（计划里未做 LRU/TTL 淘汰）。
	var limiters sync.Map // map[ip]*rate.Limiter

	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		// 与 AccessLog 等中间件一致：依赖 Trace 写入的 context，便于日志带 trace_id。
		l := logger.WithCtx(c.Request.Context())

		if cfg.Rate <= 0 || cfg.Window <= 0 {
			l.Warn("rate limit config invalid; skipping", zap.Int("rate", cfg.Rate), zap.Duration("window", cfg.Window))
			c.Next()
			return
		}

		burst := cfg.Burst
		if burst <= 0 {
			burst = 10
		}

		ip := c.ClientIP()
		raw, ok := limiters.Load(ip)
		if !ok {
			limit := rate.Limit(float64(cfg.Rate) / cfg.Window.Seconds())
			lim := rate.NewLimiter(limit, burst)
			actual, loaded := limiters.LoadOrStore(ip, lim)
			if loaded {
				raw = actual
			} else {
				raw = lim
				l.Debug("rate limiter created", zap.String("client_ip", ip), zap.Float64("tps", float64(cfg.Rate)/cfg.Window.Seconds()), zap.Int("burst", burst))
			}
		}

		lim, _ := raw.(*rate.Limiter)
		if lim == nil {
			l.Warn("rate limiter missing; skipping", zap.String("client_ip", ip))
			c.Next()
			return
		}

		if lim.Allow() {
			c.Next()
			return
		}

		// 需要返回 429：使用 Reserve 计算建议等待时间，但不占用令牌（Cancel）。
		r := lim.Reserve()
		delay := r.Delay()
		r.Cancel()

		// Retry-After 单位为秒：向上取整，避免返回 0 让客户端紧密重试。
		retryAfter := max(int(math.Ceil(delay.Seconds())), 1)

		c.Header("Retry-After", strconv.Itoa(retryAfter))
		l.Warn("request rate limited",
			zap.String("client_ip", ip),
			zap.Int("retry_after_sec", retryAfter),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
		)
		response.Fail(c, errcode.ErrRateLimit)
		c.Abort()
	}
}
