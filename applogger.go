package main

import (
	"net/http"

	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/sirupsen/logrus"
)

type event struct {
	requestURL    string
	transactionID string
	err           error
	uuid          string
}

type appLogger struct {
	log *logrus.Logger
}

func newAppLogger() *appLogger {
	logrus.SetLevel(logrus.InfoLevel)
	log := logrus.New()
	log.Formatter = new(logrus.JSONFormatter)
	return &appLogger{log}
}

func (appLogger *appLogger) ServiceStartedEvent(serviceName string, serviceConfig map[string]interface{}) {
	serviceConfig["event"] = "service_started"
	appLogger.log.WithFields(serviceConfig).Infof("%s started with configuration", serviceName)
}

func (appLogger *appLogger) TransactionStartedEvent(requestURL string, transactionID string, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "transaction_started",
		"request_url":    requestURL,
		"transaction_id": transactionID,
		"uuid":           uuid,
	}).Info()
}

func (appLogger *appLogger) RequestEvent(requestURL string, transactionID string, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "request",
		"request_uri":    requestURL,
		"transaction_id": transactionID,
		"uuid":           uuid,
	}).Info()
}

func (appLogger *appLogger) ErrorEvent(serviceName string, requestURL string, transactionID string, err error, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "error",
		"request_url":    requestURL,
		"transaction_id": transactionID,
		"error":          err,
		"uuid":           uuid,
	}).
		Warnf("Cannot reach %s host", serviceName)

}

func (appLogger *appLogger) Error(event event, errMessage string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "error",
		"request_url":    event.requestURL,
		"transaction_id": event.transactionID,
		"error":          event.err,
		"uuid":           event.uuid,
	}).
		Warn(errMessage)
}

func (appLogger *appLogger) RequestFailedEvent(serviceName string, requestURL string, resp *http.Response, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "request_failed",
		"request_url":    requestURL,
		"transaction_id": resp.Header.Get(tid.TransactionIDHeader),
		"status":         resp.StatusCode,
		"uuid":           uuid,
	}).
		Warnf("Request failed. %s responded with %s", serviceName, resp.Status)
}

func (appLogger *appLogger) ResponseEvent(serviceName string, requestURL string, resp *http.Response, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "response",
		"status":         resp.StatusCode,
		"request_url":    requestURL,
		"transaction_id": resp.Header.Get(tid.TransactionIDHeader),
		"uuid":           uuid,
	}).
		Info("Response from " + serviceName)
}
