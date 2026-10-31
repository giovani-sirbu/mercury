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
	// staleLockAfter is the window after which a claimed row counts as
	// abandoned and is re-claimed: above any healthy handler's run, and no
	// higher, since it is also the redelivery tail of a crash.
	staleLockAfter      = 60 * time.Second
	reconnectMaxBackoff = 30 * time.Second
	// listenKeepaliveInterval bounds how long the LISTEN connection sits idle
	// before a keepalive ping, under the gateway's proxy idle timeout.
	listenKeepaliveInterval = 5 * time.Minute
	// listenKeepalivePingTimeout bounds the keepalive ping so a half-open connection fails fast.
	listenKeepalivePingTimeout = 10 * time.Second
)

// ContextHandler is the consumer callback whose context carries the message's correlation id.
type ContextHandler func(ctx context.Context, msg []byte)

// Consumer subscribes to a topic via LISTEN/NOTIFY and dispatches payloads to
// handler. Rows are claimed competitively (SELECT FOR UPDATE SKIP LOCKED) so
// each message is handled by exactly one replica; the loop runs forever and
// reconnects with exponential backoff. A panic in pgx or a handler is
// recovered so one bad message never takes the topic's stream down. Legacy
// handlers get the payload alone; ConsumerCtx carries the correlation id.
func (m MessageBroker) Consumer(topic string, handler fn) {
	m.runConsumer(topic, func(_ context.Context, msg []byte) { handler(msg) })
}

// ConsumerCtx is Consumer with the correlation id on the handler's context.
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
