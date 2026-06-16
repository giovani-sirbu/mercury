package messagebroker

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	commonLog "github.com/giovani-sirbu/mercury/log"
)

const (
	claimBatchSize = 10
	pollInterval   = 5 * time.Second
	// staleLockAfter is the window after which a row's locked_at is
	// considered abandoned and the row can be re-claimed by another worker.
	// 60s is enough for any healthy handler to complete (the trade /
	// email / user handlers all finish in <5s under load) while keeping
	// the redelivery tail short on a real worker crash. Was 5m on master
	// — too long for trade-related messages.
	staleLockAfter      = 60 * time.Second
	reconnectMaxBackoff = 30 * time.Second
	// listenKeepaliveInterval bounds how long the dedicated LISTEN connection
	// waits for a notification before sending a keepalive ping. That socket
	// carries no application bytes between notifications, and the gateway's
	// nginx stream proxy closes a connection idle past its proxy_timeout (12h)
	// — surfacing here as "unexpected EOF" and forcing a reconnect on every
	// low-traffic topic. Pinging well inside that window keeps bytes on the
	// wire so an idle connection is never reaped. 5m is far below the 12h cap
	// (and any plausible intermediate idle timeout) at a cost of one trivial
	// query per idle connection per interval.
	listenKeepaliveInterval = 5 * time.Minute
	// listenKeepalivePingTimeout bounds the keepalive ping so a half-open
	// connection fails fast (and reconnects) instead of blocking the listener.
	listenKeepalivePingTimeout = 10 * time.Second
)

// ContextHandler is the context-aware consumer callback. The context carries
// the per-message correlation id (set under log.correlationIDKey) so handlers
// can propagate it to outbound HTTP / pub/sub / log calls without the caller
// having to plumb it explicitly.
type ContextHandler func(ctx context.Context, msg []byte)

// Consumer subscribes to a topic via LISTEN/NOTIFY and dispatches payloads to
// handler. Claims rows competitively (SELECT FOR UPDATE SKIP LOCKED) so each
// message is processed by exactly one replica. Runs forever; reconnects with
// exponential backoff on connection loss.
//
// Top-level panic recovery guarantees that a panic inside pgx or a handler
// does not take down the consumer goroutine (and therefore the entire stream
// of messages for that topic). Individual handler panics are also caught
// inside runHandler so one bad message does not kill the connection.
//
// Legacy (non-context-aware) handlers receive only the payload. The
// correlation id is read from the row and surfaced in broker-side logs, but
// not threaded into the handler. New code that needs to propagate the id
// should use ConsumerCtx instead.
func (m MessageBroker) Consumer(topic string, handler fn) {
	m.runConsumer(topic, func(_ context.Context, msg []byte) { handler(msg) })
}

// ConsumerCtx is the context-aware variant of Consumer. Identical claim /
// retry / panic-recovery behavior, but the handler receives a context.Context
// pre-populated with the message's correlation id.
func (m MessageBroker) ConsumerCtx(topic string, handler ContextHandler) {
	m.runConsumer(topic, handler)
}

func (m MessageBroker) runConsumer(topic string, handler ContextHandler) {
	prefixedTopic := topicWithPrefix(topic)
	registerTopic(prefixedTopic)
	commonLog.Info(fmt.Sprintf("Consumer started on topic: %s", prefixedTopic), "", "Consumer")

	backoff := time.Second
	for {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					commonLog.Error(fmt.Sprintf("Consumer panic on %s: %v\n%s", prefixedTopic, rec, debug.Stack()), "", "Consumer")
				}
			}()
			if err := m.listen(prefixedTopic, handler); err != nil {
				commonLog.Error(fmt.Sprintf("Listener lost on %s: %s (reconnecting in %s)", prefixedTopic, err, backoff), "", "Consumer")
			}
		}()
		time.Sleep(backoff)
		backoff *= 2
		if backoff > reconnectMaxBackoff {
			backoff = reconnectMaxBackoff
		}
	}
}
