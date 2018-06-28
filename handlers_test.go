package main

import (
	"fmt"
	"testing"

	"encoding/json"
	"io/ioutil"
	"reflect"

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
	ctx = context.WithValue(ctx, unrollContentKey, false)

	actual := resolveLeadImages(ctx, content, h)

	squareImgID := actual["leadImages"].([]interface{})[0].(map[string]interface{})["id"]
	wideImgID := actual["leadImages"].([]interface{})[1].(map[string]interface{})["id"]

	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947e", squareImgID.(string))
	assert.Equal(t, "http://unit-test.ft.com/content/56aed7e7-485f-303d-9605-b885b86e947f", wideImgID.(string))
}

func TestMergeEmbeds(t *testing.T) {
	data := []struct {
		name          string
		content       map[string]interface{}
		component     map[string]interface{}
		mergedContent map[string]interface{}
	}{
		{
			"Empty A remains B",
			map[string]interface{}{
				"embeds": []interface{}{},
			},
			map[string]interface{}{
				"embeds": []interface{}{
					map[string]interface{}{
						"id":                "2",
						"alternativeImages": map[string]interface{}{},
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description2",
						"lastModified":      "lastModified2",
					},
				},
			},
			map[string]interface{}{
				"embeds": []interface{}{
					map[string]interface{}{
						"id":                "2",
						"alternativeImages": map[string]interface{}{},
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description2",
						"lastModified":      "lastModified2",
					},
				},
			},
		},
		{
			"embeds are concatenated",
			map[string]interface{}{
				"embeds": []interface{}{
					map[string]interface{}{
						"id":                "1",
						"alternativeImages": map[string]interface{}{},
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description1",
						"lastModified":      "lastModified1",
					},
				},
			},
			map[string]interface{}{
				"embeds": []interface{}{
					map[string]interface{}{
						"id":                "2",
						"alternativeImages": map[string]interface{}{},
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description2",
						"lastModified":      "lastModified2",
					},
				},
			},
			map[string]interface{}{
				"embeds": []interface{}{
					map[string]interface{}{
						"id":                "1",
						"alternativeImages": map[string]interface{}{},
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description1",
						"lastModified":      "lastModified1",
					},
					map[string]interface{}{
						"id":                "2",
						"alternativeImages": map[string]interface{}{},
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description2",
						"lastModified":      "lastModified2",
					},
				},
			},
		},
		{
			"embeds are updated",
			map[string]interface{}{
				"embeds": []interface{}{
					map[string]interface{}{
						"id":                "1",
						"alternativeImages": map[string]interface{}{},
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description1",
						"lastModified":      "lastModified1",
					},
				},
			},
			map[string]interface{}{
				"embeds": []interface{}{
					map[string]interface{}{
						"id":                "1",
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description2",
						"lastModified":      "lastModified2",
					},
				},
			},
			map[string]interface{}{
				"embeds": []interface{}{
					map[string]interface{}{
						"id":                "1",
						"alternativeImages": map[string]interface{}{},
						"alternativeTitles": map[string]interface{}{},
						"description":       "Description2",
						"lastModified":      "lastModified2",
					},
				},
			},
		},
	}

	for _, row := range data {
		res := mergeParts([]responsePart{{content: row.content}, {content: row.component}}, "http://test.api.ft.com/content/")
		assert.True(t, reflect.DeepEqual(row.mergedContent, res), "Expected and actual merged content differs.\n Expected: %v\n Actual %v\n", row.mergedContent, res)
	}
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
		res := mergeParts([]responsePart{{content: row.content}, {content: row.component}}, "http://test.api.ft.com/content/")
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

	contentJson, e := ioutil.ReadFile("test-resources/embedded-enrichedcontent-output.json")
	assert.Nil(t, e, "Couldn't read enrichedcontent json")

	internalComponentJson, e := ioutil.ReadFile("test-resources/embedded-internalcomponents-output.json")
	assert.Nil(t, e, "Couldn't read internalcomponents json")

	var content, internalComponent map[string]interface{}

	err := json.Unmarshal(contentJson, &content)
	assert.Equal(t, nil, err, "Error %v", err)
	err = json.Unmarshal([]byte(internalComponentJson), &internalComponent)
	assert.Equal(t, nil, err, "Error %v", err)

	results := mergeParts([]responsePart{{content: content}, {content: internalComponent}}, "")

	promotionalTitle := results["alternativeTitles"].(map[string]interface{})["promotionalTitle"]
	shortTeaser := results["alternativeTitles"].(map[string]interface{})["shortTeaser"]
	alternativeStandfirsts := results["alternativeStandfirsts"].(map[string]interface{})["promotionalStandfirst"]

	assert.Equal(t, "promo title", promotionalTitle)
	assert.Equal(t, "short teaser", shortTeaser)
	assert.Equal(t, "standfirst", alternativeStandfirsts)
}

func AreEqualJSON(s1, s2 string) (bool, error) {
	var o1 interface{}
	var o2 interface{}

	var err error
	err = json.Unmarshal([]byte(s1), &o1)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 1 :: %s", err.Error())
	}
	err = json.Unmarshal([]byte(s2), &o2)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 2 :: %s", err.Error())
	}

	return reflect.DeepEqual(o1, o2), nil
}

func TestResolvingOverlappingMergesFullContentDC(t *testing.T) {

	contentJson, e := ioutil.ReadFile("test-resources/embedded-enrichedcontent-outputDC.json")
	assert.Nil(t, e, "Couldn't read enrichedcontent json")

	internalComponentJson, e := ioutil.ReadFile("test-resources/embedded-internalcomponents-outputDC.json")
	assert.Nil(t, e, "Couldn't read internalcomponents json")

	expectedContent, e := ioutil.ReadFile("test-resources/expanded-internal-content-api-outputDC.json")
	assert.Nil(t, e, "Couldn't read enrichedcontent json")

	var content, internalComponent map[string]interface{}

	err := json.Unmarshal(contentJson, &content)
	assert.Equal(t, nil, err, "Error %v", err)
	err = json.Unmarshal([]byte(internalComponentJson), &internalComponent)
	assert.Equal(t, nil, err, "Error %v", err)

	results := mergeParts([]responsePart{{content: content}, {content: internalComponent}}, "http://test.api.ft.com/content/")
	jsonResult, errJson := json.Marshal(results)
	assert.Equal(t, nil, errJson, "Error %v", errJson)

	areEqual, e := AreEqualJSON(string(jsonResult), string(expectedContent))
	assert.Equal(t, nil, e, "Error %v", e)
	assert.Equal(t, true, areEqual, "Error %v", areEqual)
}
