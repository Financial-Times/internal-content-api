package main

import (
	"fmt"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"io"
)

const uuidKey = "uuid"

type ContentHandler struct {
	serviceConfig *ServiceConfig
	log           *AppLogger
	metrics       *Metrics
}

func (handler ContentHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	uuid := vars["uuid"]
	handler.log.TransactionStartedEvent(request.RequestURI, tid.GetTransactionIDFromRequest(request), uuid)
	transactionID := request.Header.Get(tid.TransactionIDHeader)
	ctx := tid.TransactionAwareContext(context.Background(), transactionID)
	ctx = context.WithValue(ctx, uuidKey, uuid)
	responseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	success, contentResponse := handler.getEnrichedContent(ctx, responseWriter)
	if !success {
		return
	}
	defer contentResponse.Body.Close()
	success, topperResponse := handler.getInternalComponent(ctx, responseWriter)

	if !success {
		return
	}
	// topper response is nil when is not found
	if (topperResponse == nil) {
		io.Copy(responseWriter, contentResponse.Body)
		return
	}
	defer topperResponse.Body.Close()
	var content map[string]interface{}
	var topper map[string]interface{}

	contentEvent := event{
		"h.serviceConfig.enrichedConentAppName",
		contentResponse.Request.URL.String(),
		transactionID,
		nil,
		uuid,
	}

	topperEvent := event{
		"h.serviceConfig.AppName",
		contentResponse.Request.URL.String(),
		transactionID,
		nil,
		uuid,
	}

	contentBytes, err := ioutil.ReadAll(contentResponse.Body)
	if (err != nil) {
		contentEvent.err = err
		handler.handleErrorEvent(responseWriter, contentEvent, "Error while handling the response body")
		return
	}
	topperBytes, err := ioutil.ReadAll(topperResponse.Body)
	if (err != nil) {
		topperEvent.err = err
		handler.handleErrorEvent(responseWriter, topperEvent, "Error while handling the response body");
		return
	}

	err = json.Unmarshal(contentBytes, &content);
	if (err != nil) {
		contentEvent.err = err
		handler.handleErrorEvent(responseWriter, contentEvent, "Error while parsing the response json")
		return
	}
	err = json.Unmarshal(topperBytes, &topper);
	if (err != nil) {
		topperEvent.err = err
		handler.handleErrorEvent(responseWriter, topperEvent, "Error while parsing the response json");
		return
	}

	content["topper"] = topper["topper"];


	resultBytes, _ := json.Marshal(content);
	responseWriter.Write(resultBytes)
	handler.metrics.recordResponseEvent()
}

func (h ContentHandler) getEnrichedContent(ctx context.Context, w http.ResponseWriter) (ok bool, resp *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestUrl := fmt.Sprintf("%s%s", h.serviceConfig.enrichedContentApiUri, uuid)
	transactionId, _ := tid.GetTransactionIDFromContext(ctx)
	h.log.RequestEvent(h.serviceConfig.enrichedContentAppName, requestUrl, transactionId, uuid)
	req, err := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set(tid.TransactionIDHeader, transactionId)
	//req.Header.Set("Authorization", "Basic "+h.serviceConfig.sourceAppAuth)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)

	return h.handleResponse(req, resp, err, w, uuid, "h.serviceConfig.enrichedContentAppName", true)
}

func (h ContentHandler) getInternalComponent(ctx context.Context, w http.ResponseWriter) (ok bool, resp *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestUrl := fmt.Sprintf("%s%s", h.serviceConfig.documentStoreApiUri, uuid)
	transactionId, _ := tid.GetTransactionIDFromContext(ctx)
	h.log.RequestEvent(h.serviceConfig.documentStoreAppName, requestUrl, transactionId, uuid)

	req, err := http.NewRequest("GET", requestUrl, nil)
	//req.Host = h.serviceConfig.transformAppHostHeader
	req.Header.Set(tid.TransactionIDHeader, transactionId)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)

	return h.handleResponse(req, resp, err, w, uuid, "h.serviceConfig.documentStoreAppName", false)

}

func (h ContentHandler) handleResponse(req *http.Request, extResp *http.Response, err error, w http.ResponseWriter, uuid string, appName string, doFail bool) (ok bool, resp *http.Response) {
	//this happens when hostname cannot be resolved or host is not accessible
	if err != nil {
		h.handleError(w, err, appName, req.URL.String(), req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, nil
	}
	switch extResp.StatusCode {
	case http.StatusOK:
		h.log.ResponseEvent(appName, req.URL.String(), extResp, uuid)
		return true, extResp
	case http.StatusNotFound:
		if (doFail) {
			h.handleNotFound(w, extResp, appName, req.URL.String(), uuid)
			return false, nil
		} else {
			h.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
			h.metrics.recordRequestFailedEvent()
			return true, nil
		}
	default:
		if (doFail) {
			h.handleFailedRequest(w, extResp, appName, req.URL.String(), uuid)
			return false, nil
		} else {
			h.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
			h.metrics.recordRequestFailedEvent()
			return true, nil
		}
	}
}

func (h ContentHandler) handleErrorEvent(w http.ResponseWriter, event event, errMessage string) {
	w.WriteHeader(http.StatusInternalServerError)
	h.log.Error(event, errMessage)
	h.metrics.recordErrorEvent()
}

func (h ContentHandler) handleError(w http.ResponseWriter, err error, serviceName string, url string, transactionId string, uuid string) {
	w.WriteHeader(http.StatusServiceUnavailable)
	h.log.ErrorEvent(serviceName, url, transactionId, err, uuid)
	h.metrics.recordErrorEvent()
}

func (h ContentHandler) handleFailedRequest(w http.ResponseWriter, resp *http.Response, serviceName string, url string, uuid string) {
	w.WriteHeader(http.StatusServiceUnavailable)
	h.log.RequestFailedEvent(serviceName, url, resp, uuid)
	h.metrics.recordRequestFailedEvent()
}

func (h ContentHandler) handleNotFound(w http.ResponseWriter, resp *http.Response, serviceName string, url string, uuid string) {
	w.WriteHeader(http.StatusNotFound)
	h.log.RequestFailedEvent(serviceName, url, resp, uuid)
	h.metrics.recordRequestFailedEvent()
}
