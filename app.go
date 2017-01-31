package main

import (
	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	oldhttphandlers "github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	"net/http"
	"os"
	"time"
)

const serviceDescription = "A RESTful API for retrieving and transforming internal content"

var timeout = time.Duration(10 * time.Second)
var client = &http.Client{Timeout: timeout}

func main() {
	app := cli.App("internal-content-api", serviceDescription)
	serviceName := app.StringOpt("app-name", "internal-content-api", "The name of this service")
	appPort := app.String(cli.StringOpt{
		Name:   "app-port",
		Value:  "8084",
		Desc:   "Default port for Internal Content API",
		EnvVar: "APP_PORT",
	})
	enrichedContentAPIURI := app.String(cli.StringOpt{
		Name:   "enriched-content-api-uri",
		Value:  "http://localhost:8080/__enriched-content-read-api/enrichedcontent/",
		Desc:   "Enriched Content API URI",
		EnvVar: "ENRICHED_CONTENT_API_URI",
	})
	documentStoreAPIURI := app.String(cli.StringOpt{
		Name:   "document-store-api-uri",
		Value:  "http://localhost:8080/__document-store-api/internalcomponents/",
		Desc:   "Document Store API URI",
		EnvVar: "DOCUMENT_STORE_API_URI",
	})
	enrichedContentAppName := app.String(cli.StringOpt{
		Name:   "enriched-content-app-name",
		Value:  "Enriched Content Service",
		Desc:   "Service name of the enriched content application",
		EnvVar: "ENRICHED_CONTENT_APP_NAME",
	})
	documentStoreAppName := app.String(cli.StringOpt{
		Name:   "document-store-app-name",
		Value:  "Document Store Service",
		Desc:   "Service name of the document store application",
		EnvVar: "DOCUMENT_STORE_APP_NAME",
	})
	enrichedContentAppHealthURI := app.String(cli.StringOpt{
		Name:   "enriched-content-app-health-uri",
		Value:  "http://localhost:8080/__enriched-content-read-api/__health",
		Desc:   "URI of the Enriched Content Application health endpoint",
		EnvVar: "ENRICHED_CONTENT_APP_HEALTH_URI",
	})
	documentStoreAppHealthURI := app.String(cli.StringOpt{
		Name:   "document-store-app-health-uri",
		Value:  "http://localhost:8080/__enriched-content-read-api/__health",
		Desc:   "URI of the Document Store Application health endpoint",
		EnvVar: "DOCUMENT_STORE_APP_HEALTH_URI",
	})
	enrichedContentAppPanicGuide := app.String(cli.StringOpt{
		Name:   "enriched-content-app-panic-guide",
		Value:  "https://sites.google.com/a/ft.com/dynamic-publishing-team/content-public-read-panic-guide",
		Desc:   "Enriched content appllication application panic guide url for healthcheck. Default panic guide is for content public read.",
		EnvVar: "ENRICHED_CONTENT_APP_PANIC_GUIDE",
	})
	documentStoreAppPanicGuide := app.String(cli.StringOpt{
		Name:   "document-store-app-panic-guide",
		Value:  "https://sites.google.com/a/ft.com/dynamic-publishing-team/document-store-api-panic-guide",
		Desc:   "Document Store application panic guide url for healthcheck. Default panic guide is for document store api",
		EnvVar: "DOCUMENT_STORE_APP_PANIC_GUIDE",
	})
	envAPIHost := app.String(cli.StringOpt{
		Name:   "env-api-host",
		Value:  "api.ft.com",
		Desc:   "API host to use for URLs in responses",
		EnvVar: "ENV_API_HOST",
	})
	graphiteTCPAddress := app.String(cli.StringOpt{
		Name:   "graphite-tcp-address",
		Value:  "",
		Desc:   "Graphite TCP address, e.g. graphite.ft.com:2003. Leave as default if you do NOT want to output to graphite (e.g. if running locally)",
		EnvVar: "GRAPHITE_TCP_ADDRESS",
	})
	graphitePrefix := app.String(cli.StringOpt{
		Name:   "graphite-prefix",
		Value:  "coco.services.$ENV.content-preview.0",
		Desc:   "Prefix to use. Should start with content, include the environment, and the host name. e.g. coco.pre-prod.sections-rw-neo4j.1",
		EnvVar: "GRAPHITE_PREFIX",
	})
	logMetrics := app.Bool(cli.BoolOpt{
		Name:   "log-metrics",
		Value:  false,
		Desc:   "Whether to log metrics. Set to true if running locally and you want metrics output",
		EnvVar: "LOG_METRICS",
	})
	app.Action = func() {
		sc := serviceConfig{
			*serviceName,
			*appPort,
			*enrichedContentAPIURI,
			*documentStoreAPIURI,
			*enrichedContentAppName,
			*documentStoreAppName,
			*enrichedContentAppHealthURI,
			*documentStoreAppHealthURI,
			*enrichedContentAppPanicGuide,
			*documentStoreAppPanicGuide,
			*envAPIHost,
			*graphiteTCPAddress,
			*graphitePrefix,
		}
		appLogger := newAppLogger()
		metricsHandler := NewMetrics()
		contentHandler := contentHandler{&sc, appLogger, &metricsHandler}
		h := setupServiceHandler(sc, metricsHandler, contentHandler)
		appLogger.ServiceStartedEvent(*serviceName, sc.asMap())
		metricsHandler.OutputMetricsIfRequired(*graphiteTCPAddress, *graphitePrefix, *logMetrics)
		err := http.ListenAndServe(":"+*appPort, h)
		if err != nil {
			logrus.Fatalf("Unable to start server: %v", err)
		}
	}
	app.Run(os.Args)
}

