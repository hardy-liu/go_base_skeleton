package util

import (
	"bytes"
	"encoding/json"
)

// UnmarshalMap 安全解析 JSON 到 map，数字不丢失精度，使用 UseNumber() 后，不会将数字转成 float64 而是转成 json.Number
// json.Number 本质就是个 string，使用 num.Int64() num.Float64() num.String() 转到具体的类型
// json.Number 的值在 json.Marshal() 输出转成json字符串之后，对应的值是数字，不是字符串
func JsonSafeUnmarshalMap(data []byte) (map[string]any, error) {
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // 核心
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}
