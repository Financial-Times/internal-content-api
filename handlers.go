package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	transactionidutils "github.com/Financial-Times/transactionid-utils-go"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	gouuid "github.com/satori/go.uuid"
	"golang.org/x/net/context"
	"strings"
)

const uuidKey = "uuid"
const previewSuffix = "-preview"

type internalContentHandler struct {
	serviceConfig *serviceConfig
	log           *appLogger
	metrics       *Metrics
}

type ErrorMessage struct {
	Message string `json:"message"`
}

type responsePart struct {
	isOk       bool
	statusCode int
	response   *http.Response
}

func (h internalContentHandler) handleInternalContent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	contentUUID := vars["uuid"]
	err := validateUUID(contentUUID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		msg, _ := json.Marshal(ErrorMessage{fmt.Sprintf("The given uuid is not valid, err=%v", err)})
		w.Write([]byte(msg))
		return
	}

	tid := transactionidutils.GetTransactionIDFromRequest(r)
	h.log.TransactionStartedEvent(r.RequestURI, tid, contentUUID)

	ctx := transactionidutils.TransactionAwareContext(context.Background(), tid)
	ctx = context.WithValue(ctx, uuidKey, contentUUID)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", h.serviceConfig.cacheControlPolicy)

	parts := h.asyncContentAndComponents(ctx)

	if !contentIsOK {
		w.WriteHeader(contentStatusCode)
		return
	}
	defer cleanupResp(contentResponse, h.log.log)

	if !internalComponentsAreOK {
		w.WriteHeader(internalComponentsStatusCode)
		return
	}
	defer cleanupResp(internalComponentsResponse, h.log.log)



	contentEvent := event{
		h.serviceConfig.contentSourceAppName,
		extractRequestURL(contentResponse),
		tid,
		nil,
		contentUUID,
	}

	internalComponentsEvent := event{
		h.serviceConfig.internalComponentsSourceAppName,
		extractRequestURL(internalComponentsResponse),
		tid,
		nil,
		contentUUID,
	}

	content, err := unmarshalToMap(parts[0].response)
	if err != nil {
		contentEvent.err = err
		h.handleErrorEvent(w, contentEvent, "Error while unmarshaling the response body")
		return
	}

	internalComponents, err := unmarshalToMap(parts[1].response)
	if err != nil {
		internalComponentsEvent.err = err
		h.handleErrorEvent(w, internalComponentsEvent, "Error while unmarshaling the response body")
		return
	}

	addInternalComponentsToContent(content, internalComponents)

	resolveTopperImageURLs(content, h.serviceConfig.envAPIHost)
	resolveLeadImageURLs(content, h.serviceConfig.envAPIHost)

	resolveRequestUrl(content, h, contentUUID)
	resolveApiUrl(content, h, contentUUID)

	removeEmptyMapFields(content)

	resultBytes, _ := json.Marshal(content)
	w.Write(resultBytes)
	h.metrics.recordResponseEvent()
}

func (h internalContentHandler) asyncContentAndComponents(ctx context.Context) []responsePart {
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	var contentPart responsePart
	go func() {
		defer waitGroup.Done()
		contentPart = h.getContent(ctx)
	}()
	var internalComponentsPart responsePart
	go func() {
		defer waitGroup.Done()
		internalComponentsPart = h.getInternalComponents(ctx)
	}()
	waitGroup.Wait()
	return []responsePart{contentPart, internalComponentsPart}
}

func (h internalContentHandler) unmarshalls(parts []responsePart, contentUUID string, tid string) []event {
	var events []event
	for _, part := range parts {
		partEvent := event{
			h.serviceConfig.contentSourceAppName,
			extractRequestURL(part.response),
			tid,
			nil,
			contentUUID,
		}
		events = append(events, partEvent)
		// will have to unmarshall here and memorize result and error and events here and return as a struct then above to handle the errors with writes to writer.
	}

	return events
}

func validateUUID(contentUUID string) error {
	parsedUUID, err := gouuid.FromString(contentUUID)
	if err != nil {
		return err
	}
	if contentUUID != parsedUUID.String() {
		return fmt.Errorf("Parsed UUID (%v) is different than the given uuid (%v).", parsedUUID, contentUUID)
	}
	return nil
}

func resolveRequestUrl(content map[string]interface{}, handler internalContentHandler, contentUUID string) {
	content["requestUrl"] = createRequestUrl(handler.serviceConfig.envAPIHost, handler.serviceConfig.handlerPath, contentUUID)
}

