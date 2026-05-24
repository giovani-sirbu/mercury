package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BrokerHandlerDuration is observed by the messagebroker consumer around the
// per-message handler call. Use it to detect topics whose consumers are slow:
//
//	histogram_quantile(0.99, rate(messagebroker_handler_duration_seconds_bucket[5m])) by (topic)
var BrokerHandlerDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagebroker_handler_duration_seconds",
		Help:    "Per-message broker handler duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
	},
	[]string{"service", "topic", "outcome"}, // outcome: "ok" | "panic"
)

// BrokerMessagesProduced counts every successful Produce call. The
// correlation_id_present label distinguishes ProduceWithCorrelation calls from
// legacy Produce so we can track migration progress.
var BrokerMessagesProduced = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagebroker_messages_produced_total",
		Help: "Total messages produced to the broker outbox.",
	},
	[]string{"service", "topic", "correlation_id_present"},
)

// BrokerStaleLockReclaimed counts messages that were re-claimed because their
// previous holder didn't markProcessed before the stale window expired. A
// non-zero rate means workers are crashing mid-message OR taking longer than
// staleLockAfter to complete.
var BrokerStaleLockReclaimed = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "messagebroker_stale_lock_reclaimed_total",
		Help: "Messages re-claimed after the stale-lock window expired.",
	},
	[]string{"service", "topic"},
)

// BrokerPendingMessages is the queue-depth gauge per topic, set periodically
// by a background sampler in mercury/messagebroker. Alerting on pending > N
// catches consumer-falling-behind scenarios.
var BrokerPendingMessages = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "messagebroker_pending_messages",
		Help: "Messages in message_queue with processed_at IS NULL, sampled per topic.",
	},
	[]string{"service", "topic"},
)

// BrokerE2ELag observes the wall-clock delay between a row's created_at
// (produce time) and the moment the consumer dispatches it to the handler.
// It captures full pipeline lag — NOTIFY latency, claim contention, queue
// backlog — that BrokerHandlerDuration (handler-only) cannot see.
//
// p99 climbing without handler latency climbing = the consumer is starved
// (workers under-provisioned, claim contention, NOTIFY delays). p99
// climbing together with handler latency = handlers are the bottleneck.
//
// Buckets cover 10ms (in-process) to ~163s (multi-minute backlog) in 14
// exponential steps; tuned for HERMATIC's expected sub-second normal path
// with room for outage tails.
var BrokerE2ELag = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "messagebroker_e2e_lag_seconds",
		Help:    "Wall-clock delay from message produce (row created_at) to consumer dispatch, per topic.",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 14),
	},
	[]string{"service", "topic"},
)
