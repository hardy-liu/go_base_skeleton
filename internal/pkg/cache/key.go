package cache

import "strings"

const keyPrefix = "go_base_skeleton"

// Key 按统一规范拼接缓存 key，并自动附加系统前缀。
func Key(parts ...string) string {
	if len(parts) == 0 {
		return keyPrefix
	}

	allParts := make([]string, 0, len(parts)+1)
	allParts = append(allParts, keyPrefix)
	allParts = append(allParts, parts...)
	return strings.Join(allParts, ":")
}
