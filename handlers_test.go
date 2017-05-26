package main

import (
	"testing"

	"github.com/Financial-Times/transactionid-utils-go"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestResolveLeadImgURLs(t *testing.T) {
	content := map[string]interface{}{
		"leadImages": []interface{}{
			map[string]interface{}{"id": "56aed7e7-485f-303d-9605-b885b86e947e", "type": "square"},
			map[string]interface{}{"id": "56aed7e7-485f-303d-9605-b885b86e947f", "type": "wide"},
		},
	}

	h := internalContentHandler{
		serviceConfig: &serviceConfig{envAPIHost: "unit-test.ft.com"},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, transactionidutils.TransactionIDKey, "sample_tid")
	ctx = context.WithValue(ctx, uuidKey, "4deb3541-d455-3a39-a51f-ebd94095aa98")
	ctx = context.WithValue(ctx, expandImagesKey, false)

	actual := resolveLeadImages(ctx, content, h)

	squareImgID := actual["leadImages"].([]interface{})[0].(map[string]interface{})["id"]
	wideImgID := actual["leadImages"].([]interface{})[1].(map[string]interface{})["id"]

	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947e", squareImgID.(string))
	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947f", wideImgID.(string))
}
