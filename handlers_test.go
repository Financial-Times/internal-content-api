package main

import (
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

	req := httptest.NewRequest("GET", "http://internalcontentapi.ft.com/internalcontent/foobar", nil)
	w := httptest.NewRecorder()
	contentHandler.ServeHTTP(w, req)

	assert.Equal(t, w.Header().Get("Cache-Control"), "max-age=10")
}
