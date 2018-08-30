package main

import (
	standardLog "log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cyberdelia/go-metrics-graphite"
	"github.com/rcrowley/go-metrics"
)

type Metrics struct {
	registry           metrics.Registry
	errorMeter         string
	requestFailedMeter string
	responseMeter      string
}

func NewMetrics() Metrics {
	mx := Metrics{metrics.DefaultRegistry, "5xx", "4xx", "200"}
	mx.registry.Register(mx.errorMeter, metrics.NewMeter())
	mx.registry.Register(mx.requestFailedMeter, metrics.NewMeter())
	mx.registry.Register(mx.responseMeter, metrics.NewMeter())
	return mx
}

func (m Metrics) recordErrorEvent() {
	meter := m.registry.Get(m.errorMeter).(metrics.Meter)
	meter.Mark(1)
}

func (m Metrics) recordRequestFailedEvent() {
	meter := m.registry.Get(m.requestFailedMeter).(metrics.Meter)
	meter.Mark(1)
}

func (m Metrics) recordResponseEvent() {
	meter := m.registry.Get(m.responseMeter).(metrics.Meter)
	meter.Mark(1)
}

func metricsHTTPEndpoint(w http.ResponseWriter, r *http.Request) {
	metrics.WriteOnce(metrics.DefaultRegistry, w)
}

//OutputMetricsIfRequired will send metrics to Graphite if a non-empty graphiteTCPAddress is passed in, or to the standard log if logMetrics is true.
// Make sure a sensible graphitePrefix that will uniquely identify your service is passed in, e.g. "content.test.people.rw.neo4j.ftaps58938-law1a-eu-t
func (m Metrics) OutputMetricsIfRequired(graphiteTCPAddress string, graphitePrefix string, logMetrics bool) {
	if graphiteTCPAddress != "" {
		addr, _ := net.ResolveTCPAddr("tcp", graphiteTCPAddress)
		go graphite.Graphite(m.registry, 5*time.Second, graphitePrefix, addr)
	}
	if logMetrics { //useful locally
		//messy use of the 'standard' log package here as this method takes the log struct, not an interface, so can't use logrus.Logger
		go metrics.Log(metrics.DefaultRegistry, 60*time.Second, standardLog.New(os.Stdout, "metrics", standardLog.Lmicroseconds))
	}
}
