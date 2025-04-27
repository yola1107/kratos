package ext

import (
	"encoding/json"
)

// ToJSON json string
func ToJSON(v any) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

// ToJSONPretty converts any value to a pretty-printed JSON string.
// If encoding fails, it returns the error string.
func ToJSONPretty(v any) string {
	j, err := json.MarshalIndent(v, "", "  ") // 使用两个空格缩进
	if err != nil {
		return err.Error()
	}
	return string(j)
}
