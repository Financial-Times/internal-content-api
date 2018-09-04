package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var internalContentAPI *httptest.Server
var enrichedContentAPIMock *httptest.Server
var contentPublicReadAPIMock *httptest.Server
var contentUnrollerMock *httptest.Server

func startEnrichedContentAPIMock(status string) {
	router := mux.NewRouter()
	var getContent http.HandlerFunc
	var health http.HandlerFunc

	if status == "happy" {
		getContent = happyEnrichedContentAPIMock
		health = happyHandler
	} else if status == "unrollContent" {
		getContent = unrollContentEnrichedContentAPIMock
		health = happyHandler
	} else if status == "notFound" {
		getContent = notFoundHandler
		health = happyHandler
	} else {
		getContent = internalErrorHandler
		health = internalErrorHandler
	}

	router.Path("/enrichedcontent/{uuid}").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(getContent)})
	router.Path("/__health").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(health)})

	enrichedContentAPIMock = httptest.NewServer(router)
}

func unrollContentEnrichedContentAPIMock(writer http.ResponseWriter, request *http.Request) {
	file, err := os.Open("test-resources/enriched-content-api-output-unrollContent.json")
	if err != nil {
		return
	}
	defer file.Close()
	io.Copy(writer, file)
}

func happyEnrichedContentAPIMock(writer http.ResponseWriter, request *http.Request) {
	file, err := os.Open("test-resources/enriched-content-api-output.json")
	if err != nil {
		return
	}
	defer file.Close()
	io.Copy(writer, file)
}

func internalErrorHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

func badRequestHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
}

func happyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func startContentPublicReadAPIMock(status string) {
	router := mux.NewRouter()
	var getContent http.HandlerFunc
	var health http.HandlerFunc

	if status == "happy" {
		getContent = happyContentPublicReadAPIMock
		health = happyHandler
	} else if status == "unrollContent" {
		getContent = unrollContentPublicReadAPIMock
		health = happyHandler
	} else if status == "notFound" {
		getContent = notFoundHandler
		health = happyHandler
	} else {
		getContent = internalErrorHandler
		health = internalErrorHandler
	}

	router.Path("/internalcontent/{uuid}").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(getContent)})
	router.Path("/__health").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(health)})

	contentPublicReadAPIMock = httptest.NewServer(router)
}

func startContentUnrollerServiceMock(status string) {
	router := mux.NewRouter()
	var getExpandedContent http.HandlerFunc
	var health http.HandlerFunc

	if status == "happy" {
		getExpandedContent = happyContentUnrollerMock
		health = happyHandler
	} else if status == "unrollContent" {
		getExpandedContent = unrollContentContentUnrollerMock
		health = happyHandler
	} else if status == "badRequest" {
		getExpandedContent = badRequestHandler
		health = happyHandler
	} else {
		getExpandedContent = internalErrorHandler
		health = internalErrorHandler
	}

	router.Path("/internalcontent").Handler(handlers.MethodHandler{http.MethodPost: http.HandlerFunc(getExpandedContent)})
	router.Path("/__health").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(health)})

	contentUnrollerMock = httptest.NewServer(router)
}

func happyContentPublicReadAPIMock(writer http.ResponseWriter, request *http.Request) {
	file, err := os.Open("test-resources/content-public-read-output.json")
	if err != nil {
		return
	}
	defer file.Close()
	io.Copy(writer, file)
}

func unrollContentPublicReadAPIMock(writer http.ResponseWriter, request *http.Request) {
	file, err := os.Open("test-resources/content-public-read-output-unrollContent.json")
	if err != nil {
		return
	}
	defer file.Close()
	io.Copy(writer, file)
}

func happyContentUnrollerMock(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open("test-resources/content-unroller-output.json")
	if err != nil {
		return
	}
	defer file.Close()
	io.Copy(w, file)
}

func unrollContentContentUnrollerMock(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open("test-resources/content-unroller-output-unrollContent.json")
	if err != nil {
		return
	}
	defer file.Close()
	io.Copy(w, file)
}

func stopServices() {
	internalContentAPI.Close()
	enrichedContentAPIMock.Close()
	contentPublicReadAPIMock.Close()
}

