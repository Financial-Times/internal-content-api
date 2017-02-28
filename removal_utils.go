package main

func removeEmptyMapFields(content map[string]interface{}) {
	for key, val := range content {
		switch typedVal := val.(type) {

		case nil:
			delete(content, key)

		case string:
			if typedVal == "" {
				delete(content, key)
			}

		case map[string]interface{}:
			removeEmptyMapFields(typedVal)
			if len(typedVal) == 0 {
				delete(content, key)
			}

		case []interface{}:
			content[key] = removeEmptySliceValues(typedVal)
		}
	}
}

func removeEmptySliceValues(slice []interface{}) []interface{} {
	length := len(slice)

	for i := length - 1; i >= 0; i-- {
		switch typedElem := slice[i].(type) {

		case nil:
			slice = append(slice[:i], slice[i+1:]...)

		case string:
			if typedElem == "" {
				slice = append(slice[:i], slice[i+1:]...)
			}

		case map[string]interface{}:
			removeEmptyMapFields(typedElem)
			if len(typedElem) == 0 {
				slice = append(slice[:i], slice[i+1:]...)
			}

		case []interface{}:
			slice[i] = removeEmptySliceValues(typedElem)
		}
	}

	return slice
}
