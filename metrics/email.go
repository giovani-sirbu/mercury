package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// EmailSendTotal counts email send attempts by template, result and reason
// (unmarshal, template, smtp).
var EmailSendTotal = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "email_send_total",
		Help: "Email send attempts, partitioned by template + result + reason.",
	},
	[]string{"service", "template", "result", "reason"}, // result: ok | fail
)

// EmailSendDuration times the email handler from message arrival to the final send result.
var EmailSendDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "email_send_duration_seconds",
		Help:    "End-to-end email handler latency (parse + render + SMTP send with retries).",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 14),
	},
	[]string{"service", "template", "result"},
)