func startInternalContentService() {
	enrichedContentAPIURI := enrichedContentAPIMock.URL + "/enrichedcontent/"
	enrichedContentAPIHealthURI := enrichedContentAPIMock.URL + "/__health"
	contentPublicReadAPIURI := contentPublicReadAPIMock.URL + "/internalcontent/"
	contentPublicReadAPIHealthURI := contentPublicReadAPIMock.URL + "/__health"
	contentUnrollerURI := contentUnrollerMock.URL + "/internalcontent"
	contentUnrollerHealthURI := contentUnrollerMock.URL + "/__health"
	sc := serviceConfig{
		"internal-content-api",
		"Internal Content API",
		"8084",
		"internalcontent",
		"max-age=10",
		externalService{
			"enriched-content-read-api",
			enrichedContentAPIURI,
			enrichedContentAPIHealthURI,
			"panic guide",
			"Source app business impact",
			1},
		externalService{
			"content-public-read",
			contentPublicReadAPIURI,
			contentPublicReadAPIHealthURI,
			"panic guide",
			"Internal components app business impact",
			2},
		externalService{
			"content-unroller",
			contentUnrollerURI,
			contentUnrollerHealthURI,
			"panic guide",
			"Image resolver app business imapct",
			2},
		"api.ft.com",
		"",
		"",
		http.DefaultClient,
	}

	appLogger := newAppLogger()
	metricsHandler := NewMetrics()
	contentHandler := internalContentHandler{&sc, appLogger, &metricsHandler}

	h := setupServiceHandler(sc, metricsHandler, contentHandler)

	internalContentAPI = httptest.NewServer(h)
}

func getMapFromReader(r io.Reader) map[string]interface{} {
	var m map[string]interface{}
	json.NewDecoder(r).Decode(&m)
	return m
}

func compareResults(expectedOutput map[string]interface{}, actualOutput map[string]interface{}) (bool, error) {
	expectedOutputJSON, errJSON := json.Marshal(expectedOutput)
	if errJSON != nil {
		return false, errJSON
	}
	actualOutputJSON, errJSON := json.Marshal(actualOutput)
	if errJSON != nil {
		return false, errJSON
	}

	areEqual, e := AreEqualJSON(string(expectedOutputJSON), string(actualOutputJSON))
	if e != nil {
		return false, e
	}
	return areEqual, nil
}

func TestShouldReturn200AndInternalComponentOutput(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()
	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/full-internal-content-api-output.json")
	defer file.Close()

	expectedOutput := getMapFromReader(file)
	actualOutput := getMapFromReader(resp.Body)
	areEqual, e := compareResults(expectedOutput, actualOutput)
	assert.Equal(t, nil, e, "Error %v", e)
	assert.Equal(t, true, areEqual, "Error %v", areEqual)
	assert.Equal(t, "max-age=10", resp.Header.Get("Cache-Control"), "Should have cache control set")
}

func TestShouldReturn200WhenUnrollContentIsTrueAndInternalComponentOutput(t *testing.T) {
	startEnrichedContentAPIMock("unrollContent")
	startContentPublicReadAPIMock("unrollContent")
	startContentUnrollerServiceMock("unrollContent")
	startInternalContentService()
	defer stopServices()

	req, err := http.NewRequest(http.MethodGet, internalContentAPI.URL+"/internalcontent/9607cb04-7ac4-11e8-8e17-ed45e46cf554", nil)
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	q := req.URL.Query()
	q.Add("unrollContent", "true")
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/internal-content-api-output-unrollContent.json")
	defer file.Close()

	expectedOutput := getMapFromReader(file)
	actualOutput := getMapFromReader(resp.Body)

	areEqual, e := compareResults(expectedOutput, actualOutput)
	assert.Equal(t, nil, e, "Error %v", e)
	assert.Equal(t, true, areEqual, "Error %v", areEqual)
	assert.Equal(t, "max-age=10", resp.Header.Get("Cache-Control"), "Should have cache control set")
}

func TestShouldReturn200AndInternalComponentOutputWhenUnrollContentReturns400(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("badRequest")
	startInternalContentService()
	defer stopServices()

	req, err := http.NewRequest(http.MethodGet, internalContentAPI.URL+"/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce", nil)
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	q := req.URL.Query()
	q.Add("unrollContent", "true")
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/full-internal-content-api-output.json")
	defer file.Close()

	expectedOutput := getMapFromReader(file)
	actualOutput := getMapFromReader(resp.Body)

	assert.Equal(t, expectedOutput, actualOutput, "Response body shoud be equal to transformer response body")
	assert.Equal(t, "max-age=10", resp.Header.Get("Cache-Control"), "Should have cache control set")
}

