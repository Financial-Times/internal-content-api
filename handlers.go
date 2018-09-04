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
	"github.com/gorilla/mux"
	gouuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
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

var embedsComponentsFilter = []string{"requestUrl"}

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

type transformContent func(ctx context.Context, content map[string]interface{}, h internalContentHandler) map[string]interface{}

type retriever struct {
	uri           string
	sourceAppName string
	doFail        bool
	transformContent
}

type contextKey string

func transformContentSourceContent(ctx context.Context, content map[string]interface{}, h internalContentHandler) map[string]interface{} {
	return content
}

func transformInternalComponentsContent(ctx context.Context, content map[string]interface{}, h internalContentHandler) map[string]interface{} {
	transformedContent := h.unrollContent(ctx, content)
	transformedContent = filterKeys(transformedContent, internalComponentsFilter)
	return transformedContent
}

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
		w.Write(msg)
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
		{h.serviceConfig.content.appURI, h.serviceConfig.content.appName, true, transformContentSourceContent},
		{h.serviceConfig.internalComponents.appURI, h.serviceConfig.internalComponents.appName, false, transformInternalComponentsContent},
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
	baseURL := "https://" + h.serviceConfig.envAPIHost + "/content/"
	mergedContent := mergeParts(parts, baseURL)
	mergedContent = h.resolveAdditionalFields(ctx, mergedContent)
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
		return fmt.Errorf("Parsed UUID (%v) is different than the given uuid (%v)", parsedUUID, contentUUID)
	}
	return nil
}