func setupServiceHandler(sc serviceConfig, metricsHandler Metrics, contentHandler contentHandler) *mux.Router {
	r := mux.NewRouter()
	r.Path("/internalcontent/{uuid}").Handler(handlers.MethodHandler{"GET": oldhttphandlers.HTTPMetricsHandler(metricsHandler.registry,
		oldhttphandlers.TransactionAwareRequestLoggingHandler(logrus.StandardLogger(), contentHandler))})
	r.Path(httphandlers.BuildInfoPath).HandlerFunc(httphandlers.BuildInfoHandler)
	r.Path(httphandlers.PingPath).HandlerFunc(httphandlers.PingHandler)
	r.Path("/__health").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(fthealth.Handler(sc.serviceName, serviceDescription, sc.enrichedContentAppCheck(), sc.documentStoreAppCheck()))})
	r.Path("/__metrics").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(metricsHTTPEndpoint)})
	return r
}

type serviceConfig struct {
	serviceName                  string
	appPort                      string
	enrichedContentAPIURI        string
	documentStoreAPIURI          string
	enrichedContentAppName       string
	documentStoreAppName         string
	enrichedContentAppHealthURI  string
	documentStoreAppHealthURI    string
	enrichedContentAppPanicGuide string
	documentStoreAppPanicGuide   string
	envAPIHost                   string
	graphiteTCPAddress           string
	graphitePrefix               string
}

func (sc serviceConfig) asMap() map[string]interface{} {
	return map[string]interface{}{
		"service-name":                     sc.serviceName,
		"service-port":                     sc.appPort,
		"enriched-content-api-uri":         sc.enrichedContentAPIURI,
		"document-store-api-uri":           sc.documentStoreAPIURI,
		"enriched-content-app-name":        sc.enrichedContentAppName,
		"document-store-app-name":          sc.documentStoreAppName,
		"enriched-content-app-health-uri":  sc.enrichedContentAppHealthURI,
		"document-store-app-health-uri":    sc.documentStoreAppHealthURI,
		"enriched-content-app-panic-guide": sc.enrichedContentAppPanicGuide,
		"document-store-app-panic-guide":   sc.documentStoreAppPanicGuide,
		"env-api-host":                     sc.envAPIHost,
		"graphite-tcp-address":             sc.graphiteTCPAddress,
		"graphite-prefix":                  sc.graphitePrefix,
	}
}
