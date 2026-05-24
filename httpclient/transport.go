// Package httpclient provides a small RoundTripper wrapper that emits the
// HTTPClientRequestDuration + HTTPClientRequestsTotal metrics on every
// outbound request. Designed to be dropped in by replacing a client's
// Transport field — zero changes at the call sites of an existing
// http.Client.
package httpclient

import (
	"net/http"
	"strconv"
	"time"

	"github.com/giovani-sirbu/mercury/metrics"
)

// InstrumentTransport wraps base so each round-trip emits client metrics.
// If base is nil, http.DefaultTransport is used.
//
// Typical use in a service's externalRequest.go:
//
//	sharedHTTPClient := &http.Client{Timeout: 5 * time.Second}
//	sharedHTTPClient.Transport = httpclient.InstrumentTransport(
//	    sharedHTTPClient.Transport, "agora")
//
// The service label is fixed (the caller's name). target_host comes from
// req.URL.Host on each call so a single client used to talk to multiple
// downstreams is broken down naturally.
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

	// status_class buckets requests into 5 stable values regardless of
	// the exact code. Keeps cardinality bounded across hundreds of
	// distinct downstream status codes.
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

// classify maps a numeric HTTP status into a low-cardinality class label.
// Same scheme used elsewhere in the platform (see exchange instrumentation)
// so dashboards can reuse the same drilldown vocabulary.
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
