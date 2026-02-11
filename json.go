package lambroll

import (
	"encoding/json"
	"log/slog"
)

func isEmptyValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case bool:
		return !v
	case map[string]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	default:
		return false
	}
}

func omitEmptyValues(data any) any {
	switch v := data.(type) {
	case map[string]any:
		nonEmptyMap := make(map[string]any)
		for key, value := range v {
			nonEmptyValue := omitEmptyValues(value)
			if !isEmptyValue(nonEmptyValue) {
				nonEmptyMap[key] = nonEmptyValue
			}
		}
		if len(nonEmptyMap) != 0 {
			return nonEmptyMap
		}
	case []any:
		nonEmptyList := make([]any, 0)
		for _, value := range v {
			nonEmptyValue := omitEmptyValues(value)
			if !isEmptyValue(nonEmptyValue) {
				nonEmptyList = append(nonEmptyList, nonEmptyValue)
			}
		}
		if len(nonEmptyList) != 0 {
			return nonEmptyList
		}
	default:
		if !isEmptyValue(v) {
			return v
		}
	}
	return nil
}

func ToJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		slog.Warn("failed to marshal json", "error", err)
		return ""
	}
	return string(b)
}
