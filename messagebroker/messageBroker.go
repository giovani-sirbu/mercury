package messagebroker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/giovani-sirbu/mercury/log"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MessageBroker configures the Postgres-backed pub/sub broker.
// Messages are persisted to the message_queue outbox table (durability) and
// pg_notify fires as a wakeup signal for listening consumers.
type MessageBroker struct {
	DSN         string
	ServiceName string
	Timeout     time.Duration
}

type Producer struct {
	Pool *pgxpool.Pool
}

// fn is the consumer handler callback type. Signature preserved from the
// previous Kafka-based implementation so existing handlers work unchanged.
type fn func([]byte)

type BrokerMethods struct {
	Producer *Producer
	Produce  func(topic string, key, value []byte, producer *Producer) error
	Consumer func(topic string, handler fn)

	// ProduceWithCorrelation persists a message tagged with a correlation id.
	// Use it from any code path that already has one (HTTP handlers via
	// ginAdaptors.GetCorrelationID, pub/sub consumers via
	// log.CorrelationFromContext, hermes action chains via events.CorrelationID).
	ProduceWithCorrelation func(topic string, value []byte, correlationID string, producer *Producer) error

	// ConsumerCtx subscribes a context-aware handler. The context delivered to
	// the handler carries the message's correlation id (via
	// log.ContextWithCorrelation), so downstream HTTP / pub/sub / log calls
	// can propagate the id without it being passed as an explicit argument.
	ConsumerCtx func(topic string, handler ContextHandler)
}

type brokerState struct {
	pool        *pgxpool.Pool
	serviceName string
	dsn         string
}

var (
	state   brokerState
	stateMu sync.Mutex
)

func (broker MessageBroker) Init() BrokerMethods {
	stateMu.Lock()
	defer stateMu.Unlock()

	if state.pool == nil {
		pool, err := pgxpool.New(context.Background(), broker.DSN)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to connect to messagebus: %s", err), "Init", "MessageBroker")
		}
		if err := ensureSchema(context.Background(), pool); err != nil {
			log.Error(fmt.Sprintf("Failed to bootstrap messagebus schema: %s", err), "Init", "MessageBroker")
		}
		state = brokerState{pool: pool, serviceName: broker.ServiceName, dsn: broker.DSN}
		go runCleanup(pool)
	}

	return BrokerMethods{
		Producer:               &Producer{Pool: state.pool},
		Produce:                broker.Produce,
		Consumer:               broker.Consumer,
		ProduceWithCorrelation: broker.ProduceWithCorrelation,
		ConsumerCtx:            broker.ConsumerCtx,
	}
}

func topicWithPrefix(topic string) string {
	return fmt.Sprintf("%s%s", os.Getenv("TOPIC_PREFIX"), topic)
}

// Ping verifies the messagebus pool is reachable. Used by the /readyz health
// endpoint to assert the broker is functional before k8s routes traffic to
// the pod. Returns an error if the pool was never initialized, if the
// underlying Postgres connection is down, or if the ping times out within
// ctx.
func Ping(ctx context.Context) error {
	stateMu.Lock()
	pool := state.pool
	stateMu.Unlock()
	if pool == nil {
		return fmt.Errorf("messagebroker: pool not initialized")
	}
	return pool.Ping(ctx)
}

// Close releases the messagebus pool. Used by services during graceful
// shutdown after consumers have drained. Safe to call when no pool was ever
// initialized.
func Close() {
	stateMu.Lock()
	defer stateMu.Unlock()
	if state.pool != nil {
		state.pool.Close()
		state.pool = nil
	}
}
