package main

import (
	"net"
	"net/http"
	"os"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	oldhttphandlers "github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/Financial-Times/service-status-go/gtg"
	"github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
)

const serviceDescription = "A RESTful API for retrieving and transforming internal content"

func main() {
	app := cli.App("internal-content-api", serviceDescription)
	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "internal-content-api",
		Desc:   "The system code of this service",
		EnvVar: "APP_SYSTEM_CODE",
	})
	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "Internal Content API",
		Desc:   "The name of this service",
		EnvVar: "APP_NAME",
	})
	appPort := app.String(cli.StringOpt{
		Name:   "app-port",
		Value:  "8084",
		Desc:   "Default port of the service",
		EnvVar: "APP_PORT",
	})
	handlerPath := app.String(cli.StringOpt{
		Name:   "handler-path",
		Value:  "internalcontent",
		Desc:   "Path on which the handler will be mapped",
		EnvVar: "HANDLER_PATH",
	})
	cacheControlPolicy := app.String(cli.StringOpt{
		Name:   "cache-control-policy",
		Value:  "no-store",
		Desc:   "Cache control policy header",
		EnvVar: "CACHE_CONTROL_POLICY",
	})
	contentSourceURI := app.String(cli.StringOpt{
		Name:   "content-source-uri",
		Value:  "http://localhost:8080/__enriched-content-read-api/enrichedcontent/",
		Desc:   "Content source URI",
		EnvVar: "CONTENT_SOURCE_URI",
	})
	internalComponentsSourceURI := app.String(cli.StringOpt{
		Name:   "internal-components-source-uri",
		Value:  "http://localhost:8080/__content-public-read/internalcontent/",
		Desc:   "Internal components source URI",
		EnvVar: "INTERNAL_COMPONENTS_SOURCE_URI",
	})
	contentSourceAppName := app.String(cli.StringOpt{
		Name:   "content-source-app-name",
		Value:  "Content Source Service",
		Desc:   "Service name of the content source application",
		EnvVar: "CONTENT_SOURCE_APP_NAME",
	})
	internalComponentsSourceAppName := app.String(cli.StringOpt{
		Name:   "internal-components-source-app-name",
		Value:  "Internal Components Source Service",
		Desc:   "Service name of the internal components source application",
		EnvVar: "INTERNAL_COMPONENTS_SOURCE_APP_NAME",
	})
	contentSourceAppHealthURI := app.String(cli.StringOpt{
		Name:   "content-source-app-health-uri",
		Value:  "http://localhost:8080/__enriched-content-read-api/__health",
		Desc:   "URI of the Content Source Application health endpoint",
		EnvVar: "CONTENT_SOURCE_APP_HEALTH_URI",
	})
	internalComponentsSourceAppHealthURI := app.String(cli.StringOpt{
		Name:   "internal-components-source-app-health-uri",
		Value:  "http://localhost:8080/__content-public-read/__health",
		Desc:   "URI of the Internal Components Source Application health endpoint",
		EnvVar: "INTERNAL_COMPONENTS_SOURCE_APP_HEALTH_URI",
	})
	contentSourceAppPanicGuide := app.String(cli.StringOpt{
		Name:   "content-source-app-panic-guide",
		Value:  "https://dewey.ft.com/enriched-content-read-api.html",
		Desc:   "Content source appllication panic guide url for healthcheck. Default panic guide is for Enriched Content.",
		EnvVar: "CONTENT_SOURCE_APP_PANIC_GUIDE",
	})
	internalComponentsSourceAppPanicGuide := app.String(cli.StringOpt{
		Name:   "internal-components-source-app-panic-guide",
		Value:  "https://dewey.in.ft.com/runbooks/contentreadapi.html",
		Desc:   "Internal components source application panic guide url for healthcheck. Default panic guide is for Document Store API.",
		EnvVar: "INTERNAL_COMPONENTS_SOURCE_APP_PANIC_GUIDE",
	})
	contentSourceAppBusinessImpact := app.String(cli.StringOpt{
		Name:   "content-source-app-business-impact",
		Value:  "No articles would be available",
		Desc:   "Describe the business impact the content source app would produce if it is broken.",
		EnvVar: "CONTENT_SOURCE_APP_BUSINESS_IMPACT",
	})
	internalComponentsSourceAppBusinessImpact := app.String(cli.StringOpt{
		Name:   "internal-components-source-app-business-impact",
		Value:  "Articles won't have the internal components",
		Desc:   "Describe the business impact the internal components source app would produce if it is broken.",
		EnvVar: "INTERNAL_COMPONENTS_SOURCE_APP_BUSINESS_IMPACT",
	})
	contentUnrollerURI := app.String(cli.StringOpt{
		Name:   "content-unroller-uri",
		Value:  "http://localhost:8080/__content-unroller/internalcontent",
		Desc:   "URI of the content unroller application",
		EnvVar: "CONTENT_UNROLLER_URI",
	})
	contentUnrollerAppName := app.String(cli.StringOpt{
		Name:   "content-unroller-app-name",
		Value:  "content-unroller",
		Desc:   "Service of the content unroller application",
		EnvVar: "CONTENT_UNROLLER_APP_NAME",
	})
	contentUnrollerAppHealthURI := app.String(cli.StringOpt{
		Name:   "content-unroller-app-health-uri",
		Value:  "http://localhost:8080/__content-unroller/__health",
		Desc:   "URI of the Content Unroller service health endpoint",
		EnvVar: "CONTENT_UNROLLER_APP_HEALTH_URI",
	})
	contentUnrollerAppPanicGuide := app.String(cli.StringOpt{
		Name:   "content-unroller-app-panic-guide",
		Value:  "https://dewey.ft.com/content-unroller.html",
		Desc:   "Content Unroller application panic guide url for healthcheck.",
		EnvVar: "CONTENT_UNROLLER_APP_PANIC_GUIDE",
	})
	contentUnrollerAppBusinessImpact := app.String(cli.StringOpt{
		Name:   "content-unroller-app-business-impact",
		Value:  "Dynamic Content and images would not be available",
		Desc:   "Describe the business impact the content unroller app would produce if it is broken.",
		EnvVar: "CONTENT_UNROLLER_APP_BUSINESS_IMPACT",
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
		httpClient := &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 100,
				Dial: (&net.Dialer{
					KeepAlive: 30 * time.Second,
				}).Dial,
			},
		}
		sc := serviceConfig{
			*appSystemCode,
			*appName,
			*appPort,
			*handlerPath,
			*cacheControlPolicy,
			*contentSourceURI,
			*contentSourceAppName,
			*contentSourceAppHealthURI,
			*contentSourceAppPanicGuide,
			*contentSourceAppBusinessImpact,
			*internalComponentsSourceURI,
			*internalComponentsSourceAppName,
			*internalComponentsSourceAppHealthURI,
			*internalComponentsSourceAppPanicGuide,
			*internalComponentsSourceAppBusinessImpact,
			*contentUnrollerURI,
			*contentUnrollerAppName,
			*contentUnrollerAppHealthURI,
			*contentUnrollerAppPanicGuide,
			*contentUnrollerAppBusinessImpact,
			*envAPIHost,
			*graphiteTCPAddress,
			*graphitePrefix,
			httpClient,
		}
		appLogger := newAppLogger()
		metricsHandler := NewMetrics()
		contentHandler := internalContentHandler{&sc, appLogger, &metricsHandler}
		h := setupServiceHandler(sc, metricsHandler, contentHandler)
		appLogger.ServiceStartedEvent(*appSystemCode, sc.asMap())
		metricsHandler.OutputMetricsIfRequired(*graphiteTCPAddress, *graphitePrefix, *logMetrics)
		err := http.ListenAndServe(":"+*appPort, h)
		if err != nil {
			logrus.Fatalf("Unable to start server: %v", err)
		}
	}
	app.Run(os.Args)
}

