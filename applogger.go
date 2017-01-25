package main

import (
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/Sirupsen/logrus"
	"net/http"
)

type event struct {
	serviceName   string
	requestUrl    string
	transactionId string
	err           error
	uuid          string
}

type AppLogger struct {
	log *logrus.Logger
}

func NewAppLogger() *AppLogger {
	logrus.SetLevel(logrus.InfoLevel)
	log := logrus.New()
	log.Formatter = new(logrus.JSONFormatter)
	return &AppLogger{log}
}

func (appLogger *AppLogger) ServiceStartedEvent(serviceName string, serviceConfig map[string]interface{}) {
	serviceConfig["event"] = "service_started"
	appLogger.log.WithFields(serviceConfig).Infof("%s started with configuration", serviceName)
}

func (appLogger *AppLogger) TransactionStartedEvent(requestUrl string, transactionId string, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "transaction_started",
		"request_url":    requestUrl,
		"transaction_id": transactionId,
		"uuid":           uuid,
	}).Info()
}

func (appLogger *AppLogger) RequestEvent(serviceName string, requestUrl string, transactionId string, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "request",
		"service_name":   serviceName,
		"request_uri":    requestUrl,
		"transaction_id": transactionId,
		"uuid":           uuid,
	}).Info()
}

func (appLogger *AppLogger) ErrorEvent(serviceName string, requestUrl string, transactionId string, err error, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "error",
		"service_name":   serviceName,
		"request_url":    requestUrl,
		"transaction_id": transactionId,
		"error":          err,
		"uuid":           uuid,
	}).
		Warnf("Cannot reach %s host", serviceName)

}

func (appLogger *AppLogger) Error(event event, errMessage string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "error",
		"service_name":   event.serviceName,
		"request_url":    event.requestUrl,
		"transaction_id": event.transactionId,
		"error":          event.err,
		"uuid":           event.uuid,
	}).
		Warn(errMessage)
}

func (appLogger *AppLogger) RequestFailedEvent(serviceName string, requestUrl string, resp *http.Response, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "request_failed",
		"service_name":   serviceName,
		"request_url":    requestUrl,
		"transaction_id": resp.Header.Get(tid.TransactionIDHeader),
		"status":         resp.StatusCode,
		"uuid":           uuid,
	}).
		Warnf("Request failed. %s responded with %s", serviceName, resp.Status)
}

func (appLogger *AppLogger) ResponseEvent(serviceName string, requestUrl string, resp *http.Response, uuid string) {
	appLogger.log.WithFields(logrus.Fields{
		"event":          "response",
		"service_name":   serviceName,
		"status":         resp.StatusCode,
		"request_url":    requestUrl,
		"transaction_id": resp.Header.Get(tid.TransactionIDHeader),
		"uuid":           uuid,
	}).
		Info("Response from " + serviceName)
}
