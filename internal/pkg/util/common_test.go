package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapSetIfNotExist(t *testing.T) {
	t.Run("set when key missing", func(t *testing.T) {
		m := map[string]any{}

		// 缺失 key 时应写入新值。
		MapSetIfNotExist(m, "name", "alice")

		assert.Equal(t, "alice", m["name"])
	})

	t.Run("keep original when key exists", func(t *testing.T) {
		m := map[string]any{"name": "alice"}

		// 已存在 key 时不应覆盖旧值。
		MapSetIfNotExist(m, "name", "bob")

		assert.Equal(t, "alice", m["name"])
	})
}

func TestEnsurePointer(t *testing.T) {
	var p *int

	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name:    "nil value",
			input:   nil,
			wantErr: "result cannot be nil",
		},
		{
			name:    "non pointer",
			input:   123,
			wantErr: "result must be a pointer",
		},
		{
			name:  "pointer value",
			input: &struct{}{},
		},
		{
			name:  "typed nil pointer is allowed",
			input: p,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// EnsurePointer 只校验“是否为指针类型”，不要求指针已指向有效值。
			err := EnsurePointer(tt.input)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestTruncateUTF8(t *testing.T) {
	t.Run("keep original when shorter than limit", func(t *testing.T) {
		in := []byte("hello")

		// 未超限时直接返回原内容。
		out := TruncateUTF8(in, len(in))

		assert.Equal(t, in, out)
	})

	t.Run("truncate without breaking multibyte rune", func(t *testing.T) {
		in := []byte("ab中cd")

		// 4 字节不足以容纳 "ab中"，应安全截断到完整 rune 边界。
		out := TruncateUTF8(in, 4)

		assert.Equal(t, "ab", string(out))
	})

	t.Run("include rune when boundary fits exactly", func(t *testing.T) {
		in := []byte("ab中cd")

		// 5 字节刚好容纳 "ab中"，应保留完整中文字符。
		out := TruncateUTF8(in, 5)

		assert.Equal(t, "ab中", string(out))
	})

	t.Run("zero limit returns empty", func(t *testing.T) {
		in := []byte("中文")

		// 0 上限应返回空切片。
		out := TruncateUTF8(in, 0)

		assert.Empty(t, out)
	})
}
