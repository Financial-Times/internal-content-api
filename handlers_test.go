package main

import (
	"testing"

	"encoding/json"
	"github.com/Financial-Times/transactionid-utils-go"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"reflect"
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

func TestMergeEmbeddedMapsWithOverlappingFields(t *testing.T) {
	data := []struct {
		name          string
		content       map[string]interface{}
		component     map[string]interface{}
		mergedContent map[string]interface{}
	}{
		{
			"Simple fields",
			map[string]interface{}{
				"field_c1": "value_c1",
			},
			map[string]interface{}{
				"field_ic1": "value_ic1",
			},
			map[string]interface{}{
				"field_c1":  "value_c1",
				"field_ic1": "value_ic1",
			},
		},
		{
			"Empty map as destination",
			map[string]interface{}{},
			map[string]interface{}{
				"field_ic1": "value_ic1",
			},
			map[string]interface{}{
				"field_ic1": "value_ic1",
			},
		},
		{
			"Empty map as argument",
			map[string]interface{}{
				"field_c1": "value_c1",
			},
			map[string]interface{}{},
			map[string]interface{}{
				"field_c1": "value_c1",
			},
		},
		{
			"Embedded fields as destination",
			map[string]interface{}{
				"field_c1": map[string]interface{}{
					"field_embed_c1": "value_c1",
				},
			},
			map[string]interface{}{
				"field_ic1": "value_ic1",
			},
			map[string]interface{}{
				"field_c1": map[string]interface{}{
					"field_embed_c1": "value_c1",
				},
				"field_ic1": "value_ic1",
			},
		},
		{
			"Embedded fields as agument",
			map[string]interface{}{
				"field_c1": "value_c1",
			},
			map[string]interface{}{
				"field_ic1": map[string]interface{}{
					"field_embed_ic1": "value_ic1",
				},
			},
			map[string]interface{}{
				"field_c1": "value_c1",
				"field_ic1": map[string]interface{}{
					"field_embed_ic1": "value_ic1",
				},
			},
		},
		{
			"Overlapping embedded fields",
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": "value_1",
				},
			},
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_2": "value_2",
				},
			},
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": "value_1",
					"field_2": "value_2",
				},
			},
		},
		{
			"Overlapping embedded fields - same names still overwritten",
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": "value_1",
				},
				"field_c2": "value_c2",
			},
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": "value_2",
				},
				"field_c2": map[string]interface{}{
					"field_1": "value_c2",
				},
			},
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": "value_2",
				},
				"field_c2": map[string]interface{}{
					"field_1": "value_c2",
				},
			},
		},
		{
			"Overlapping embedded fields - map on one side, value on other",
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": map[string]interface{}{
						"field_x": "1",
					},
				},
				"field_c2": "value_c2",
			},
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": "value_2",
				},
				"field_c2": map[string]interface{}{
					"field_1": "value_c2",
				},
			},
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": "value_2",
				},
				"field_c2": map[string]interface{}{
					"field_1": "value_c2",
				},
			},
		},
		{
			"Overlapping embedded fields - value on one side, map on other",
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": "value_2",
				},
				"field_c2": "value_c2",
			},
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": map[string]interface{}{
						"field_x": "1",
					},
				},
				"field_c2": map[string]interface{}{
					"field_1": "value_c2",
				},
			},
			map[string]interface{}{
				"field_c": map[string]interface{}{
					"field_1": map[string]interface{}{
						"field_x": "1",
					},
				},
				"field_c2": map[string]interface{}{
					"field_1": "value_c2",
				},
			},
		},
		{
			"Complex example",
			map[string]interface{}{
				"field_c": "value_c",
				"field": map[string]interface{}{
					"field_1": "value_1",
					"field_2": "value_2",
				},
			},
			map[string]interface{}{
				"field": map[string]interface{}{
					"field_1": "value_2",
					"field_3": "value_3",
				},
				"field_ic": "value_ic",
			},
			map[string]interface{}{
				"field_c": "value_c",
				"field": map[string]interface{}{
					"field_1": "value_2",
					"field_2": "value_2",
					"field_3": "value_3",
				},
				"field_ic": "value_ic",
			},
		},
	}

	for _, row := range data {
		res := mergeParts([]responsePart{{content: row.content}, {content: row.component}})
		assert.True(t, reflect.DeepEqual(row.mergedContent, res), "Expected and actual merged content differs.\n Expected: %v\n Actual %v\n", row.mergedContent, res)
	}

}

