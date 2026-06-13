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
	return omitEmptyValuesIn(data, "", false)
}

// omitEmptyValuesIn removes empty values recursively. parentKey is the key under
// which data sits in its parent map (empty for the root). keepEmptyStr is true
// while processing the Environment.Variables map: there, string values are kept
// even when empty, because an intentionally-empty environment variable is
// deployed to AWS as-is and dropping it would hide it from diff/render output.
// All other empty values (null, false, empty objects/arrays, and empty strings
// outside Environment.Variables) are still dropped everywhere.
func omitEmptyValuesIn(data any, parentKey string, keepEmptyStr bool) any {
	switch v := data.(type) {
	case map[string]any:
		nonEmptyMap := make(map[string]any)
		for key, value := range v {
			// Inside Environment.Variables, keep string values verbatim so an
			// empty environment variable survives.
			if keepEmptyStr {
				if s, ok := value.(string); ok {
					nonEmptyMap[key] = s
					continue
				}
			}
			inEnvVars := parentKey == "Environment" && key == "Variables"
			nonEmptyValue := omitEmptyValuesIn(value, key, inEnvVars)
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
			nonEmptyValue := omitEmptyValuesIn(value, "", false)
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