func (h internalContentHandler) asyncRetrievalsAndUnmarshalls(ctx context.Context, retrievers []retriever, uuid string, tid string) []responsePart {
	responseParts := make([]responsePart, len(retrievers))
	m := sync.RWMutex{}
	var wg sync.WaitGroup
	wg.Add(len(retrievers))
	for i, r := range retrievers {
		go func(i int, r retriever) {
			part := h.retrieveAndUnmarshall(ctx, r, uuid, tid)
			m.Lock()
			defer m.Unlock()
			defer wg.Done()
			part.content = r.transformContent(ctx, part.content, h)
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
		requestURL:    extractRequestURL(resp),
		transactionID: tid,
		uuid:          uuid,
	}
	part.content, part.e.err = unmarshallToMap(resp)
	return part
}

func replaceUUID(content map[string]interface{}) {
	recUUID, ok := content["uuid"]
	if ok {
		content["id"] = recUUID
		delete(content, "uuid")
	}
}

func (h internalContentHandler) unrollContent(ctx context.Context, content map[string]interface{}) map[string]interface{} {
	var transformedContent map[string]interface{}
	unrollContent := ctx.Value(unrollContentKey).(bool)
	if !unrollContent {
		return content
	}
	replaceUUID(content)
	var err error
	transformedContent, err = h.getUnrolledContent(ctx, content)
	if err != nil {
		uuid := ctx.Value(uuidKey).(string)
		transactionID, _ := transactionidutils.GetTransactionIDFromContext(ctx)
		h.handleError(err, h.serviceConfig.contentUnroller.appName, h.serviceConfig.contentUnroller.appURI, transactionID, uuid)
		return content
	}
	return transformedContent
}

func (h internalContentHandler) resolveAdditionalFields(ctx context.Context, content map[string]interface{}) map[string]interface{} {
	uuid := ctx.Value(uuidKey).(string)
	resolveRequestURL(content, h, uuid)
	resolveAPIURL(content, h, uuid)
	removeEmptyMapFields(content)
	return content
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

func mergeParts(parts []responsePart, baseURL string) map[string]interface{} {
	if len(parts) == 0 {
		return make(map[string]interface{})
	}
	if len(parts) == 1 {
		return parts[0].content
	}

	contents := make([]map[string]interface{}, len(parts))
	for i, p := range parts {
		if p.content != nil {
			contents[i] = p.content
		} else {
			contents[i] = make(map[string]interface{})
		}
	}

	for i := 1; i < len(contents); i++ {
		contents[0] = mergeTwoContents(contents[0], contents[i], baseURL)
	}
	return contents[0]
}

func extractIDValue(id string) string {
	y := strings.Split(id, "/")
	return y[len(y)-1]
}

func matchIDs(idA string, idB string) bool {
	return extractIDValue(idA) == extractIDValue(idB)
}

func sameIds(idA string, idB string) bool {
	return idA == idB || matchIDs(idA, idB)
}

func filterEmbedsKeys(m map[string]interface{}, filter []string) map[string]interface{} {
	for _, valueInFilter := range filter {
		_, foundInM := m[valueInFilter]
		if foundInM {
			delete(m, valueInFilter)
		}
	}
	return m
}

func transformEmbeds(vMap []interface{}, baseURL string) {
	for _, valueMapB := range vMap {
		valueMap, ok := valueMapB.(map[string]interface{})
		if !ok {
			return
		}
		valueMap = filterEmbedsKeys(valueMap, embedsComponentsFilter)
		id, ok := valueMap["id"]
		if ok {
			valueMap["id"] = baseURL + extractIDValue(id.(string))
		}

		uuid, ok := valueMap["uuid"]
		if ok {
			valueMap["id"] = baseURL + uuid.(string)
			delete(valueMap, "uuid")
		}
	}
}

func mergeTwoEmbeds(a []interface{}, b []interface{}, baseURL string) []interface{} {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	for _, valueInB := range b {
		valueMapB, isMapInB := valueInB.(map[string]interface{})
		if isMapInB {
			lbFound := false
			for aKey, valueInA := range a {
				valueMapA, isMapInA := valueInA.(map[string]interface{})
				if isMapInA {
					if sameIds(valueMapB["id"].(string), valueMapA["id"].(string)) {
						a[aKey] = mergeTwoContents(valueMapA, valueMapB, baseURL)
						lbFound = true
						break
					}
				}
			}
			if !lbFound {
				a = append(a, valueMapB)
			}
		}
	}
	return a
}

func mergeTwoContents(a map[string]interface{}, b map[string]interface{}, baseURL string) map[string]interface{} {
	for key, valueInB := range b {
		foundValInA, foundInA := a[key]
		if foundInA {
			if key == "embeds" {
				arrInA, isArrInA := foundValInA.([]interface{})
				if isArrInA {
					transformEmbeds(arrInA, baseURL)
				}
				arrInB, isArrInB := valueInB.([]interface{})
				if isArrInB {
					transformEmbeds(arrInB, baseURL)
				}

				if isArrInA && isArrInB {
					a[key] = mergeTwoEmbeds(arrInA, arrInB, baseURL)
				} else {
					a[key] = arrInB
				}
			} else {
				mapInA, isMapInA := foundValInA.(map[string]interface{})
				mapInB, isMapInB := valueInB.(map[string]interface{})
				if isMapInA && isMapInB {
					a[key] = mergeTwoContents(mapInA, mapInB, baseURL)
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

func (h internalContentHandler) expandLeadImages(ec map[string]interface{}) (map[string]interface{}, error) {
	leadImages, found := ec["leadImages"]
	if !found {
		return ec, errors.New("cannot find leadImages in response")
	}

	leadImagesAsArray := (leadImages).([]interface{})
	for i := 0; i < len(leadImagesAsArray); i++ {
		leadImageAsMap, ok := leadImagesAsArray[i].(map[string]interface{})
		if ok {
			h.transformLeadImage(leadImageAsMap)
		}
	}
	return ec, nil
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
	req, err := http.NewRequest(http.MethodPost, h.serviceConfig.contentUnroller.appURI, bytes.NewReader(body))
	if err != nil {
		return expandedContent, err
	}
	req.Header.Set(transactionidutils.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.serviceConfig.httpClient.Do(req)
	if err != nil {
		return content, err
	}
	defer resp.Body.Close()

	uuid := ctx.Value(uuidKey).(string)
	if resp.StatusCode != http.StatusOK {
		h.log.RequestFailedEvent(h.serviceConfig.contentUnroller.appName, req.URL.String(), resp, uuid)
		h.metrics.recordRequestFailedEvent()
		errMsg := fmt.Sprintf("Received status code %d from %v.", resp.StatusCode, h.serviceConfig.contentUnroller.appName)
		return expandedContent, errors.New(errMsg)
	}
	h.log.ResponseEvent(h.serviceConfig.contentUnroller.appName, req.URL.String(), resp, uuid)

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return expandedContent, err
	}

	err = json.Unmarshal(body, &expandedContent)
	if err != nil {
		return expandedContent, err
	}
	return h.expandLeadImages(expandedContent)
}

func (h internalContentHandler) transformLeadImage(leadImage map[string]interface{}) {
	if id, found := leadImage["id"]; found {
		leadImage["id"] = "https://" + h.serviceConfig.envAPIHost + "/content/" + extractIDValue(id.(string))
	}
	imageModel, found := leadImage["image"]
	if !found {
		//if image field is not found inside the image, continue the processing
		return
	}
	var apiURL interface{}
	imageModelAsMap, ok := imageModel.(map[string]interface{})
	if !ok {
		return
	}
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

func resolveRequestURL(content map[string]interface{}, handler internalContentHandler, contentUUID string) {
	content["requestUrl"] = createRequestURL(handler.serviceConfig.envAPIHost, handler.serviceConfig.handlerPath, contentUUID)
}

func resolveAPIURL(content map[string]interface{}, handler internalContentHandler, contentUUID string) {
	handlerPath := handler.serviceConfig.handlerPath
	if !isPreview(handlerPath) {
		content["apiUrl"] = createRequestURL(handler.serviceConfig.envAPIHost, handlerPath, contentUUID)
	} else {
		delete(content, "apiUrl")
	}
}

func isPreview(handlerPath string) bool {
	return strings.HasSuffix(handlerPath, previewSuffix)
}

func createRequestURL(APIHost string, handlerPath string, uuid string) string {
	if isPreview(handlerPath) {
		handlerPath = strings.TrimSuffix(handlerPath, previewSuffix)
	}
	return "https://" + APIHost + "/" + handlerPath + "/" + uuid
}

func (h internalContentHandler) callService(ctx context.Context, r retriever) (responsePart, *http.Response) {
	uuid := ctx.Value(uuidKey).(string)
	requestURL := fmt.Sprintf("%s%s", r.uri, uuid)
	transactionID, _ := transactionidutils.GetTransactionIDFromContext(ctx)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		h.handleError(err, r.sourceAppName, requestURL, req.Header.Get(transactionidutils.TransactionIDHeader), uuid)
		return responsePart{isOk: false, statusCode: http.StatusInternalServerError}, nil
	}
	req.Header.Set(transactionidutils.TransactionIDHeader, transactionID)
	req.Header.Set("Content-Type", "application/json")

	unrollContent, ok := ctx.Value(unrollContentKey).(bool)
	if ok && r.sourceAppName == h.serviceConfig.content.appName {
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
	return content, err
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
