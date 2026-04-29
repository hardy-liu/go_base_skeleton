package alert

type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Alert 为与渠道无关的告警事件；业务只构造本结构，渠道由 Notifier 组合。
type Alert struct {
	Level   Level
	Title   string
	Message string
	TraceID string
	Extra   map[string]any
}
