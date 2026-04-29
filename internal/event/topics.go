package event

// 事件 topic 常量，Publisher 和 Consumer 共用，确保两侧 topic 一致。
const (
	TopicExample = "example" // 示例事件
	TopicDebug   = "debug"   // 调试事件
)

// PublishTopics 返回允许发布并需要初始化审计日志写盘器的 topic 列表（单一来源）。
func PublishTopics() []string {
	return []string{
		TopicExample,
		TopicDebug,
	}
}