func createRequestUrl(APIHost string, handlerPath string, uuid string) string {
	if isPreview(handlerPath) {
		handlerPath = strings.TrimSuffix(handlerPath, previewSuffix)
	}
	return "http://" + APIHost + "/" + handlerPath + "/" + uuid
}

func resolveApiUrl(content map[string]interface{}, handler internalContentHandler, contentUUID string) {
	handlerPath := handler.serviceConfig.handlerPath
	if !isPreview(handlerPath) {
		content["apiUrl"] = createRequestUrl(handler.serviceConfig.envAPIHost, handlerPath, contentUUID)
	}
}

func isPreview(handlerPath string) bool {
	return strings.HasSuffix(handlerPath, previewSuffix)
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

func (h internalContentHandler) getContent(ctx context.Context) (responsePart ) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", h.serviceConfig.contentSourceURI, uuid)
	transactionID, _ := transactionidutils.GetTransactionIDFromContext(ctx)
	h.log.RequestEvent(h.serviceConfig.contentSourceAppName, requestURL, transactionID, uuid)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		h.handleError(err, h.serviceConfig.contentSourceAppName, requestURL, req.Header.Get(transactionidutils.TransactionIDHeader), uuid)
		return responsePart{false, http.StatusInternalServerError, nil}
	}
	req.Header.Set(transactionidutils.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.serviceConfig.httpClient.Do(req)

	return h.handleResponse(req, resp, err, uuid, h.serviceConfig.contentSourceAppName, true)
}

func (h internalContentHandler) getInternalComponents(ctx context.Context) responsePart {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", h.serviceConfig.internalComponentsSourceURI, uuid)
	transactionID, _ := transactionidutils.GetTransactionIDFromContext(ctx)
	h.log.RequestEvent(h.serviceConfig.internalComponentsSourceAppName, requestURL, transactionID, uuid)

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		h.handleError(err, h.serviceConfig.internalComponentsSourceAppName, requestURL, req.Header.Get(transactionidutils.TransactionIDHeader), uuid)
		return responsePart{false, http.StatusInternalServerError, nil}
	}
	req.Header.Set(transactionidutils.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.serviceConfig.httpClient.Do(req)

	return h.handleResponse(req, resp, err, uuid, h.serviceConfig.internalComponentsSourceAppName, false)
}

func (h internalContentHandler) handleResponse(req *http.Request, extResp *http.Response, err error, uuid string, appName string, doFail bool) responsePart {
	//this happens when hostname cannot be resolved or host is not accessible
	if err != nil {
		h.handleError(err, appName, req.URL.String(), req.Header.Get(transactionidutils.TransactionIDHeader), uuid)
		return responsePart{false, http.StatusServiceUnavailable, nil}
	}
	switch extResp.StatusCode {
	case http.StatusOK:
		h.log.ResponseEvent(appName, req.URL.String(), extResp, uuid)
		return responsePart{true, http.StatusOK, extResp}
	case http.StatusNotFound:
		if doFail {
			h.handleNotFound(extResp, appName, req.URL.String(), uuid)
			return responsePart{false, http.StatusNotFound, nil}
		}
		h.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
		h.metrics.recordRequestFailedEvent()
		return responsePart{true, http.StatusNotFound, nil}

	default:
		if doFail {
			h.handleFailedRequest(extResp, appName, req.URL.String(), uuid)
			return responsePart{false, http.StatusServiceUnavailable, nil}
		}
		h.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
		h.metrics.recordRequestFailedEvent()
		return responsePart{true, http.StatusOK, nil}
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

func (h internalContentHandler) handleErrorEvent(w http.ResponseWriter, event event, errMessage string) {
	w.WriteHeader(http.StatusInternalServerError)
	h.log.Error(event, errMessage)
	h.metrics.recordErrorEvent()
}

func (h internalContentHandler) handleError(err error, serviceName string, url string, transactionID string, uuid string) {
	h.log.ErrorEvent(serviceName, url, transactionID, err, uuid)
	h.metrics.recordErrorEvent()
}

func (h internalContentHandler) handleFailedRequest(resp *http.Response, serviceName string, url string, uuid string) {
	h.log.RequestFailedEvent(serviceName, url, resp, uuid)
	h.metrics.recordRequestFailedEvent()
}

func (h internalContentHandler) handleNotFound(resp *http.Response, serviceName string, url string, uuid string) {
	h.log.RequestFailedEvent(serviceName, url, resp, uuid)
	h.metrics.recordRequestFailedEvent()
}
