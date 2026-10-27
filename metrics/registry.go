// Package metrics defines every Prometheus metric the services share and the
// registry /metrics scrapes. Label sets stay bounded: ids go to logs, never labels.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry holds every metric, with the Go runtime and process collectors pre-registered.
var Registry = newRegistry()

func newRegistry() *prometheus.Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(collectors.NewGoCollector())
	r.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return r
}

// Handler is the /metrics HTTP handler; unauthenticated, the services bind to a private network.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		Registry: Registry,
	})
}