func TestShouldReturn200AndInternalComponentOutputWhenUnrollContentReturns500(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("unhappy")
	startInternalContentService()
	defer stopServices()

	req, err := http.NewRequest(http.MethodGet, internalContentAPI.URL+"/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce", nil)
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	q := req.URL.Query()
	q.Add("unrollContent", "true")
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/full-internal-content-api-output.json")
	defer file.Close()

	expectedOutput := getMapFromReader(file)
	actualOutput := getMapFromReader(resp.Body)

	assert.Equal(t, expectedOutput, actualOutput, "Response body shoud be equal to transformer response body")
	assert.Equal(t, "max-age=10", resp.Header.Get("Cache-Control"), "Should have cache control set")
}

func TestShouldReturn404(t *testing.T) {
	startEnrichedContentAPIMock("notFound")
	startContentPublicReadAPIMock("notFound")
	startContentUnrollerServiceMock("badRequest")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Response status should be 404")
}

func TestShouldReturn200AndPartialInternalComponentOutputWhenDocumentNotFound(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("notFound")
	startContentUnrollerServiceMock("badRequest")
	startInternalContentService()
	defer stopServices()
	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/partial-internal-content-api-output.json")
	defer file.Close()

	expectedOutput := getMapFromReader(file)
	actualOutput := getMapFromReader(resp.Body)

	assert.Equal(t, expectedOutput, actualOutput, "Response body shoud be equal to transformer response body")
}

func TestShouldReturn200AndPartialInternalComponentOutputWhenDocumentFailed(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("unhappy")
	startContentUnrollerServiceMock("badRequest")
	startInternalContentService()
	defer stopServices()
	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/partial-internal-content-api-output.json")
	defer file.Close()

	expectedOutput := getMapFromReader(file)
	actualOutput := getMapFromReader(resp.Body)

	assert.Equal(t, expectedOutput, actualOutput, "Response body shoud be equal to transformer response body")
}

