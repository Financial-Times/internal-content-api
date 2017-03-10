package main

import (
	"errors"
	"fmt"
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"net/http"
	"github.com/Financial-Times/service-status-go/gtg"
)

func (sc *serviceConfig) gtgCheck() gtg.Status {
	err := sc.contentSourceAppCheck()
	if err != nil {
		return gtg.Status{GoodToGo:false, Message:fmt.Sprintf("%s is not reachable", sc.contentSourceAppName)}
	}

	err = sc.internalComponentsSourceAppCheck()
	if err != nil {
		return gtg.Status{GoodToGo:false, Message:fmt.Sprintf("%s is not reachable", sc.internalComponentsSourceAppName)}
	}

	return gtg.Status{GoodToGo:true}
}

func (sc *serviceConfig) contentSourceAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   sc.contentSourceAppBusinessImpact,
		Name:             sc.contentSourceAppName,
		PanicGuide:       sc.contentSourceAppPanicGuide,
		Severity:         1,
		TechnicalSummary: "Checks that " + sc.contentSourceAppName + " is reachable. " + sc.serviceName + " requests content from " + sc.contentSourceAppName,
		Checker: func() (string, error) {
			return checkServiceAvailability(sc.contentSourceAppName, sc.contentSourceAppHealthURI)
		},
	}
}

func (sc *serviceConfig) internalComponentsSourceAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   sc.internalComponentsSourceAppBusinessImpact,
		Name:             sc.internalComponentsSourceAppName,
		PanicGuide:       sc.internalComponentsSourceAppPanicGuide,
		Severity:         2,
		TechnicalSummary: "Checks that " + sc.internalComponentsSourceAppName + " is reachable. " + sc.serviceName + " relies on " + sc.internalComponentsSourceAppName + " to get the internal components",
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
