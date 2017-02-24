package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var mapWithSingleValue = map[string]interface{}{
	"string": "value",
}

var mapWithValueAndEmptySlice = map[string]interface{}{
	"string": "value",
	"slice":  []interface{}{},
}

func TestEmptyFieldRemoval_NilIsRemoved(t *testing.T) {
	content := map[string]interface{}{
		"string": "value",
		"nil":    nil,
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithSingleValue, content)
}

func TestEmptyFieldRemoval_EmptyStringIsRemoved(t *testing.T) {
	content := map[string]interface{}{
		"string":      "value",
		"emptyString": "",
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithSingleValue, content)
}

func TestEmptyFieldRemoval_EmptyMapIsRemoved(t *testing.T) {
	content := map[string]interface{}{
		"string":   "value",
		"emptyMap": map[string]interface{}{},
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithSingleValue, content)
}

func TestEmptyFieldRemoval_MapWithEmptyStringIsRemoved(t *testing.T) {
	content := map[string]interface{}{
		"string": "value",
		"emptyMap": map[string]interface{}{
			"emptyString": "",
		},
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithSingleValue, content)
}

func TestEmptyFieldRemoval_EmptyArrayIsNotRemoved(t *testing.T) {
	content := map[string]interface{}{
		"string": "value",
		"slice":  []interface{}{},
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithValueAndEmptySlice, content)
}

func TestEmptyFieldRemoval_ArrayWithNilIsEmptied(t *testing.T) {
	content := map[string]interface{}{
		"string": "value",
		"slice":  []interface{}{nil},
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithValueAndEmptySlice, content)
}

func TestEmptyFieldRemoval_ArrayWithEmptyValueIsEmptied(t *testing.T) {
	content := map[string]interface{}{
		"string": "value",
		"slice":  []interface{}{""},
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithValueAndEmptySlice, content)
}

func TestEmptyFieldRemoval_SliceWithEmptyMapIsEmptied(t *testing.T) {
	content := map[string]interface{}{
		"string": "value",
		"slice": []interface{}{
			map[string]interface{}{},
		},
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithValueAndEmptySlice, content)
}

func TestEmptyFieldRemoval_SliceWithMapWithNilIsEmptied(t *testing.T) {
	content := map[string]interface{}{
		"string": "value",
		"slice": []interface{}{
			map[string]interface{}{
				"nil": nil,
			},
		},
	}

	removeEmptyMapFields(content)

	assert.Equal(t, mapWithValueAndEmptySlice, content)
}

func TestEmptyFieldRemoval_complexCase(t *testing.T) {
	content := map[string]interface{}{
		"string":      "value",
		"emptyString": "",
		"nil":         nil,
		"map1":        map[string]interface{}{},
		"map2": map[string]interface{}{
			"map": map[string]interface{}{
				"nil": nil,
			},
		},
		"map3": map[string]interface{}{
			"map": map[string]interface{}{
				"string": "value",
				"slice": []interface{}{
					map[string]interface{}{
						"nil": nil,
					},
					map[string]interface{}{
						"emptyString": "",
					},
				},
			},
		},
		"slice1": []interface{}{
			map[string]interface{}{
				"nil": nil,
			},
			map[string]interface{}{
				"emptyString": "",
			},
			map[string]interface{}{
				"string": "value",
				"int":    0,
			},
			map[string]interface{}{
				"string": "value",
				"float":  0.0,
			},
		},
		"slice2": []interface{}{1, 2, 3},
		"slice3": []interface{}{
			nil,
			1,
			2,
			3,
			[]interface{}{
				map[string]interface{}{
					"nil": nil,
				},
				map[string]interface{}{
					"emptyString": "",
				},
				"",
				"",
			},
		},
	}

	expected := map[string]interface{}{
		"string": "value",
		"map3": map[string]interface{}{
			"map": map[string]interface{}{
				"string": "value",
				"slice":  []interface{}{},
			},
		},
		"slice1": []interface{}{
			map[string]interface{}{
				"string": "value",
				"int":    0,
			},
			map[string]interface{}{
				"string": "value",
				"float":  0.0,
			},
		},
		"slice2": []interface{}{1, 2, 3},
		"slice3": []interface{}{
			1,
			2,
			3,
			[]interface{}{},
		},
	}

	removeEmptyMapFields(content)

	assert.Equal(t, expected, content)
}