func TestShouldReturn503whenEnrichedContentApiIsNotAvailable(t *testing.T) {
	startEnrichedContentAPIMock("unhappy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")

	if err != nil {
		assert.FailNow(t, "Cannot send request to internalcontent endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "Response status should be 503")
}

func TestShouldBeHealthy(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__health")
	if err != nil {
		assert.FailNow(t, "Cannot send request to health endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	var res fthealth.HealthResult

	json.NewDecoder(resp.Body).Decode(&res)

	assert.Equal(t, true, res.Ok, "The service should be healthy")
}

func TestShouldBeUnhealthyWhenMethodeApiIsNotHappy(t *testing.T) {
	startEnrichedContentAPIMock("unhappy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__health")
	if err != nil {
		assert.FailNow(t, "Cannot send request to health endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	var res fthealth.HealthResult

	json.NewDecoder(resp.Body).Decode(&res)

	assert.Equal(t, false, res.Ok, "The service should be unhealthy")

	for i := 0; i < len(res.Checks); i++ {
		switch res.Checks[i].Name {
		case "enriched-content-read-api":
			assert.Equal(t, false, res.Checks[i].Ok, "The Enriched Content should be unhealthy")
		case "content-public-read":
			assert.Equal(t, true, res.Checks[i].Ok, "The Document Store should be healthy")
		case "content-unroller":
			assert.Equal(t, true, res.Checks[i].Ok, "The Image Resolver should be healthy")
		default:
			assert.FailNow(t, "Not a valid check")
		}
	}
}

func TestShouldBeUnhealthyWhenTransformerIsNotHappy(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("unhappy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__health")
	if err != nil {
		assert.FailNow(t, "Cannot send request to health endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	var res fthealth.HealthResult

	json.NewDecoder(resp.Body).Decode(&res)

	assert.Equal(t, false, res.Ok, "The service should be unhealthy")

	for i := 0; i < len(res.Checks); i++ {
		switch res.Checks[i].Name {
		case "enriched-content-read-api":
			assert.Equal(t, true, res.Checks[i].Ok, "The Enriched Content should be unhealthy")
		case "content-public-read":
			assert.Equal(t, false, res.Checks[i].Ok, "The Document Store should be healthy")
		case "content-unroller":
			assert.Equal(t, true, res.Checks[i].Ok, "The Image Resolver should be healthy")
		default:
			assert.FailNow(t, "Not a valid check")
		}
	}

}

func TestShouldBeUnhealthyWhenContentUnrollerIsUnhealthy(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("unhappy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__health")
	if err != nil {
		assert.FailNow(t, "Cannot send request to health endpoint", err.Error())
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	var res fthealth.HealthResult

	json.NewDecoder(resp.Body).Decode(&res)

	assert.Equal(t, false, res.Ok, "The service should be unhealthy")

	for i := 0; i < len(res.Checks); i++ {
		switch res.Checks[i].Name {
		case "enriched-content-read-api":
			assert.Equal(t, true, res.Checks[i].Ok, "The Enriched Content should be healthy")
		case "content-public-read":
			assert.Equal(t, true, res.Checks[i].Ok, "The Document Store should be healthy")
		case "content-unroller":
			assert.Equal(t, false, res.Checks[i].Ok, "The Image Resolver should be unhealthy")
		default:
			assert.FailNow(t, "Not a valid check")
		}
	}
}

func TestShouldBeGoodToGo(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__gtg")
	if err != nil {
		assert.FailNow(t, "Cannot send request to gtg endpoint", err.Error())
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")
}

func TestShouldNotBeGoodToGoWhenMethodeApiIsNotHappy(t *testing.T) {
	startEnrichedContentAPIMock("unhappy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__gtg")
	if err != nil {
		assert.FailNow(t, "Cannot send request to gtg endpoint", err.Error())
	}

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "Response status should be 503")
}

func TestShouldNotBeGoodToGoWhenTransformerIsNotHappy(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("unhappy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__gtg")
	if err != nil {
		assert.FailNow(t, "Cannot send request to gtg endpoint", err.Error())
	}

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "Response status should be 503")
}

func TestShouldNotBeGoodToGoWhenContentUnrollerNotHappy(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("unhappy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__gtg")
	if err != nil {
		assert.FailNow(t, "Cannot send request to gtg endpoint", err.Error())
	}

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "Response status should be 503")
}

func TestShouldReturn400WhenInvalidUUID(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startContentPublicReadAPIMock("happy")
	startContentUnrollerServiceMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/123-invalid-uuid")

	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Response status should be 400")
}

func TestServiceasMap(t *testing.T) {
	sc := serviceConfig{
		"appSystemCode",
		"appName",
		"appPort",
		"handlerPath",
		"cacheControlPolicy",
		externalService{
			"contentSourceAppName",
			"contentSourceURI",
			"contentSourceAppHealthURI",
			"contentSourceAppPanicGuide",
			"contentSourceAppBusinessImpact",
			1},
		externalService{
			"internalComponentsSourceAppName",
			"internalComponentsSourceURI",
			"internalComponentsSourceAppHealthURI",
			"internalComponentsSourceAppPanicGuide",
			"internalComponentsSourceAppBusinessImpact",
			2},
		externalService{
			"contentUnrollerAppName",
			"contentUnrollerSourceURI",
			"contentUnrollerAppHealthURI",
			"contentUnrollerAppPanicGuide",
			"contentUnrollerAppBusinessImpact",
			2},
		"envAPIHost",
		"graphiteTCPAddress",
		"graphitePrefix",
		nil,
	}
	resp := sc.asMap()
	expected := map[string]interface{}{
		"app-system-code":      "appSystemCode",
		"app-name":             "appName",
		"app-port":             "appPort",
		"cache-control-policy": "cacheControlPolicy",
		"handler-path":         "handlerPath",
		"content-source": map[string]interface{}{
			"app-uri":             "contentSourceURI",
			"app-name":            "contentSourceAppName",
			"app-health-uri":      "contentSourceAppHealthURI",
			"app-panic-guide":     "contentSourceAppPanicGuide",
			"app-business-impact": "contentSourceAppBusinessImpact"},
		"internal-components": map[string]interface{}{
			"app-uri":             "internalComponentsSourceURI",
			"app-name":            "internalComponentsSourceAppName",
			"app-health-uri":      "internalComponentsSourceAppHealthURI",
			"app-panic-guide":     "internalComponentsSourceAppPanicGuide",
			"app-business-impact": "internalComponentsSourceAppBusinessImpact"},
		"content-unroller": map[string]interface{}{
			"app-uri":             "contentUnrollerSourceURI",
			"app-name":            "contentUnrollerAppName",
			"app-health-uri":      "contentUnrollerAppHealthURI",
			"app-panic-guide":     "contentUnrollerAppPanicGuide",
			"app-business-impact": "contentUnrollerAppBusinessImpact"},
		"env-api-host":         "envAPIHost",
		"graphite-tcp-address": "graphiteTCPAddress",
		"graphite-prefix":      "graphitePrefix",
	}
	assert.Equal(t, resp, expected, "Wrong return from asMap")
}
