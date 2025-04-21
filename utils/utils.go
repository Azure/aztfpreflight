package utils

import (
	"encoding/json"
)

func FormatJson(input string) string {
	var result interface{}
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return input
	}
	return ToJson(result)
}

func ToJson(input interface{}) string {
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func ToCompactJson(input interface{}) string {
	data, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return string(data)
}
