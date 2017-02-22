package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveImgURLs(t *testing.T) {
	APIHost := "unit-test.ft.com"
	content := map[string]interface{}{
		"topper": map[string]interface{}{
			"images": []interface{}{
				map[string]interface{}{"id": "56aed7e7-485f-303d-9605-b885b86e947e", "type": "square"},
				map[string]interface{}{"id": "56aed7e7-485f-303d-9605-b885b86e947f", "type": "wide"},
			},
		},
	}

	resolveImageURLs(content, APIHost)

	squareImgID := content["topper"].(map[string]interface{})["images"].([]interface{})[0].(map[string]interface{})["id"]
	wideImgID := content["topper"].(map[string]interface{})["images"].([]interface{})[1].(map[string]interface{})["id"]

	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947e", squareImgID.(string))
	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947f", wideImgID.(string))
}

func TestServeHTTP_CacheControlHeaderIsSet(t *testing.T) {
	sc := serviceConfig{
		cacheControlPolicy: "max-age=10",
	}

	appLogger := newAppLogger()
	metricsHandler := NewMetrics()
	contentHandler := contentHandler{&sc, appLogger, &metricsHandler}

	req, _ := http.NewRequest("GET", "http://internalcontentapi.ft.com/internalcontent/foobar", nil)
	w := httptest.NewRecorder()
	contentHandler.ServeHTTP(w, req)

	assert.Equal(t, w.Header().Get("Cache-Control"), "max-age=10")
}

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
