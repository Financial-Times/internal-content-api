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
	previewSuffix               = "-preview"
	uuidKey          contextKey = "uuid"
	unrollContentKey contextKey = "unrollContent"
)

var internalComponentsFilter = map[string]interface{}{
	"id":               "",
	"uuid":             "",
	"lastModified":     "",
	"publishReference": "",
}

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

	unrollContentParam := r.URL.Query().Get(unrollContentKey.String())
	unrollContent, err := strconv.ParseBool(unrollContentParam)
	if err != nil {
		unrollContent = false
	}

	ctx := context.WithValue(transactionidutils.TransactionAwareContext(context.Background(), tid), uuidKey, uuid)
	ctx = context.WithValue(ctx, unrollContentKey, unrollContent)

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

func resolveDC(ctx context.Context, content map[string]interface{}, h internalContentHandler) map[string]interface{} {
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
	unrollContent := ctx.Value(unrollContentKey).(bool)
	if unrollContent {
		var err error
		content["id"] = content["uuid"]
		delete(content, "uuid")

		uuid := ctx.Value(uuidKey).(string)
		transformedContent, err = h.getUnrolledContent(ctx, content)
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

func (h internalContentHandler) resolveAdditionalFields(ctx context.Context, parts []responsePart) map[string]interface{} {
	uuid := ctx.Value(uuidKey).(string)
	parts[1].content = resolveDC(ctx, parts[1].content, h)
	parts[1].content = filterKeys(parts[1].content, internalComponentsFilter)
	baseUrl := "http://" + h.serviceConfig.envAPIHost + "/content/"
	mergedContent := mergeParts(parts, baseUrl)
	resolveRequestUrl(mergedContent, h, uuid)
	resolveApiUrl(mergedContent, h, uuid)
	removeEmptyMapFields(mergedContent)
	return mergedContent
}

func filterKeys(m map[string]interface{}, filter map[string]interface{}) map[string]interface{} {
	for key, valueInFilter := range filter {
		foundValInM, foundInM := m[key]
		if foundInM {
			mapInM, isMapInM := foundValInM.(map[string]interface{})
			mapInFilter, isMapInFilter := valueInFilter.(map[string]interface{})
			if isMapInM && isMapInFilter {
				m[key] = filterKeys(mapInM, mapInFilter)
			} else {
				delete(m, key)
			}
		}
	}
	return m
}

func mergeParts(parts []responsePart, baseUrl string) map[string]interface{} {
	if len(parts) == 0 {
		return make(map[string]interface{})
	}
	if len(parts) == 1 {
		return parts[0].content
	}

	contents := make([]map[string]interface{}, len(parts))
	for i, p := range parts {
		contents[i] = p.content
	}

	for i := 1; i < len(contents); i++ {
		contents[0] = mergeTwoContents(contents[0], contents[i], baseUrl)
	}
	return contents[0]
}

func fixIds(valueMap map[string]interface{}, baseUrl string) {
	uuid, ok := valueMap["uuid"]
	if ok {
		return
	}
	idURL := baseUrl + uuid.(string)
	valueMap["id"] = idURL
	delete(valueMap, "uuid")
}

func mergeTwoEmbeds(a []interface{}, b []interface{}, baseUrl string) []interface{} {
	//	fmt.Println("mergeTwoEmbeds")
	for _, valueInB := range b {
		valueMapB, isMapInB := valueInB.(map[string]interface{})
		if isMapInB {
			fixIds(valueMapB, baseUrl)
			if len(a) == 0 {
				a = append(a, valueMapB)
			} else {
				for aKey, valueInA := range a {
					valueMapA, isMapInA := valueInA.(map[string]interface{})
					if isMapInA {
						fixIds(valueMapA, baseUrl)
						if valueMapB["id"] == valueMapA["id"] {
							//							fmt.Println("mergeTwoContents", valueMapA, valueMapB)
							a[aKey] = mergeTwoContents(valueMapA, valueMapB, baseUrl)
						} else {
							//							fmt.Println("append", a, valueMapB)
							a = append(a, valueMapB)
						}
					}
				}
			}
		}
	}
	return a
}

func mergeTwoContents(a map[string]interface{}, b map[string]interface{}, baseUrl string) map[string]interface{} {
	for key, valueInB := range b {
		foundValInA, foundInA := a[key]
		if foundInA {
			if key == "embeds" {
				arrInA, isArrInA := foundValInA.([]interface{})
				arrInB, isArrInB := valueInB.([]interface{})
				if isArrInA && isArrInB {
					a[key] = mergeTwoEmbeds(arrInA, arrInB, baseUrl)
				} else {
					a[key] = valueInB
				}
			} else {
				mapInA, isMapInA := foundValInA.(map[string]interface{})
				mapInB, isMapInB := valueInB.(map[string]interface{})
				if isMapInA && isMapInB {
					a[key] = mergeTwoContents(mapInA, mapInB, baseUrl)
				} else {
					a[key] = valueInB
				}
			}
		} else {
			a[key] = valueInB
		}
	}
	return a
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
	unrollContent := ctx.Value(unrollContentKey).(bool)
	if unrollContent {
		var err error
		uuid := ctx.Value(uuidKey).(string)
		transformedContent, err = h.getUnrolledContent(ctx, content)
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

func (h internalContentHandler) getUnrolledContent(ctx context.Context, content map[string]interface{}) (map[string]interface{}, error) {
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

	unrollContent, ok := ctx.Value(unrollContentKey).(bool)
	if ok && r.sourceAppName == h.serviceConfig.contentSourceAppName {
		q := req.URL.Query()
		q.Add(unrollContentKey.String(), strconv.FormatBool(unrollContent))
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
