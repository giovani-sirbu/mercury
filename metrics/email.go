package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// EmailSendTotal counts every email send attempt iris makes, partitioned
// by template + outcome. Use with messagebroker_pending_messages{topic=
// "send-emails"} (already live) to get the full picture: queue depth +
// delivery success.
//
//	sum by (result) (rate(email_send_total[5m]))
//
// fail rate climbing = SMTP unhealthy or template error. The reason label
// distinguishes "unmarshal" (broker payload broken) from "template" (file
// missing / parse fail) from "smtp" (sender library returned an error).
var EmailSendTotal = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "email_send_total",
		Help: "Email send attempts, partitioned by template + result + reason.",
	},
	[]string{"service", "template", "result", "reason"}, // result: ok | fail
)

// EmailSendDuration measures the full handler latency from broker message
// arrival to a successful sender.Send() return (or final retry-exhausted
// failure). Includes template render + all SMTP retries.
//
//	histogram_quantile(0.99, rate(email_send_duration_seconds_bucket[5m]))
//
// p99 climbing = SMTP relay slow or template render hot path slow.
var EmailSendDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "email_send_duration_seconds",
		Help:    "End-to-end email handler latency (parse + render + SMTP send with retries).",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 14),
	},
	[]string{"service", "template", "result"},
)
