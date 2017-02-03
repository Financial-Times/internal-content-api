package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestResolveImgURLs(t *testing.T) {
	APIHost := "unit-test.ft.com"
	topper := map[string]interface{}{
		"images": []interface{}{
			map[string]interface{}{"id": "56aed7e7-485f-303d-9605-b885b86e947e", "type": "square"},
			map[string]interface{}{"id": "56aed7e7-485f-303d-9605-b885b86e947f", "type": "wide"},
		},
	}

	resolveImageURLs(topper, APIHost)

	squareImgID := topper["images"].([]interface{})[0].(map[string]interface{})["id"]
	wideImgID := topper["images"].([]interface{})[1].(map[string]interface{})["id"]

	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947e", squareImgID.(string))
	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947f", wideImgID.(string))
}
