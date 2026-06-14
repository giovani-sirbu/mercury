// Package metrics defines every Prometheus metric used across HERMATIC services
// and exposes the registry that each service's /metrics endpoint scrapes.
//
// Design: one shared registry per process (mercury is a library imported by all
// services), one place for every metric name + label set. Each service registers
// the same metrics with its `service` label set to its name. Grafana queries the
// service label to fan out across exchanges, broker topics, etc.
//
// Adding a new metric: declare it in the appropriate file (http.go, broker.go,
// exchange.go, trades.go, workers.go), use NewXxxVec with bounded label sets
// (never put trade_id / user_id / correlation_id in a label — those go to logs).
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry holds every metric exposed by a HERMATIC service. The default Go
// runtime + process collectors are pre-registered so /metrics ships
// go_goroutines, process_cpu_seconds_total, etc. without any extra wiring.
var Registry = newRegistry()

func newRegistry() *prometheus.Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(collectors.NewGoCollector())
	r.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return r
}

// Handler returns the ready-to-mount /metrics HTTP handler. Wire into each
// service's routes with:
//
//	r.GET("/metrics", gin.WrapH(metrics.Handler()))
//
// Not auth-protected: services bind to a private vRack IP (no public path)
// and Prometheus scrapes them from obs-1 inside the same VPC.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		Registry: Registry,
	})
}
