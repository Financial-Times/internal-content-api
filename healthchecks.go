package main

import (
	"errors"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
	"net/http"
)

func (sc *serviceConfig) gtgCheck() gtg.Status {
	msg, err := sc.checkServiceAvailability(sc.contentSourceAppName, sc.contentSourceAppHealthURI)
	if err != nil {
		return gtg.Status{GoodToGo: false, Message: msg}
	}

	msg, err = sc.checkServiceAvailability(sc.internalComponentsSourceAppName, sc.internalComponentsSourceAppHealthURI)
	if err != nil {
		return gtg.Status{GoodToGo: false, Message: msg}
	}

	msg, err = sc.checkServiceAvailability(sc.imageResolverAppName, sc.imageResolverAppHealthURI)
	if err != nil {
		return gtg.Status{GoodToGo: false, Message: msg}
	}

	return gtg.Status{GoodToGo: true}
}

func (sc *serviceConfig) contentSourceAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   sc.contentSourceAppBusinessImpact,
		Name:             sc.contentSourceAppName,
		PanicGuide:       sc.contentSourceAppPanicGuide,
		Severity:         1,
		TechnicalSummary: "Checks that " + sc.contentSourceAppName + " is reachable. " + sc.appName + " requests content from " + sc.contentSourceAppName,
		Checker: func() (string, error) {
			return sc.checkServiceAvailability(sc.contentSourceAppName, sc.contentSourceAppHealthURI)
		},
	}
}

func (sc *serviceConfig) internalComponentsSourceAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   sc.internalComponentsSourceAppBusinessImpact,
		Name:             sc.internalComponentsSourceAppName,
		PanicGuide:       sc.internalComponentsSourceAppPanicGuide,
		Severity:         2,
		TechnicalSummary: "Checks that " + sc.internalComponentsSourceAppName + " is reachable. " + sc.appName + " relies on " + sc.internalComponentsSourceAppName + " to get the internal components",
		Checker: func() (string, error) {
			return sc.checkServiceAvailability(sc.internalComponentsSourceAppName, sc.internalComponentsSourceAppHealthURI)
		},
	}
}

func (sc *serviceConfig) imageResolverAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   sc.imageResolverAppBusinessImpact,
		Name:             sc.imageResolverAppName,
		PanicGuide:       sc.imageResolverAppPanicGuide,
		Severity:         2,
		TechnicalSummary: "Checks that " + sc.imageResolverAppName + " is reachable. " + sc.appName + " relies on " + sc.imageResolverAppName + " to get the expanded images",
		Checker: func() (string, error) {
			return sc.checkServiceAvailability(sc.imageResolverAppName, sc.imageResolverAppHealthURI)
		},
	}
}

func (sc *serviceConfig) checkServiceAvailability(serviceName string, healthURI string) (string, error) {
	req, err := http.NewRequest("GET", healthURI, nil)
	resp, err := sc.httpClient.Do(req)
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