func TestFilterKeys(t *testing.T) {
	data := []struct {
		name            string
		content         map[string]interface{}
		filter          map[string]interface{}
		filteredContent map[string]interface{}
	}{
		{
			"simple",
			map[string]interface{}{
				"a": "1",
				"b": "2",
			},
			map[string]interface{}{
				"a": "",
			},
			map[string]interface{}{
				"b": "2",
			},
		},
		{
			"embedded",
			map[string]interface{}{
				"a": map[string]interface{}{
					"a": "11",
					"b": "22",
				},
				"b": "2",
			},
			map[string]interface{}{
				"a": map[string]interface{}{
					"a": "",
				},
			},
			map[string]interface{}{
				"a": map[string]interface{}{
					"b": "22",
				},
				"b": "2",
			},

		},
		{
			"empty filter",
			map[string]interface{}{
				"a": map[string]interface{}{
					"a": "11",
					"b": "22",
				},
				"b": "2",
			},
			map[string]interface{}{},
			map[string]interface{}{
				"a": map[string]interface{}{
					"a": "11",
					"b": "22",
				},
				"b": "2",
			},

		},
		{
			"empty content",
			map[string]interface{}{},
			map[string]interface{}{
				"a": map[string]interface{}{
					"a": "11",
					"b": "22",
				},
				"b": "2",
			},
			map[string]interface{}{},

		},
	}

	for _, row := range data {
		res := filterKeys(row.content, row.filter)
		assert.True(t, reflect.DeepEqual(row.filteredContent, res), "Expected and actual filtered content differs.\n Expected: %v\n Actual %v\n", row.filteredContent, res)
	}
}

func TestResolvingOverlappingMergesFullContent(t *testing.T) {

	contentJson := `{"uuid":"uuid1","title":"title1","alternativeStandfirsts":{"promotionalStandfirst":"stand first"},"alternativeTitles":{"promotionalTitle":"promo title","contentPackageTitle":null},"type":"Article","byline":"","brands":[{"id":"http://api.ft.com/things/brandid1"}],"identifiers":[{"authority":"id_key","identifierValue":"id_value"}],"publishedDate":"2017-08-24T07:47:10.000Z","standfirst":"standfirst","body":"<body> some text <\/body>","description":null,"mediaType":null,"pixelWidth":null,"pixelHeight":null,"internalBinaryUrl":null,"externalBinaryUrl":null,"members":null,"mainImage":null,"standout":{"editorsChoice":false,"exclusive":false,"scoop":false},"comments":{"enabled":true},"copyright":null,"webUrl":null,"publishReference":"tid_1","lastModified":"2017-08-30T11:12:42.772Z","canBeSyndicated":"verify","firstPublishedDate":"2017-08-24T07:47:10.000Z","accessLevel":"subscribed","canBeDistributed":"yes"}`
	internalComponentJson := `{"design":null,"tableOfContents":null,"topper":null,"leadImages":[],"unpublishedContentDescription":null,"bodyXML":"<body> some text <\/body>","uuid":"uuid1","lastModified":"2017-08-30T11:12:42.772Z","publishReference":"tid_1","alternativeTitles":{"shortTeaser":"short teaser"}}`
	var content, internalComponent map[string]interface{}

	err := json.Unmarshal([]byte(contentJson), &content)
	assert.Equal(t, nil, err, "Error %v", err)
	err = json.Unmarshal([]byte(internalComponentJson), &internalComponent)
	assert.Equal(t, nil, err, "Error %v", err)

	results := mergeParts([]responsePart{{content: content}, {content: internalComponent}})

	promotionalTitle := results["alternativeTitles"].(map[string]interface{})["promotionalTitle"]
	shortTeaser := results["alternativeTitles"].(map[string]interface{})["shortTeaser"]
	alternativeStandfirsts := results["alternativeStandfirsts"].(map[string]interface{})["promotionalStandfirst"]

	assert.Equal(t, "promo title", promotionalTitle)
	assert.Equal(t, "short teaser", shortTeaser)
	assert.Equal(t, "stand first", alternativeStandfirsts)
}
