package main

import (
	"github.com/rcrowley/go-metrics"
	"net/http"
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
