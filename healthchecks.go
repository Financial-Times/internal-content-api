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
	return gtg.FailFastParallelCheck([]gtg.StatusChecker{
		func() gtg.Status {
			return gtgCheck(func() (string, error) {
				return sc.checkServiceAvailability(sc.content.appName, sc.content.appHealthURI)
			})
		},
		func() gtg.Status {
			return gtgCheck(
				func() (string, error) {
					return sc.checkServiceAvailability(sc.internalComponents.appName, sc.internalComponents.appHealthURI)
				})
		},
		func() gtg.Status {
			return gtgCheck(
				func() (string, error) {
					return sc.checkServiceAvailability(sc.contentUnroller.appName, sc.contentUnroller.appHealthURI)
				})
		},
	})()
}

func gtgCheck(handler func() (string, error)) gtg.Status {
	if _, err := handler(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}
	return gtg.Status{GoodToGo: true}
}

func (sc *serviceConfig) Check(e externalService) fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   e.appBusinessImpact,
		Name:             e.appName,
		PanicGuide:       e.appPanicGuide,
		Severity:         e.severity,
		TechnicalSummary: e.appBusinessImpact,
		Checker: func() (string, error) {
			return sc.checkServiceAvailability(e.appName, e.appHealthURI)
		},
	}
}

func (sc *serviceConfig) checkServiceAvailability(serviceName string, healthURI string) (string, error) {
	req, err := http.NewRequest("GET", healthURI, nil)
	if err != nil {
		msg := fmt.Sprintf("%s service is unreachable: %v", serviceName, err)
		return msg, errors.New(msg)
	}
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
