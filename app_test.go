package main

import (
	"encoding/json"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var internalContentAPI *httptest.Server
var enrichedContentAPIMock *httptest.Server
var documentStoreAPIMock *httptest.Server

func startEnrichedContentAPIMock(status string) {
	router := mux.NewRouter()
	var getContent http.HandlerFunc
	var health http.HandlerFunc

	if status == "happy" {
		getContent = happyEnrichedContentAPIMock
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

func happyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func startDocumentStoreAPIMock(status string) {
	router := mux.NewRouter()
	var getContent http.HandlerFunc
	var health http.HandlerFunc

	if status == "happy" {
		getContent = happyDocumentStoreAPIMock
		health = happyHandler

	} else if status == "notFound" {
		getContent = notFoundHandler
		health = happyHandler
	} else {
		getContent = internalErrorHandler
		health = internalErrorHandler
	}

	router.Path("/internalcomponents/{uuid}").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(getContent)})
	router.Path("/__health").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(health)})

	documentStoreAPIMock = httptest.NewServer(router)
}

func happyDocumentStoreAPIMock(writer http.ResponseWriter, request *http.Request) {
	file, err := os.Open("test-resources/document-store-api-output.json")
	if err != nil {
		return
	}
	defer file.Close()
	io.Copy(writer, file)
}

func stopServices() {
	internalContentAPI.Close()
	enrichedContentAPIMock.Close()
	documentStoreAPIMock.Close()
}

func startInternalContentService() {
	enrichedContentAPIURI := enrichedContentAPIMock.URL + "/enrichedcontent/"
	enrichedContentAPIHealthURI := enrichedContentAPIMock.URL + "/__health"
	documentStoreAPIURI := documentStoreAPIMock.URL + "/internalcomponents/"
	documentStoreAPIHealthURI := documentStoreAPIMock.URL + "/__health"
	sc := serviceConfig{
		"internal-content-api",
		"Internal Content API",
		"8084",
		"internalcontent",
		"no-store",
		enrichedContentAPIURI,
		documentStoreAPIURI,
		"enriched-content-read-api",
		"document-store-api",
		enrichedContentAPIHealthURI,
		documentStoreAPIHealthURI,
		"panic guide",
		"panic guide",
		"Source app business impact",
		"Internal components app business impact",
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

func TestShouldReturn200AndInternalComponentOutput(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startDocumentStoreAPIMock("happy")
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

	assert.Equal(t, expectedOutput, actualOutput, "Response body shoud be equal to transformer response body")
}

func TestShouldReturn404(t *testing.T) {
	startEnrichedContentAPIMock("notFound")
	startDocumentStoreAPIMock("notFound")
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
	startDocumentStoreAPIMock("notFound")
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
	startDocumentStoreAPIMock("unhappy")
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
	startDocumentStoreAPIMock("happy")
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
	startDocumentStoreAPIMock("happy")
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
	startDocumentStoreAPIMock("happy")
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
		case "document-store-api":
			assert.Equal(t, true, res.Checks[i].Ok, "The Document Store should be healthy")
		default:
			assert.FailNow(t, "Not a valid check")
		}
	}
}

func TestShouldBeUnhealthyWhenTransformerIsNotHappy(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startDocumentStoreAPIMock("unhappy")
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
		case "document-store-api":
			assert.Equal(t, false, res.Checks[i].Ok, "The Document Store should be healthy")
		default:
			assert.FailNow(t, "Not a valid check")
		}
	}

}

func TestShouldBeGoodToGo(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startDocumentStoreAPIMock("happy")
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
	startDocumentStoreAPIMock("happy")
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
	startDocumentStoreAPIMock("unhappy")
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
	startDocumentStoreAPIMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/123-invalid-uuid")

	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Response status should be 400")
}