func setupServiceHandler(sc serviceConfig, metricsHandler Metrics, contentHandler internalContentHandler) *mux.Router {
	r := mux.NewRouter()
	r.Path("/" + sc.handlerPath + "/{uuid}").Handler(handlers.MethodHandler{"GET": oldhttphandlers.HTTPMetricsHandler(metricsHandler.registry,
		oldhttphandlers.TransactionAwareRequestLoggingHandler(logrus.StandardLogger(), contentHandler))})
	r.Path(httphandlers.BuildInfoPath).HandlerFunc(httphandlers.BuildInfoHandler)
	r.Path(httphandlers.PingPath).HandlerFunc(httphandlers.PingHandler)

	timedHC := fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  sc.appSystemCode,
			Description: serviceDescription,
			Name:        sc.appName,
			Checks:      []fthealth.Check{sc.contentSourceAppCheck(), sc.internalComponentsSourceAppCheck(), sc.contentUnrollerAppCheck()},
		},
		Timeout: 10 * time.Second,
	}
	r.Path("/__health").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(fthealth.Handler(&timedHC))})

	gtgHandler := httphandlers.NewGoodToGoHandler(gtg.StatusChecker(sc.GTG))
	r.Path("/__gtg").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(gtgHandler)})
	r.Path("/__metrics").Handler(handlers.MethodHandler{"GET": http.HandlerFunc(metricsHTTPEndpoint)})
	return r
}

