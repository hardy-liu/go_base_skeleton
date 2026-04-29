package trace

import (
	"context"

	"github.com/google/uuid"
)

// traceKey 为 context.Value 的私有键类型，避免与其他包键冲突。
type traceKey struct{}

// HeaderKey 为 HTTP 请求/响应中传递 Trace ID 的头部名称。
const HeaderKey = "X-Trace-Id"

// New 生成新的 Trace ID（UUID 字符串）。
func New() string {
	return uuid.New().String()
}

// WithCtx 将 traceID 存入 ctx，供 logger、GORM 等从同一请求上下文读取。
func WithCtx(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceKey{}, traceID)
}

// FromCtx 从 ctx 取出 Trace ID；未设置时返回空字符串。
func FromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(traceKey{}).(string); ok {
		return v
	}
	return ""
}
