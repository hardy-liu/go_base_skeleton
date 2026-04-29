package util

import (
	"errors"
	"reflect"
	"unicode/utf8"
)

// map辅助函数：不存在则设置
func MapSetIfNotExist(m map[string]any, key string, val any) {
	if _, ok := m[key]; !ok {
		m[key] = val
	}
}

// EnsurePointer 确保v是指针
func EnsurePointer(v any) error {
	if v == nil {
		return errors.New("result cannot be nil")
	}

	if reflect.ValueOf(v).Kind() != reflect.Pointer {
		return errors.New("result must be a pointer")
	}

	return nil
}

// TruncateUTF8 截取字节数组，避免截断多字节字符（中文字符安全），maxBytes 最大字节数（非字符）
func TruncateUTF8(b []byte, maxBytes int) []byte {
	if len(b) <= maxBytes {
		return b
	}
	i := 0
	for i < maxBytes {
		_, size := utf8.DecodeRune(b[i:])
		if i+size > maxBytes {
			break
		}
		i += size
	}
	return b[:i]
}