type serviceConfig struct {
	appSystemCode                             string
	appName                                   string
	appPort                                   string
	handlerPath                               string
	cacheControlPolicy                        string
	contentSourceURI                          string
	contentSourceAppName                      string
	contentSourceAppHealthURI                 string
	contentSourceAppPanicGuide                string
	contentSourceAppBusinessImpact            string
	internalComponentsSourceURI               string
	internalComponentsSourceAppName           string
	internalComponentsSourceAppHealthURI      string
	internalComponentsSourceAppPanicGuide     string
	internalComponentsSourceAppBusinessImpact string
	contentUnrollerSourceURI                  string
	contentUnrollerAppName                    string
	contentUnrollerAppHealthURI               string
	contentUnrollerAppPanicGuide              string
	contentUnrollerAppBusinessImpact          string
	envAPIHost                                string
	graphiteTCPAddress                        string
	graphitePrefix                            string
	httpClient                                *http.Client
}

func (sc serviceConfig) asMap() map[string]interface{} {
	return map[string]interface{}{
		"app-system-code":                                sc.appSystemCode,
		"app-name":                                       sc.appName,
		"app-port":                                       sc.appPort,
		"cache-control-policy":                           sc.cacheControlPolicy,
		"handler-path":                                   sc.handlerPath,
		"content-source-uri":                             sc.contentSourceURI,
		"content-source-app-name":                        sc.contentSourceAppName,
		"content-source-app-health-uri":                  sc.contentSourceAppHealthURI,
		"content-source-app-panic-guide":                 sc.contentSourceAppPanicGuide,
		"content-source-app-business-impact":             sc.contentSourceAppBusinessImpact,
		"internal-components-source-uri":                 sc.internalComponentsSourceURI,
		"internal-components-source-app-name":            sc.internalComponentsSourceAppName,
		"internal-components-source-app-health-uri":      sc.internalComponentsSourceAppHealthURI,
		"internal-components-source-app-panic-guide":     sc.internalComponentsSourceAppPanicGuide,
		"internal-components-source-app-business-impact": sc.internalComponentsSourceAppBusinessImpact,
		"content-unroller-source-uri":                    sc.contentUnrollerSourceURI,
		"content-unroller-app-name":                      sc.contentUnrollerAppName,
		"content-unroller-app-health-uri":                sc.contentUnrollerAppHealthURI,
		"content-unroller-app-panic-guide":               sc.contentUnrollerAppPanicGuide,
		"content-unroller-app-bussines-impact":           sc.contentUnrollerAppBusinessImpact,
		"env-api-host":                                   sc.envAPIHost,
		"graphite-tcp-address":                           sc.graphiteTCPAddress,
		"graphite-prefix":                                sc.graphitePrefix,
	}
}
