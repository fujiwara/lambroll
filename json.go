package lambroll

func isEmptyValue(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case bool:
		return !v
	case map[string]interface{}:
		return len(v) == 0
	case []interface{}:
		return len(v) == 0
	default:
		return false
	}
}

func omitEmptyValues(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		nonEmptyMap := make(map[string]interface{})
		for key, value := range v {
			nonEmptyValue := omitEmptyValues(value)
			if !isEmptyValue(nonEmptyValue) {
				nonEmptyMap[key] = nonEmptyValue
			}
		}
		if len(nonEmptyMap) != 0 {
			return nonEmptyMap
		}
	case []interface{}:
		nonEmptyList := make([]interface{}, 0)
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
