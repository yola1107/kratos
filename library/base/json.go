package base

import (
	"encoding/json"
)

// ToJSON json string
func ToJSON(v interface{}) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}
