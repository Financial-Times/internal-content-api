package main

import (
	"errors"
	"fmt"
	"net/http"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

// GTG is the HTTP handler function for the Good-To-Go of the methode content placeholder mapper
func (sc *serviceConfig) GTG() gtg.Status {
	contentSourceAppCheck := func() gtg.Status {
		return gtgCheck(sc.contentSourceAppChecker)
	}

	internalComponentsCheck := func() gtg.Status {
		return gtgCheck(sc.internalComponentsSourceAppChecker)
	}

	contentUnrollerAppCheck := func() gtg.Status {
		return gtgCheck(sc.contentUnrollerAppChecker)
	}

	return gtg.FailFastParallelCheck([]gtg.StatusChecker{
		contentSourceAppCheck,
		internalComponentsCheck,
		contentUnrollerAppCheck,
	})()
}

func gtgCheck(handler func() (string, error)) gtg.Status {
	if _, err := handler(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
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
		Checker:          sc.contentSourceAppChecker,
	}
}

func (sc *serviceConfig) internalComponentsSourceAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   sc.internalComponentsSourceAppBusinessImpact,
		Name:             sc.internalComponentsSourceAppName,
		PanicGuide:       sc.internalComponentsSourceAppPanicGuide,
		Severity:         2,
		TechnicalSummary: "Checks that " + sc.internalComponentsSourceAppName + " is reachable. " + sc.appName + " relies on " + sc.internalComponentsSourceAppName + " to get the internal components",
		Checker:          sc.internalComponentsSourceAppChecker,
	}
}

func (sc *serviceConfig) contentUnrollerAppCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   sc.contentUnrollerAppBusinessImpact,
		Name:             sc.contentUnrollerAppName,
		PanicGuide:       sc.contentUnrollerAppPanicGuide,
		Severity:         2,
		TechnicalSummary: "Checks that " + sc.contentUnrollerAppName + " is reachable. " + sc.appName + " relies on " + sc.contentUnrollerAppName + " to get the expanded images",
		Checker:          sc.contentUnrollerAppChecker,
	}
}

func (sc *serviceConfig) contentSourceAppChecker() (string, error) {
	return sc.checkServiceAvailability(sc.contentSourceAppName, sc.contentSourceAppHealthURI)
}

func (sc *serviceConfig) internalComponentsSourceAppChecker() (string, error) {
	return sc.checkServiceAvailability(sc.internalComponentsSourceAppName, sc.internalComponentsSourceAppHealthURI)
}

func (sc *serviceConfig) contentUnrollerAppChecker() (string, error) {
	return sc.checkServiceAvailability(sc.contentUnrollerAppName, sc.contentUnrollerAppHealthURI)
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
