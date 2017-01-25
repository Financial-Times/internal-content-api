package main

import (
	"bytes"
	"encoding/json"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var internalContentApi *httptest.Server
var enrichedContentApiMock *httptest.Server
var documentStoreApiMock *httptest.Server

func startEnrichedContentApiMock(status string) {
	router := mux.NewRouter();
	var getContent http.HandlerFunc
	var health http.HandlerFunc

	if status == "happy" {
		getContent = happyEnrichedContentApiMock
		health = happyHandler

	} else if status == "notFound" {
		getContent = notFoundHandler
		health = happyHandler
	} else {
		getContent = internalErrorHandler
		health = internalErrorHandler
	}

	router.Path("/enrichedcontent/{uuid}").Handler(handlers.MethodHandler{"GET" : http.HandlerFunc(getContent)})
	router.Path("/__health").Handler(handlers.MethodHandler{"GET" : http.HandlerFunc(health)})

	enrichedContentApiMock = httptest.NewServer(router);
}

func happyEnrichedContentApiMock(writer http.ResponseWriter, request *http.Request) {
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

func startDocumentStoreApiMock(status string) {
	router := mux.NewRouter();
	var getContent http.HandlerFunc
	var health http.HandlerFunc

	if status == "happy" {
		getContent = happyDocumentStoreApiMock
		health = happyHandler

	} else if status == "notFound" {
		getContent = notFoundHandler
		health = happyHandler
	} else {
		getContent = internalErrorHandler
		health = internalErrorHandler
	}

	router.Path("/internalcomponents/{uuid}").Handler(handlers.MethodHandler{"GET" : http.HandlerFunc(getContent)})
	router.Path("/__health").Handler(handlers.MethodHandler{"GET" : http.HandlerFunc(health)})

	documentStoreApiMock = httptest.NewServer(router);
}

func happyDocumentStoreApiMock(writer http.ResponseWriter, request *http.Request) {
	file, err := os.Open("test-resources/document-store-api-output.json")
	if err != nil {
		return
	}
	defer file.Close()
	io.Copy(writer, file)
}

func stopServices() {
	internalContentApi.Close()
	enrichedContentApiMock.Close()
	documentStoreApiMock.Close()
}

func startInternalContentService() {
	enrichedContentApiUri := enrichedContentApiMock.URL + "/enrichedcontent/"
	enrichedContentApiHealthUri := enrichedContentApiMock.URL + "/__health"
	documentStoreApiUri := documentStoreApiMock.URL + "/internalcomponents/"
	documentStoreApiHealthUri := documentStoreApiMock.URL + "/__health"

	sc := ServiceConfig{
		"internal-content-api",
		"8084",
		enrichedContentApiUri,
		documentStoreApiUri,
		"Enriched Content",
		"Document Store",
		enrichedContentApiHealthUri,
		documentStoreApiHealthUri,
		"",
		"",
	}

	appLogger := NewAppLogger()
	metricsHandler := NewMetrics()
	contentHandler := ContentHandler{&sc, appLogger, &metricsHandler}

	h := setupServiceHandler(sc, metricsHandler, contentHandler)

	internalContentApi = httptest.NewServer(h)
}

func getStringFromReader(r io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	return buf.String()
}

func TestShouldReturn200AndInternalComponentOutput(t *testing.T) {
	startEnrichedContentApiMock("happy")
	startDocumentStoreApiMock("happy")
	startInternalContentService()
	defer stopServices()
	resp, err := http.Get(internalContentApi.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/full-internal-content-api-output.json")
	defer file.Close()

	expectedOutput := getStringFromReader(file)
	actualOutput := getStringFromReader(resp.Body)

	assert.Equal(t, expectedOutput, actualOutput, "Response body shoud be equal to transformer response body")
}

func TestShouldReturn404(t *testing.T) {
	startEnrichedContentApiMock("notFound")
	startDocumentStoreApiMock("notFound")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentApi.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Response status should be 404")
}

func TestShouldReturn200AndPartialInternalComponentOutputWhenDocumentNotFound(t *testing.T) {
	startEnrichedContentApiMock("happy")
	startDocumentStoreApiMock("notFound")
	startInternalContentService()
	defer stopServices()
	resp, err := http.Get(internalContentApi.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/partial-internal-content-api-output.json")
	defer file.Close()

	expectedOutput := getStringFromReader(file)
	actualOutput := getStringFromReader(resp.Body)

	assert.Equal(t, expectedOutput, actualOutput, "Response body shoud be equal to transformer response body")
}

func TestShouldReturn200AndPartialInternalComponentOutputWhenDocumentFailed(t *testing.T) {
	startEnrichedContentApiMock("happy")
	startDocumentStoreApiMock("unhappy")
	startInternalContentService()
	defer stopServices()
	resp, err := http.Get(internalContentApi.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	file, _ := os.Open("test-resources/partial-internal-content-api-output.json")
	defer file.Close()

	expectedOutput := getStringFromReader(file)
	actualOutput := getStringFromReader(resp.Body)

	assert.Equal(t, expectedOutput, actualOutput, "Response body shoud be equal to transformer response body")
}

func TestShouldReturn503whenEnrichedContentApiIsNotAvailable(t *testing.T) {
	startEnrichedContentApiMock("unhappy")
	startDocumentStoreApiMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentApi.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")

	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "Response status should be 503")
}

func TestShouldBeHealthy(t *testing.T) {
	startEnrichedContentApiMock("happy")
	startDocumentStoreApiMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentApi.URL + "/__health")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	var res fthealth.HealthResult

	json.NewDecoder(resp.Body).Decode(&res)

	assert.Equal(t, true, res.Ok, "The service should be healthy")
}

func TestShouldBeUnhealthyWhenMethodeApiIsNotHappy(t *testing.T) {
	startEnrichedContentApiMock("unhappy")
	startDocumentStoreApiMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentApi.URL + "/__health")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	var res fthealth.HealthResult

	json.NewDecoder(resp.Body).Decode(&res)

	assert.Equal(t, false, res.Ok, "The service should be unhealthy")

	for i := 0; i < len(res.Checks); i++ {
		switch res.Checks[i].Name {
		case "Enriched Content Availabililty Check":
			assert.Equal(t, false, res.Checks[i].Ok, "The Enriched Content should be unhealthy")
		case "Document Store Availabililty Check":
			assert.Equal(t, true, res.Checks[i].Ok, "The Document Store should be healthy")
		default:
			panic("Not a valid check")
		}
	}
}

func TestShouldBeUnhealthyWhenTransformerIsNotHappy(t *testing.T) {
	startEnrichedContentApiMock("happy")
	startDocumentStoreApiMock("unhappy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentApi.URL + "/__health")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status should be 200")

	var res fthealth.HealthResult

	json.NewDecoder(resp.Body).Decode(&res)

	assert.Equal(t, false, res.Ok, "The service should be unhealthy")

	for i := 0; i < len(res.Checks); i++ {
		switch res.Checks[i].Name {
		case "Enriched Content Availabililty Check":
			assert.Equal(t, true, res.Checks[i].Ok, "The Enriched Content should be unhealthy")
		case "Document Store Availabililty Check":
			assert.Equal(t, false, res.Checks[i].Ok, "The Document Store should be healthy")
		default:
			panic("Not a valid check")
		}
	}

}
