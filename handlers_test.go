package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestResolveLeadImgURLs(t *testing.T) {
	APIHost := "unit-test.ft.com"
	content := map[string]interface{}{
		"leadImages": []interface{}{
			map[string]interface{}{"id": "56aed7e7-485f-303d-9605-b885b86e947e", "type": "square"},
			map[string]interface{}{"id": "56aed7e7-485f-303d-9605-b885b86e947f", "type": "wide"},
		},
	}

	resolveLeadImageURLs(content, APIHost)

	squareImgID := content["leadImages"].([]interface{})[0].(map[string]interface{})["id"]
	wideImgID := content["leadImages"].([]interface{})[1].(map[string]interface{})["id"]

	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947e", squareImgID.(string))
	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947f", wideImgID.(string))
}
