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
var excludedAttributes = map[string]bool{"uuid": true, "lastModified": true, "publishReference": true}

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
	event1     event
	content    map[string]interface{}
}

type retriever func(context.Context) responsePart

func (h internalContentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	err := validateUUID(uuid)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		msg, _ := json.Marshal(ErrorMessage{fmt.Sprintf("The given uuid is not valid, err=%v", err)})
		w.Write([]byte(msg))
		return
	}
	tid := transactionidutils.GetTransactionIDFromRequest(r)
	h.log.TransactionStartedEvent(r.RequestURI, tid, uuid)
	ctx := context.WithValue(transactionidutils.TransactionAwareContext(context.Background(), tid), uuidKey, uuid)
	parts := h.asyncRetrievalsAndUnmarshalls(ctx, []retriever{h.getContent, h.getInternalComponents}, uuid, tid)
	for _, p := range parts {
		if !p.isOk {
			w.WriteHeader(p.statusCode)
			return
		}
		defer cleanupResp(p.response, h.log.log)
		if p.event1.err != nil {
			h.handleErrorEvent(w, p.event1, "Error while unmarshaling the response body")
			return
		}
	}
	mergedContent := mergeParts(parts)
	resolveTopperImageURLs(mergedContent, h.serviceConfig.envAPIHost)
	resolveLeadImageURLs(mergedContent, h.serviceConfig.envAPIHost)
	resolveRequestUrl(mergedContent, h, uuid)
	resolveApiUrl(mergedContent, h, uuid)
	removeEmptyMapFields(mergedContent)
	resultBytes, _ := json.Marshal(mergedContent)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", h.serviceConfig.cacheControlPolicy)
	w.Write(resultBytes)
	h.metrics.recordResponseEvent()
}

func (h internalContentHandler) asyncRetrievalsAndUnmarshalls(ctx context.Context, getters []retriever, uuid string, tid string) []responsePart {
	responseParts := make([]responsePart, 0, len(getters))
	m := sync.RWMutex{}
	var wg sync.WaitGroup
	wg.Add(len(getters))
	for _, g := range getters {
		go func() {
			part := h.retrieveAndUnmarshall(ctx, wg, g, uuid, tid)
			m.Lock()
			defer m.Unlock()
			responseParts = append(responseParts, part)
			wg.Done()
		}()
	}
	wg.Wait()
	return responseParts
}

func (h internalContentHandler) retrieveAndUnmarshall(ctx context.Context, wg sync.WaitGroup, r retriever, uuid string, tid string) responsePart {
	defer wg.Done()
	part := r(ctx)
	part.event1 = event{
		h.serviceConfig.contentSourceAppName,
		extractRequestURL(part.response),
		tid,
		nil,
		uuid,
	}
	part.content, part.event1.err = unmarshalToMap(part.response)
	return part
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

func mergeParts(parts []responsePart) map[string]interface{} {
	content := make(map[string]interface{})
	for _, p := range parts {
		for key, value := range p.content {
			_, found := excludedAttributes[key]
			if !found {
				content[key] = value
			}
		}
	}
	return content
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

func (h internalContentHandler) getContent(ctx context.Context) responsePart {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", h.serviceConfig.contentSourceURI, uuid)
	transactionID, _ := transactionidutils.GetTransactionIDFromContext(ctx)
	h.log.RequestEvent(h.serviceConfig.contentSourceAppName, requestURL, transactionID, uuid)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		h.handleError(err, h.serviceConfig.contentSourceAppName, requestURL, req.Header.Get(transactionidutils.TransactionIDHeader), uuid)
		return responsePart{isOk: false, statusCode: http.StatusInternalServerError}
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
		return responsePart{isOk: false, statusCode: http.StatusInternalServerError}
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
		return responsePart{isOk: false, statusCode: http.StatusServiceUnavailable}
	}
	switch extResp.StatusCode {
	case http.StatusOK:
		h.log.ResponseEvent(appName, req.URL.String(), extResp, uuid)
		return responsePart{isOk: true, statusCode: http.StatusOK, response: extResp}
	case http.StatusNotFound:
		if doFail {
			h.handleNotFound(extResp, appName, req.URL.String(), uuid)
			return responsePart{isOk: false, statusCode: http.StatusNotFound}
		}
		h.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
		h.metrics.recordRequestFailedEvent()
		return responsePart{isOk: true, statusCode: http.StatusNotFound}

	default:
		if doFail {
			h.handleFailedRequest(extResp, appName, req.URL.String(), uuid)
			return responsePart{isOk: false, statusCode: http.StatusServiceUnavailable}
		}
		h.log.RequestFailedEvent(appName, req.URL.String(), extResp, uuid)
		h.metrics.recordRequestFailedEvent()
		return responsePart{isOk: true, statusCode: http.StatusOK}
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
