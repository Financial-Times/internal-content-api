package main

import (
	"encoding/json"
	"fmt"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
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
	var contentIsOK bool
	var contentResponse *http.Response
	go func() {
		defer waitGroup.Done()
		contentIsOK, contentResponse = handler.getContent(ctx, responseWriter)
	}()

	var internalComponentsAreOK bool
	var internalComponentsResponse *http.Response
	go func() {
		defer waitGroup.Done()
		internalComponentsAreOK, internalComponentsResponse = handler.getInternalComponents(ctx, responseWriter)
	}()

	waitGroup.Wait()
	if !contentIsOK {
		return
	}
	defer cleanupResp(contentResponse, handler.log.log)

	if !internalComponentsAreOK {
		return
	}
	// internal components response is nil when is not found
	if internalComponentsResponse == nil {
		io.Copy(responseWriter, contentResponse.Body)
		return
	}
	defer cleanupResp(internalComponentsResponse, handler.log.log)

	var content map[string]interface{}
	var internalComponents map[string]interface{}

	contentEvent := event{
		"h.serviceConfig.contentSourceAppName",
		contentResponse.Request.URL.String(),
		transactionID,
		nil,
		uuid,
	}

	internalComponentsEvent := event{
		"h.serviceConfig.internalComponentsSourceAppName",
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
	internalComponentsBytes, err := ioutil.ReadAll(internalComponentsResponse.Body)
	if err != nil {
		internalComponentsEvent.err = err
		handler.handleErrorEvent(responseWriter, internalComponentsEvent, "Error while handling the response body")
		return
	}

	err = json.Unmarshal(contentBytes, &content)
	if err != nil {
		contentEvent.err = err
		handler.handleErrorEvent(responseWriter, contentEvent, "Error while parsing the response json")
		return
	}
	err = json.Unmarshal(internalComponentsBytes, &internalComponents)
	if err != nil {
		internalComponentsEvent.err = err
		handler.handleErrorEvent(responseWriter, internalComponentsEvent, "Error while parsing the response json")
		return
	}

	addInternalComponentsToContent(content, internalComponents)
	resolveImageURLs(content, handler.serviceConfig.envAPIHost)

	resultBytes, _ := json.Marshal(content)
	responseWriter.Write(resultBytes)
	handler.metrics.recordResponseEvent()
}

func addInternalComponentsToContent(content map[string]interface{}, internalComponents map[string]interface{}) {
	excludedAttributes := map[string]bool{"uuid": true, "lastModified": true, "publishReference": true}
	for key, value := range internalComponents {
		_, found := excludedAttributes[key]
		if !found {
			content[key] = value
		}
	}
}

func resolveImageURLs(content map[string]interface{}, APIHost string) {
	topper, found := content["topper"].(map[string]interface{})
	if !found {
		return
	}

	ii := topper["images"]
	images, ok := ii.([]interface{})
	if !ok {
		return
	}
	for i, iimg := range images {
		img, ok := iimg.(map[string]interface{})
		if !ok {
			continue
		}
		imgURL := "http://" + APIHost + "/content/" + img["id"].(string)
		img["id"] = imgURL
		images[i] = img
	}
	topper["images"] = images
}

func (handler contentHandler) getContent(ctx context.Context, w http.ResponseWriter) (ok bool, resp *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", handler.serviceConfig.contentSourceURI, uuid)
	transactionID, _ := tid.GetTransactionIDFromContext(ctx)
	handler.log.RequestEvent(handler.serviceConfig.contentSourceAppName, requestURL, transactionID, uuid)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		handler.handleError(w, err, "h.serviceConfig.contentSourceAppName", "", req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, nil
	}
	req.Header.Set(tid.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)

	return handler.handleResponse(req, resp, err, w, uuid, "h.serviceConfig.contentSourceAppName", true)
}

func (handler contentHandler) getInternalComponents(ctx context.Context, w http.ResponseWriter) (ok bool, resp *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", handler.serviceConfig.internalComponentsSourceURI, uuid)
	transactionID, _ := tid.GetTransactionIDFromContext(ctx)
	handler.log.RequestEvent(handler.serviceConfig.internalComponentsSourceAppName, requestURL, transactionID, uuid)

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		handler.handleError(w, err, "h.serviceConfig.internalComponentsSourceAppName", "", req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, nil
	}
	req.Header.Set(tid.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)

	return handler.handleResponse(req, resp, err, w, uuid, "h.serviceConfig.internalComponentsSourceAppName", false)

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
