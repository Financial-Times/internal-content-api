package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

const uuidKey = "uuid"

type contentHandler struct {
	serviceConfig *serviceConfig
	log           *appLogger
	metrics       *Metrics
}

func (handler contentHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	uuid := vars["uuid"]
	handler.log.TransactionStartedEvent(request.RequestURI, tid.GetTransactionIDFromRequest(request), uuid)
	transactionID := request.Header.Get(tid.TransactionIDHeader)
	ctx := tid.TransactionAwareContext(context.Background(), transactionID)
	ctx = context.WithValue(ctx, uuidKey, uuid)
	responseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	var successContent bool
	var contentResponse *http.Response
	go func() {
		defer waitGroup.Done()
		successContent, contentResponse = handler.getEnrichedContent(ctx, responseWriter)
	}()

	var successTopper bool
	var topperResponse *http.Response
	go func() {
		defer waitGroup.Done()
		successTopper, topperResponse = handler.getInternalComponent(ctx, responseWriter)
	}()

	waitGroup.Wait()
	if !successContent {
		return
	}
	defer cleanupResp(contentResponse, handler.log.log)

	if !successTopper {
		return
	}
	// topper response is nil when is not found
	if topperResponse == nil {
		io.Copy(responseWriter, contentResponse.Body)
		return
	}
	defer cleanupResp(topperResponse, handler.log.log)
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
	if err != nil {
		contentEvent.err = err
		handler.handleErrorEvent(responseWriter, contentEvent, "Error while handling the response body")
		return
	}
	topperBytes, err := ioutil.ReadAll(topperResponse.Body)
	if err != nil {
		topperEvent.err = err
		handler.handleErrorEvent(responseWriter, topperEvent, "Error while handling the response body")
		return
	}

	err = json.Unmarshal(contentBytes, &content)
	if err != nil {
		contentEvent.err = err
		handler.handleErrorEvent(responseWriter, contentEvent, "Error while parsing the response json")
		return
	}
	err = json.Unmarshal(topperBytes, &topper)
	if err != nil {
		topperEvent.err = err
		handler.handleErrorEvent(responseWriter, topperEvent, "Error while parsing the response json")
		return
	}
	content["topper"] = topper["topper"]

	//hack
	resolveImageURLs(topper["topper"].(map[string]interface{}), handler.serviceConfig.envAPIHost)

	resultBytes, _ := json.Marshal(content)
	responseWriter.Write(resultBytes)
	handler.metrics.recordResponseEvent()
}

func resolveImageURLs(topper map[string]interface{}, APIHost string) {
	images, ok := topper["images"].([]interface{})
	if !ok {
		return
	}
	for _, img := range images {
		img, ok := img.(map[string]interface{})
		if !ok {
			continue
		}
		imgURL := "http://" + APIHost + "/content/" + img["id"].(string)
		img["id"] = imgURL
	}
}

func (handler contentHandler) getEnrichedContent(ctx context.Context, w http.ResponseWriter) (ok bool, resp *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", handler.serviceConfig.enrichedContentAPIURI, uuid)
	transactionID, _ := tid.GetTransactionIDFromContext(ctx)
	handler.log.RequestEvent(handler.serviceConfig.enrichedContentAppName, requestURL, transactionID, uuid)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		handler.handleError(w, err, "h.serviceConfig.enrichedContentAppName", "", req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, nil
	}
	req.Header.Set(tid.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)

	return handler.handleResponse(req, resp, err, w, uuid, "h.serviceConfig.enrichedContentAppName", true)
}

func (handler contentHandler) getInternalComponent(ctx context.Context, w http.ResponseWriter) (ok bool, resp *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", handler.serviceConfig.documentStoreAPIURI, uuid)
	transactionID, _ := tid.GetTransactionIDFromContext(ctx)
	handler.log.RequestEvent(handler.serviceConfig.documentStoreAppName, requestURL, transactionID, uuid)

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		handler.handleError(w, err, "h.serviceConfig.documentStoreAppName", "", req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, nil
	}
	req.Header.Set(tid.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)

	return handler.handleResponse(req, resp, err, w, uuid, "h.serviceConfig.documentStoreAppName", false)

}

func (handler contentHandler) handleResponse(req *http.Request, extResp *http.Response, err error, w http.ResponseWriter, uuid string, appName string, doFail bool) (ok bool, resp *http.Response) {
	//this happens when hostname cannot be resolved or host is not accessible
	if err != nil {
		handler.handleError(w, err, appName, req.URL.String(), req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, nil
	}
	switch extResp.StatusCode {
	case http.StatusOK:
		handler.log.ResponseEvent(appName, req.URL.String(), extResp, uuid)
		return true, extResp
	case http.StatusNotFound:
		if doFail {
			handler.handleNotFound(w, extResp, appName, req.URL.String(), uuid)
			return false, nil
		}
		handler.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
		handler.metrics.recordRequestFailedEvent()
		return true, nil

	default:
		if doFail {
			handler.handleFailedRequest(w, extResp, appName, req.URL.String(), uuid)
			return false, nil
		}
		handler.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
		handler.metrics.recordRequestFailedEvent()
		return true, nil

	}
}

func cleanupResp(resp *http.Response, log *logrus.Logger) {
	_, err := io.Copy(ioutil.Discard, resp.Body)
	if err != nil {
		log.Warningf("[%v]", err)
	}
	err = resp.Body.Close()
	if err != nil {
		log.Warningf("[%v]", err)
	}
}

func (handler contentHandler) handleErrorEvent(w http.ResponseWriter, event event, errMessage string) {
	w.WriteHeader(http.StatusInternalServerError)
	handler.log.Error(event, errMessage)
	handler.metrics.recordErrorEvent()
}

func (handler contentHandler) handleError(w http.ResponseWriter, err error, serviceName string, url string, transactionID string, uuid string) {
	w.WriteHeader(http.StatusServiceUnavailable)
	handler.log.ErrorEvent(serviceName, url, transactionID, err, uuid)
	handler.metrics.recordErrorEvent()
}

func (handler contentHandler) handleFailedRequest(w http.ResponseWriter, resp *http.Response, serviceName string, url string, uuid string) {
	w.WriteHeader(http.StatusServiceUnavailable)
	handler.log.RequestFailedEvent(serviceName, url, resp, uuid)
	handler.metrics.recordRequestFailedEvent()
}

func (handler contentHandler) handleNotFound(w http.ResponseWriter, resp *http.Response, serviceName string, url string, uuid string) {
	w.WriteHeader(http.StatusNotFound)
	handler.log.RequestFailedEvent(serviceName, url, resp, uuid)
	handler.metrics.recordRequestFailedEvent()
}
