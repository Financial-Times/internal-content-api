package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"

	"bytes"
	"errors"
	"strconv"
	"strings"

	transactionidutils "github.com/Financial-Times/transactionid-utils-go"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	gouuid "github.com/satori/go.uuid"
	"golang.org/x/net/context"
)

const (
	previewSuffix              = "-preview"
	uuidKey         contextKey = "uuid"
	expandImagesKey contextKey = "expandImages"
)

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
	e          event
	content    map[string]interface{}
}

type retriever struct {
	uri           string
	sourceAppName string
	doFail        bool
}

type contextKey string

func (c contextKey) String() string {
	return string(c)
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

	expandImagesParam := r.URL.Query().Get(expandImagesKey.String())
	expandImages, err := strconv.ParseBool(expandImagesParam)
	if err != nil {
		expandImages = false
	}

	ctx := context.WithValue(transactionidutils.TransactionAwareContext(context.Background(), tid), uuidKey, uuid)
	ctx = context.WithValue(ctx, expandImagesKey, expandImages)

	retrievers := []retriever{
		{h.serviceConfig.contentSourceURI, h.serviceConfig.contentSourceAppName, true},
		{h.serviceConfig.internalComponentsSourceURI, h.serviceConfig.internalComponentsSourceAppName, false},
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

	mergedContent := h.resolveAdditionalFields(ctx, parts)
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
		serviceName:   r.sourceAppName,
		requestURL:    extractRequestURL(resp),
		transactionID: tid,
		uuid:          uuid,
	}
	part.content, part.e.err = unmarshallToMap(resp)
	return part
}

func (h internalContentHandler) resolveAdditionalFields(ctx context.Context, parts []responsePart) map[string]interface{} {
	uuid := ctx.Value(uuidKey).(string)
	mergedContent := mergeParts(parts)
	mergedContent = resolveLeadImages(ctx, mergedContent, h)
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
				// merge values - even if value represents a map
				// Note: no need to check more than 1 level deep for our model/use cases
				if p_map, ok := value.(map[string]interface{}); ok {
					if c_map, ok := content[key].(map[string]interface{}); ok {
						for k2, v2 := range p_map {
							c_map[k2] = v2
						}
					} else {
						content[key] = value
					}
				} else {
					content[key] = value
				}
			}
		}
	}
	return content
}

func resolveLeadImages(ctx context.Context, content map[string]interface{}, h internalContentHandler) map[string]interface{} {
	leadImages, ok := content["leadImages"].([]interface{})
	if !ok {
		return content
	}

	for _, img := range leadImages {
		img, ok := img.(map[string]interface{})
		if !ok {
			continue
		}
		imgURL := "http://" + h.serviceConfig.envAPIHost + "/content/" + img["id"].(string)
		img["id"] = imgURL
	}

	var transformedContent map[string]interface{}
	expandImages := ctx.Value(expandImagesKey).(bool)
	if expandImages {
		var err error
		uuid := ctx.Value(uuidKey).(string)
		transformedContent, err = h.getExpandedImages(ctx, content)
		if err != nil {
			transactionID, _ := transactionidutils.GetTransactionIDFromContext(ctx)
			h.handleError(err, h.serviceConfig.imageResolverAppName, h.serviceConfig.imageResolverSourceURI, transactionID, uuid)
			return content
		}
	} else {
		return content
	}
	return transformedContent
}

func (h internalContentHandler) getExpandedImages(ctx context.Context, content map[string]interface{}) (map[string]interface{}, error) {
	var expandedContent map[string]interface{}
	transactionID, err := transactionidutils.GetTransactionIDFromContext(ctx)
	if err != nil {
		transactionID = transactionidutils.NewTransactionID()
	}
	body, err := json.Marshal(content)
	if err != nil {
		return expandedContent, err
	}

	req, err := http.NewRequest(http.MethodPost, h.serviceConfig.imageResolverSourceURI, bytes.NewReader(body))
	if err != nil {
		return expandedContent, err
	}
	req.Header.Set(transactionidutils.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.serviceConfig.httpClient.Do(req)
	if err != nil {
		return expandedContent, err
	}
	defer resp.Body.Close()

	uuid := ctx.Value(uuidKey).(string)
	if resp.StatusCode != http.StatusOK {
		h.log.RequestFailedEvent(h.serviceConfig.imageResolverAppName, req.URL.String(), resp, uuid)
		h.metrics.recordRequestFailedEvent()
		errMsg := fmt.Sprintf("Received status code %d from %v.", resp.StatusCode, h.serviceConfig.imageResolverAppName)
		return expandedContent, errors.New(errMsg)
	}
	h.log.ResponseEvent(h.serviceConfig.imageResolverAppName, req.URL.String(), resp, uuid)

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return expandedContent, err
	}

	err = json.Unmarshal(body, &expandedContent)
	if err != nil {
		return expandedContent, err
	}

	leadImages, found := expandedContent["leadImages"]
	if !found {
		return expandedContent, errors.New("Cannot find leadImages in response.")
	}

	leadImagesAsArray := (leadImages).([]interface{})
	for i := 0; i < len(leadImagesAsArray); i++ {
		leadImageAsMap := leadImagesAsArray[i].(map[string]interface{})
		transformLeadImage(leadImageAsMap)
	}
	return expandedContent, nil
}

func transformLeadImage(leadImage map[string]interface{}) {
	imageModel, found := leadImage["image"]
	if !found {
		//if image field is not found inside the image, continue the processing
		return
	}

	var apiURL interface{}
	imageModelAsMap := imageModel.(map[string]interface{})
	if apiURL, found = imageModelAsMap["requestUrl"]; !found {
		if apiURL, found = leadImage["id"]; !found {
			return
		}
	}

	apiURLAsString, ok := apiURL.(string)
	if !ok {
		return
	}
	imageModelAsMap["id"] = apiURLAsString
	imageModelAsMap["apiUrl"] = apiURLAsString
	delete(imageModelAsMap, "requestUrl")
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

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		h.handleError(err, r.sourceAppName, requestURL, req.Header.Get(transactionidutils.TransactionIDHeader), uuid)
		return responsePart{isOk: false, statusCode: http.StatusInternalServerError}, nil
	}
	req.Header.Set(transactionidutils.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")

	expandImages, ok := ctx.Value(expandImagesKey).(bool)
	if ok && r.sourceAppName == h.serviceConfig.contentSourceAppName {
		q := req.URL.Query()
		q.Add(expandImagesKey.String(), strconv.FormatBool(expandImages))
		req.URL.RawQuery = q.Encode()
	}

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
