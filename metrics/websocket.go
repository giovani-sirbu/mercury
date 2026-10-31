package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// WSConnectionState is 1 while a WebSocket stream is connected, 0 otherwise.
var WSConnectionState = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "ws_connection_state",
		Help: "1 if the WebSocket stream is connected, 0 otherwise.",
	},
	[]string{"service", "exchange", "stream"},
)

// WSMessagesReceived counts the frames read from a stream.
var WSMessagesReceived = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "ws_messages_received_total",
		Help: "WebSocket frames received from the stream, partitioned by stream.",
	},
	[]string{"service", "exchange", "stream"},
)

// WSReconnects counts reconnect attempts by reason: dial, read, panic, expired.
var WSReconnects = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "ws_reconnects_total",
		Help: "Number of WebSocket reconnect attempts, partitioned by reason.",
	},
	[]string{"service", "exchange", "stream", "reason"},
)

// WSLastMessageAge is the age in seconds of the most recent frame, sampled per stream.
var WSLastMessageAge = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "ws_last_message_age_seconds",
		Help: "Age in seconds of the most recent message received on the WebSocket stream.",
	},
	[]string{"service", "exchange", "stream"},
)
