package main

func removeEmptyMapFields(content map[string]interface{}) {
	for key, val := range content {
		switch typedVal := val.(type) {

		default:
			if val == nil {
				delete(content, key)
			}

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
	sliceCopy := []interface{}{}

	for _, elem := range slice {

		switch typedElem := elem.(type) {

		default:
			if elem != nil {
				sliceCopy = append(sliceCopy, elem)
			}

		case string:
			if typedElem != "" {
				sliceCopy = append(sliceCopy, typedElem)
			}

		case map[string]interface{}:
			removeEmptyMapFields(typedElem)
			if len(typedElem) != 0 {
				sliceCopy = append(sliceCopy, typedElem)
			}

		case []interface{}:
			sliceCopy = append(sliceCopy, removeEmptySliceValues(typedElem))

		}
	}

	return sliceCopy
}
