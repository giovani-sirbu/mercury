package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BrokerHandlerDuration times the per-message consumer handler call.
var BrokerHandlerDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagebroker_handler_duration_seconds",
		Help:    "Per-message broker handler duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
	},
	[]string{"service", "topic", "outcome"}, // outcome: "ok" | "panic"
)

// BrokerMessagesProduced counts successful Produce calls, with or without a correlation id.
var BrokerMessagesProduced = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagebroker_messages_produced_total",
		Help: "Total messages produced to the broker outbox.",
	},
	[]string{"service", "topic", "correlation_id_present"},
)

// BrokerStaleLockReclaimed counts messages re-claimed after their stale-lock window expired.
var BrokerStaleLockReclaimed = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagebroker_stale_lock_reclaimed_total",
		Help: "Messages re-claimed after the stale-lock window expired.",
	},
	[]string{"service", "topic"},
)

// BrokerPendingMessages is the queue depth per topic, set by the pending sampler.
var BrokerPendingMessages = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "messagebroker_pending_messages",
		Help: "Messages in message_queue with processed_at IS NULL, sampled per topic.",
	},
	[]string{"service", "topic"},
)

// BrokerE2ELag is the delay from a row's produce time to its dispatch to the handler.
var BrokerE2ELag = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagebroker_e2e_lag_seconds",
		Help:    "Wall-clock delay from message produce (row created_at) to consumer dispatch, per topic.",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 14),
	},
	[]string{"service", "topic"},
)
