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
			removeEmptySliceValues(&typedVal)
			content[key] = typedVal
		}
	}
}

func removeEmptySliceValues(slice *[]interface{}) {
	localSlice := *slice
	length := len(localSlice)

	for i := length - 1; i >= 0; i-- {
		switch typedElem := localSlice[i].(type) {

		case nil:
			localSlice = append(localSlice[:i], localSlice[i+1:]...)

		case string:
			if typedElem == "" {
				localSlice = append(localSlice[:i], localSlice[i+1:]...)
			}

		case map[string]interface{}:
			removeEmptyMapFields(typedElem)
			if len(typedElem) == 0 {
				localSlice = append(localSlice[:i], localSlice[i+1:]...)
			}

		case []interface{}:
			removeEmptySliceValues(&typedElem)
			localSlice[i] = typedElem
		}
	}

	*slice = localSlice
}
