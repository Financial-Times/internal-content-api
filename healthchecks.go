package main

import (
	"errors"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"net/http"
)

func (sc *serviceConfig) contentSourceAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   "No articles would be available",
		Name:             sc.contentSourceAppName + " Availabililty Check",
		PanicGuide:       sc.contentSourceAppPanicGuide,
		Severity:         1,
		TechnicalSummary: "Checks that " + sc.contentSourceAppName + " Service is reachable. Internal Content Service requests content from " + sc.contentSourceAppName + " service.",
		Checker: func() (string, error) {
			return checkServiceAvailability(sc.contentSourceAppName, sc.contentSourceAppHealthURI)
		},
	}
}

func (sc *serviceConfig) internalComponentsSourceAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   "Articles won't have the internal components",
		Name:             sc.internalComponentsSourceAppName + " Availabililty Check",
		PanicGuide:       sc.internalComponentsSourceAppPanicGuide,
		Severity:         2,
		TechnicalSummary: "Checks that " + sc.internalComponentsSourceAppName + " Service is reachable. Internal Content Service relies on " + sc.internalComponentsSourceAppName + " service to get the internal components.",
		Checker: func() (string, error) {
			return checkServiceAvailability(sc.internalComponentsSourceAppName, sc.internalComponentsSourceAppHealthURI)
		},
	}
}

func checkServiceAvailability(serviceName string, healthURI string) (string, error) {
	req, err := http.NewRequest("GET", healthURI, nil)
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
