package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// WSConnectionState reports whether each WebSocket subscription is up. 1 =
// connected, 0 = disconnected (or never connected). One series per
// (exchange, stream) pair — bounded because the set of streams we open is
// fixed (price feed + user data per exchange).
//
//	sum by (exchange, stream) (ws_connection_state) == 0
//
// is the canonical "stream down" alert.
var WSConnectionState = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "ws_connection_state",
		Help: "1 if the WebSocket stream is connected, 0 otherwise.",
	},
	[]string{"service", "exchange", "stream"},
)

// WSMessagesReceived counts every frame we successfully read from a WS
// stream. Combined with WSLastMessageAge it tells "no frames in N
// seconds" apart from "no connection at all".
var WSMessagesReceived = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "ws_messages_received_total",
		Help: "WebSocket frames received from the stream, partitioned by stream.",
	},
	[]string{"service", "exchange", "stream"},
)

// WSReconnects counts reconnection attempts. The reason label captures
// why the previous connection ended:
//   - "dial":   could not establish the TCP/WS handshake
//   - "read":   ReadMessage returned an error (server closed, network)
//   - "panic":  reader goroutine panicked
//   - "expired": listen key expired (user-data stream only)
//
// A steady non-zero reconnect rate = the upstream is flapping. Hermes's
// price stream goes silent during reconnect — a fast reconnect is fine,
// a slow one starves the trade engine.
var WSReconnects = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "ws_reconnects_total",
		Help: "Number of WebSocket reconnect attempts, partitioned by reason.",
	},
	[]string{"service", "exchange", "stream", "reason"},
)

// WSLastMessageAge is the wall-clock age of the most recent received
// frame, in seconds. Sampled by a background goroutine per stream so
// staleness is visible even when the connection appears "up" but the
// upstream has silently stopped pushing.
//
//	ws_last_message_age_seconds{exchange="binance",stream="price"} > 30
//
// fires when the stream goes silent. Far more reliable than relying on
// ws_connection_state alone — a connected-but-mute upstream is a common
// failure mode that hangs trade decisions.
var WSLastMessageAge = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "ws_last_message_age_seconds",
		Help: "Age in seconds of the most recent message received on the WebSocket stream.",
	},
	[]string{"service", "exchange", "stream"},
)
