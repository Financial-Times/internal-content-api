package main

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

type mockTransport struct {
	responseStatusCode int
}

func initializeServiceConfig(httpClient *http.Client) *serviceConfig {
	return &serviceConfig{
		contentSourceAppName:"",
		contentSourceAppHealthURI:"",
		internalComponentsSourceAppName:"",
		internalComponentsSourceAppHealthURI:"",
		httpClient,
	}
}

func initializeMockedHTTPClient(responseStatusCode int) *http.Client {
	client := http.DefaultClient
	client.Transport = &mockTransport{
		responseStatusCode: responseStatusCode,
	}

	return client
}

func TestGtgWithReachableDependentServices(t *testing.T) {
	httpClient := initializeMockedHTTPClient(http.StatusOK)
	sc := initializeServiceConfig(httpClient)
	gtgStatus := sc.gtgCheck()
	assert.Equal(t, true, gtgStatus.GoodToGo)
}
