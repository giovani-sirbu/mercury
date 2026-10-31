// Package httpclient is a RoundTripper wrapper that emits the outbound HTTP client metrics.
package httpclient

import (
	"net/http"
	"strconv"
	"time"

	"github.com/giovani-sirbu/mercury/metrics"
)

// InstrumentTransport wraps base so each round trip emits client metrics; nil base is the default transport.
func InstrumentTransport(base http.RoundTripper, serviceName string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &measuringRoundTripper{base: base, service: serviceName}
}

type measuringRoundTripper struct {
	base    http.RoundTripper
	service string
}

func (m *measuringRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := m.base.RoundTrip(req)
	elapsed := time.Since(start).Seconds()

	var statusClass string
	if err != nil {
		statusClass = "transport_error"
	} else {
		statusClass = classify(resp.StatusCode)
	}

	host := req.URL.Host
	if host == "" {
		host = "<unknown>"
	}
	method := req.Method

	metrics.HTTPClientRequestDuration.
		WithLabelValues(m.service, host, method, statusClass).
		Observe(elapsed)
	metrics.HTTPClientRequestsTotal.
		WithLabelValues(m.service, host, method, statusClass).
		Inc()

	return resp, err
}

// classify maps an HTTP status onto its class label.
func classify(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return strconv.Itoa(code)
	}
}
