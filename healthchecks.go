package main

import (
	"errors"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"net/http"
)

func (sc *ServiceConfig) enrichedContentAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   "No articles would be available",
		Name:             sc.enrichedContentAppName + " Availabililty Check",
		PanicGuide:       sc.enrichedContentAppPanicGuide,
		Severity:         1,
		TechnicalSummary: "Checks that " + sc.enrichedContentAppName + " Service is reachable. Internal Content Service requests enriched content from " + sc.enrichedContentAppName + " service.",
		Checker: func() (string, error) {
			return checkServiceAvailability(sc.enrichedContentAppName, sc.enrichedContentAppHealthUri)
		},
	}
}

func (sc *ServiceConfig) documentStoreAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   "Articles won't have the internal component",
		Name:             sc.documentStoreAppName + " Availabililty Check",
		PanicGuide:       sc.documentStoreAppPanicGuide,
		Severity:         2,
		TechnicalSummary: "Checks that " + sc.documentStoreAppName + " Service is reachable. Internal Content Service relies on " + sc.documentStoreAppName + " service to get the internal components.",
		Checker: func() (string, error) {
			return checkServiceAvailability(sc.documentStoreAppName, sc.documentStoreAppHealthUri)
		},
	}
}

func checkServiceAvailability(serviceName string, healthUri string) (string, error) {
	req, err := http.NewRequest("GET", healthUri, nil)
	resp, err := client.Do(req)
	if err != nil {
		msg := fmt.Sprintf("%s service is unreachable: %v", serviceName, err)
		return msg, errors.New(msg)
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("%s service is not responding with OK. status=%d", serviceName, resp.StatusCode)
		return msg, errors.New(msg)
	}
	return "Ok", nil
}
