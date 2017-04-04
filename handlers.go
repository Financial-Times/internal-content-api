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
	e	   event
	content    map[string]interface{}
}

type retriever struct {
	uri           string
	sourceAppName string
	doFail        bool
}

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

	retrievers := []retriever{
		{h.serviceConfig.contentSourceURI, h.serviceConfig.contentSourceAppName, true},
		{h.serviceConfig.internalComponentsSourceURI, h.serviceConfig.internalComponentsSourceAppName, false },
	}
	parts := h.asyncRetrievalsAndUnmarshalls(ctx, retrievers, uuid, tid)
	for _, p := range parts {
		if !p.isOk {
			w.WriteHeader(p.statusCode)
			return
		}
		if p.e.err != nil {
			h.handleErrorEvent(p.e, "Error while unmarshaling the response body")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	mergedContent := h.resolveAdditionalFields(parts, uuid)

	resultBytes, _ := json.Marshal(mergedContent)
	w.Header().Set("Cache-Control", h.serviceConfig.cacheControlPolicy)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(resultBytes)
	h.metrics.recordResponseEvent()
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

func (h internalContentHandler) asyncRetrievalsAndUnmarshalls(ctx context.Context, retrievers []retriever, uuid string, tid string) []responsePart {
	responseParts := make([]responsePart, len(retrievers), len(retrievers))
	m := sync.RWMutex{}
	var wg sync.WaitGroup
	wg.Add(len(retrievers))
	for i, r := range retrievers {
		go func(i int, r retriever) {
			part := h.retrieveAndUnmarshall(ctx, r, uuid, tid)
			m.Lock()
			defer m.Unlock()
			defer wg.Done()
			responseParts[i] = part
		}(i, r)
	}
	wg.Wait()
	return responseParts
}

func (h internalContentHandler) retrieveAndUnmarshall(ctx context.Context, r retriever, uuid string, tid string) responsePart {
	part, resp := h.callService(ctx, r)
	defer cleanupResp(resp, h.log.log)

	part.e = event{
		serviceName: r.sourceAppName,
		requestURL: extractRequestURL(resp),
		transactionID: tid,
		uuid: uuid,
	}
	part.content, part.e.err = unmarshallToMap(resp)
	return part
}

func (h internalContentHandler) resolveAdditionalFields(parts []responsePart, uuid string) map[string]interface{} {
	mergedContent := mergeParts(parts)
	resolveLeadImageURLs(mergedContent, h.serviceConfig.envAPIHost)
	resolveRequestUrl(mergedContent, h, uuid)
	resolveApiUrl(mergedContent, h, uuid)
	removeEmptyMapFields(mergedContent)
	return mergedContent
}

func mergeParts(parts []responsePart) map[string]interface{} {
	content := parts[0].content
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		for key, value := range p.content {
			_, found := excludedAttributes[key]
			if !found {
				content[key] = value
			}
		}
	}
	return content
}

func resolveLeadImageURLs(content map[string]interface{}, APIHost string) {
	leadImages, ok := content["leadImages"].([]interface{})
	if !ok {
		return
	}

	for _, img := range leadImages {
		img, ok := img.(map[string]interface{})
		if !ok {
			continue
		}
		imgURL := "http://" + APIHost + "/content/" + img["id"].(string)
		img["id"] = imgURL
	}
}

func resolveRequestUrl(content map[string]interface{}, handler internalContentHandler, contentUUID string) {
	content["requestUrl"] = createRequestUrl(handler.serviceConfig.envAPIHost, handler.serviceConfig.handlerPath, contentUUID)
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

func createRequestUrl(APIHost string, handlerPath string, uuid string) string {
	if isPreview(handlerPath) {
		handlerPath = strings.TrimSuffix(handlerPath, previewSuffix)
	}
	return "http://" + APIHost + "/" + handlerPath + "/" + uuid
}

func (h internalContentHandler) callService(ctx context.Context, r retriever) (responsePart, *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", r.uri, uuid)
	transactionID, _ := transactionidutils.GetTransactionIDFromContext(ctx)
	h.log.RequestEvent(r.sourceAppName, requestURL, transactionID, uuid)

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		h.handleError(err, r.sourceAppName, requestURL, req.Header.Get(transactionidutils.TransactionIDHeader), uuid)
		return responsePart{isOk: false, statusCode: http.StatusInternalServerError}, nil
	}
	req.Header.Set(transactionidutils.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.serviceConfig.httpClient.Do(req)
	//this happens when hostname cannot be resolved or host is not accessible
	if err != nil {
		h.handleError(err, r.sourceAppName, req.URL.String(), req.Header.Get(transactionidutils.TransactionIDHeader), uuid)
		return responsePart{isOk: false, statusCode: http.StatusServiceUnavailable}, nil
	}

	return h.handleResponse(req, resp, uuid, r.sourceAppName, r.doFail), resp
}

func (h internalContentHandler) handleResponse(req *http.Request, resp *http.Response, uuid string, appName string, doFail bool) responsePart {
	switch resp.StatusCode {

	case http.StatusOK:
		h.log.ResponseEvent(appName, req.URL.String(), resp, uuid)
		return responsePart{isOk: true, statusCode: http.StatusOK}

	case http.StatusNotFound:
		if doFail {
			h.handleNotFound(resp, appName, req.URL.String(), uuid)
			return responsePart{isOk: false, statusCode: http.StatusNotFound}
		}
		h.log.RequestFailedEvent(appName, req.URL.String(), resp, uuid)
		h.metrics.recordRequestFailedEvent()
		return responsePart{isOk: true, statusCode: http.StatusNotFound}

	default:
		if doFail {
			h.handleFailedRequest(resp, appName, req.URL.String(), uuid)
			return responsePart{isOk: false, statusCode: http.StatusServiceUnavailable}
		}
		h.log.RequestFailedEvent(appName, req.URL.String(), resp, uuid)
		h.metrics.recordRequestFailedEvent()
		return responsePart{isOk: true, statusCode: http.StatusOK}
	}
}

func unmarshallToMap(resp *http.Response) (map[string]interface{}, error) {
	var content map[string]interface{}

	if resp == nil || resp.StatusCode != http.StatusOK {
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

func extractRequestURL(resp *http.Response) string {
	if resp == nil {
		return "N/A"
	}

	return resp.Request.URL.String()
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

func (h internalContentHandler) handleErrorEvent(event event, errMessage string) {
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