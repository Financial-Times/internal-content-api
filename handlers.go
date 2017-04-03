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
	"github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"strings"
)

const uuidKey = "uuid"
const previewSuffix = "-preview"

type contentHandler struct {
	serviceConfig *serviceConfig
	log           *appLogger
	metrics       *Metrics
}

type ErrorMessage struct {
	Message string `json:"message"`
}

func (handler contentHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	contentUUID := vars["uuid"]
	err := validateUUID(contentUUID)
	if err != nil {
		responseWriter.WriteHeader(http.StatusBadRequest)
		msg, _ := json.Marshal(ErrorMessage{fmt.Sprintf("The given uuid is not valid, err=%v", err)})
		responseWriter.Write([]byte(msg))
		return
	}

	transactionID := tid.GetTransactionIDFromRequest(request)
	handler.log.TransactionStartedEvent(request.RequestURI, transactionID, contentUUID)

	ctx := tid.TransactionAwareContext(context.Background(), transactionID)
	ctx = context.WithValue(ctx, uuidKey, contentUUID)

	responseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	responseWriter.Header().Set("Cache-Control", handler.serviceConfig.cacheControlPolicy)

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	var contentIsOK bool
	var contentStatusCode int
	var content map[string]interface{}
	go func() {
		defer waitGroup.Done()
		contentIsOK, contentStatusCode, content = handler.getContent(ctx)
	}()

	var internalComponentsAreOK bool
	var internalComponentsStatusCode int
	var internalComponents map[string]interface{}
	go func() {
		defer waitGroup.Done()
		internalComponentsAreOK, internalComponentsStatusCode, internalComponents = handler.getInternalComponents(ctx)
	}()

	waitGroup.Wait()

	if !contentIsOK {
		responseWriter.WriteHeader(contentStatusCode)
		return
	}

	if !internalComponentsAreOK {
		responseWriter.WriteHeader(internalComponentsStatusCode)
		return
	}

	addInternalComponentsToContent(content, internalComponents)

	resolveTopperImageURLs(content, handler.serviceConfig.envAPIHost)
	resolveLeadImageURLs(content, handler.serviceConfig.envAPIHost)

	resolveRequestUrl(content, handler, contentUUID)
	resolveApiUrl(content, handler, contentUUID)

	removeEmptyMapFields(content)

	resultBytes, _ := json.Marshal(content)
	responseWriter.Write(resultBytes)
	handler.metrics.recordResponseEvent()
}

func validateUUID(contentUUID string) error {
	parsedUUID, err := uuid.FromString(contentUUID)
	if err != nil {
		return err
	}
	if contentUUID != parsedUUID.String() {
		return fmt.Errorf("Parsed UUID (%v) is different than the given uuid (%v).", parsedUUID, contentUUID)
	}
	return nil
}

func resolveRequestUrl(content map[string]interface{}, handler contentHandler, contentUUID string) {
	content["requestUrl"] = createRequestUrl(handler.serviceConfig.envAPIHost, handler.serviceConfig.handlerPath, contentUUID)
}

func createRequestUrl(APIHost string, handlerPath string, uuid string) string {
	if isPreview(handlerPath) {
		handlerPath = strings.TrimSuffix(handlerPath, previewSuffix)
	}
	return "http://" + APIHost + "/" + handlerPath + "/" + uuid
}

func resolveApiUrl(content map[string]interface{}, handler contentHandler, contentUUID string) {
	handlerPath := handler.serviceConfig.handlerPath
	if !isPreview(handlerPath) {
		content["apiUrl"] = createRequestUrl(handler.serviceConfig.envAPIHost, handlerPath, contentUUID)
	}
}

func isPreview(handlerPath string) bool {
	return strings.HasSuffix(handlerPath, previewSuffix)
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

func resolveTopperImageURLs(content map[string]interface{}, APIHost string) {
	topper, ok := content["topper"].(map[string]interface{})
	if !ok {
		return
	}

	ii := topper["images"]
	images, ok := ii.([]interface{})
	if !ok {
		return
	}

	resolveImageURLs(images, APIHost)
}

func resolveLeadImageURLs(content map[string]interface{}, APIHost string) {
	leadImages, ok := content["leadImages"].([]interface{})
	if !ok {
		return
	}

	resolveImageURLs(leadImages, APIHost)
}

func resolveImageURLs(images []interface{}, APIHost string) {
	for _, img := range images {
		img, ok := img.(map[string]interface{})
		if !ok {
			continue
		}
		imgURL := "http://" + APIHost + "/content/" + img["id"].(string)
		img["id"] = imgURL
	}
}

func (handler contentHandler) getContent(ctx context.Context) (bool, int, map[string]interface{}) {
	return handler.callService(ctx, handler.serviceConfig.contentSourceURI, handler.serviceConfig.contentSourceAppName, true)
}

func (handler contentHandler) getInternalComponents(ctx context.Context) (bool, int, map[string]interface{}) {
	return handler.callService(ctx, handler.serviceConfig.internalComponentsSourceURI, handler.serviceConfig.internalComponentsSourceAppName, false)
}

func (handler contentHandler) callService(ctx context.Context, appUri string, appName string, doFail bool) (bool, int, map[string]interface{}) {
	uuid := ctx.Value(uuidKey).(string)
	transactionID, _ := tid.GetTransactionIDFromContext(ctx)

	requestURL := fmt.Sprintf("%s%s", appUri, uuid)
	handler.log.RequestEvent(appName, requestURL, transactionID, uuid)

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		handler.handleError(err, appName, requestURL, transactionID, uuid)
		return false, http.StatusInternalServerError, nil
	}
	req.Header.Set(tid.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := handler.serviceConfig.httpClient.Do(req)
	if err != nil {
		handler.handleError(err, appName, requestURL, transactionID, uuid)
		return false, http.StatusServiceUnavailable, nil
	}

	return handler.handleResponse(req, resp, transactionID, uuid, appName, doFail)
}

func (handler contentHandler) handleResponse(req *http.Request, resp *http.Response, transactionID, uuid, appName string, doFail bool) (bool, int, map[string]interface{}) {
	defer cleanupResp(resp, handler.log.log)

	switch resp.StatusCode {
	case http.StatusOK:
		handler.log.ResponseEvent(appName, req.URL.String(), resp, uuid)
	case http.StatusNotFound:
		if doFail {
			handler.handleNotFound(resp, appName, req.URL.String(), uuid)
			return false, http.StatusNotFound, nil
		}
		handler.log.RequestFailedEvent(appName, req.URL.String(), resp, uuid)
		handler.metrics.recordRequestFailedEvent()
		return true, http.StatusNotFound, map[string]interface{} {}
	default:
		if doFail {
			handler.handleFailedRequest(resp, appName, req.URL.String(), uuid)
			return false, http.StatusServiceUnavailable, nil
		}
		handler.log.RequestFailedEvent(appName, req.URL.String(), resp, uuid)
		handler.metrics.recordRequestFailedEvent()
		return true, http.StatusNotFound, map[string]interface{} {}
	}

	content, err := unmarshalToMap(resp)
	if err != nil {
		errorEvent := event{
			appName,
			req.URL.String(),
			transactionID,
			err,
			uuid,
		}
		handler.handleErrorEvent(errorEvent, "Error while unmarshaling the response body")
		return false, http.StatusInternalServerError, nil
	}

	return true, resp.StatusCode, content
}

func unmarshalToMap(resp *http.Response) (map[string]interface{}, error) {
	content := map[string]interface{} {}

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

func (handler contentHandler) handleErrorEvent(event event, errMessage string) {
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