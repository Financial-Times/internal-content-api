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
	responseWriter.Header().Set("Cache-Control", handler.serviceConfig.cacheControlPolicy)

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	var contentIsOK bool
	var contentStatusCode int
	var contentResponse *http.Response
	go func() {
		defer waitGroup.Done()
		contentIsOK, contentStatusCode, contentResponse = handler.getContent(ctx)
	}()

	var internalComponentsAreOK bool
	var internalComponentsStatusCode int
	var internalComponentsResponse *http.Response
	go func() {
		defer waitGroup.Done()
		internalComponentsAreOK, internalComponentsStatusCode, internalComponentsResponse = handler.getInternalComponents(ctx)
	}()

	waitGroup.Wait()
	if !contentIsOK {
		responseWriter.WriteHeader(contentStatusCode)
		return
	}
	defer cleanupResp(contentResponse, handler.log.log)

	if !internalComponentsAreOK {
		responseWriter.WriteHeader(internalComponentsStatusCode)
		return
	}
	defer cleanupResp(internalComponentsResponse, handler.log.log)

	contentEvent := event{
		handler.serviceConfig.contentSourceAppName,
		extractRequestURL(contentResponse),
		transactionID,
		nil,
		uuid,
	}

	internalComponentsEvent := event{
		handler.serviceConfig.internalComponentsSourceAppName,
		extractRequestURL(internalComponentsResponse),
		transactionID,
		nil,
		uuid,
	}

	content, err := unmarshalToMap(contentResponse)
	if err != nil {
		contentEvent.err = err
		handler.handleErrorEvent(responseWriter, contentEvent, "Error while unmarshaling the response body")
		return
	}

	internalComponents, err := unmarshalToMap(internalComponentsResponse)
	if err != nil {
		internalComponentsEvent.err = err
		handler.handleErrorEvent(responseWriter, internalComponentsEvent, "Error while unmarshaling the response body")
		return
	}

	addInternalComponentsToContent(content, internalComponents)
	resolveImageURLs(content, handler.serviceConfig.envAPIHost)
	removeEmptyMapFields(content)

	resultBytes, _ := json.Marshal(content)
	responseWriter.Write(resultBytes)
	handler.metrics.recordResponseEvent()
}

func extractRequestURL(resp *http.Response) string {
	if resp == nil {
		return "N/A"
	}

	return resp.Request.URL.String()
}

func unmarshalToMap(resp *http.Response) (map[string]interface{}, error) {
	var content map[string]interface{}

	if resp == nil {
		return content, nil
	}

	contentBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return content, err
	}
	err = json.Unmarshal(contentBytes, &content)
	if err != nil {
		return content, err
	}

	return content, nil
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
	topper, ok := content["topper"].(map[string]interface{})
	if !ok {
		return
	}

	ii := topper["images"]
	images, ok := ii.([]interface{})
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

func (handler contentHandler) getContent(ctx context.Context) (ok bool, statusCode int, resp *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", handler.serviceConfig.contentSourceURI, uuid)
	transactionID, _ := tid.GetTransactionIDFromContext(ctx)
	handler.log.RequestEvent(handler.serviceConfig.contentSourceAppName, requestURL, transactionID, uuid)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		handler.handleError(err, handler.serviceConfig.contentSourceAppName, requestURL, req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, http.StatusInternalServerError, nil
	}
	req.Header.Set(tid.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)

	return handler.handleResponse(req, resp, err, uuid, handler.serviceConfig.contentSourceAppName, true)
}

func (handler contentHandler) getInternalComponents(ctx context.Context) (ok bool, statusCode int, resp *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", handler.serviceConfig.internalComponentsSourceURI, uuid)
	transactionID, _ := tid.GetTransactionIDFromContext(ctx)
	handler.log.RequestEvent(handler.serviceConfig.internalComponentsSourceAppName, requestURL, transactionID, uuid)

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		handler.handleError(err, handler.serviceConfig.internalComponentsSourceAppName, requestURL, req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, http.StatusInternalServerError, nil
	}
	req.Header.Set(tid.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)

	return handler.handleResponse(req, resp, err, uuid, handler.serviceConfig.internalComponentsSourceAppName, false)
}

func (handler contentHandler) handleResponse(req *http.Request, extResp *http.Response, err error, uuid string, appName string, doFail bool) (ok bool, statusCode int, resp *http.Response) {
	//this happens when hostname cannot be resolved or host is not accessible
	if err != nil {
		handler.handleError(err, appName, req.URL.String(), req.Header.Get(tid.TransactionIDHeader), uuid)
		return false, http.StatusServiceUnavailable, nil
	}
	switch extResp.StatusCode {
	case http.StatusOK:
		handler.log.ResponseEvent(appName, req.URL.String(), extResp, uuid)
		return true, http.StatusOK, extResp
	case http.StatusNotFound:
		if doFail {
			handler.handleNotFound(extResp, appName, req.URL.String(), uuid)
			return false, http.StatusNotFound, nil
		}
		handler.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
		handler.metrics.recordRequestFailedEvent()
		return true, http.StatusNotFound, nil

	default:
		if doFail {
			handler.handleFailedRequest(extResp, appName, req.URL.String(), uuid)
			return false, http.StatusServiceUnavailable, nil
		}
		handler.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
		handler.metrics.recordRequestFailedEvent()
		return true, http.StatusOK, nil

	}
}

func cleanupResp(resp *http.Response, log *logrus.Logger) {
	if resp == nil {
		return
	}

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

func (handler contentHandler) handleError(err error, serviceName string, url string, transactionID string, uuid string) {
	handler.log.ErrorEvent(serviceName, url, transactionID, err, uuid)
	handler.metrics.recordErrorEvent()
}

func (handler contentHandler) handleFailedRequest(resp *http.Response, serviceName string, url string, uuid string) {
	handler.log.RequestFailedEvent(serviceName, url, resp, uuid)
	handler.metrics.recordRequestFailedEvent()
}

func (handler contentHandler) handleNotFound(resp *http.Response, serviceName string, url string, uuid string) {
	handler.log.RequestFailedEvent(serviceName, url, resp, uuid)
	handler.metrics.recordRequestFailedEvent()
}
