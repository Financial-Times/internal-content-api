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
		"8084",
		"internalcontent",
		"no-store",
		enrichedContentAPIURI,
		documentStoreAPIURI,
		"Enriched Content",
		"Document Store",
		enrichedContentAPIHealthURI,
		documentStoreAPIHealthURI,
		"panic guide",
		"panic guide",
		"Source app business impact",
		"Internal components app business impact",
		"api.ft.com",
		"",
		"",
	}

	appLogger := newAppLogger()
	metricsHandler := NewMetrics()
	contentHandler := contentHandler{&sc, appLogger, &metricsHandler}

	h := setupServiceHandler(sc, metricsHandler, contentHandler)

	internalContentAPI = httptest.NewServer(h)
}

func getStringFromReader(r io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	return buf.String()
}

func TestShouldReturn200AndInternalComponentOutput(t *testing.T) {
	startEnrichedContentAPIMock("happy")
	startDocumentStoreAPIMock("happy")
	startInternalContentService()
	defer stopServices()
	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
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
	startEnrichedContentAPIMock("notFound")
	startDocumentStoreAPIMock("notFound")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
	if err != nil {
		panic(err)
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
	startEnrichedContentAPIMock("happy")
	startDocumentStoreAPIMock("unhappy")
	startInternalContentService()
	defer stopServices()
	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")
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
	startEnrichedContentAPIMock("unhappy")
	startDocumentStoreAPIMock("happy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/internalcontent/5c3cae78-dbef-11e6-9d7c-be108f1c1dce")

	if err != nil {
		panic(err)
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
		panic(err)
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
	startEnrichedContentAPIMock("happy")
	startDocumentStoreAPIMock("unhappy")
	startInternalContentService()
	defer stopServices()

	resp, err := http.Get(internalContentAPI.URL + "/__health")
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
