package messagebroker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/giovani-sirbu/mercury/log"
	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultPoolMaxConns bounds the messagebus pool per service; MESSAGEBUS_MAX_CONNS overrides it.
const defaultPoolMaxConns = 4

// MessageBroker configures the Postgres-backed pub/sub broker: an outbox table plus pg_notify.
type MessageBroker struct {
	DSN         string
	ServiceName string
	Timeout     time.Duration
}

type Producer struct {
	Pool *pgxpool.Pool
}

// fn is the legacy consumer handler callback.
type fn func([]byte)

type BrokerMethods struct {
	Producer *Producer
	Produce  func(topic string, key, value []byte, producer *Producer) error
	Consumer func(topic string, handler fn)

	// ProduceWithCorrelation persists a message tagged with a correlation id.
	ProduceWithCorrelation func(topic string, value []byte, correlationID string, producer *Producer) error

	// ConsumerCtx subscribes a handler whose context carries the message's correlation id.
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
		poolCfg, err := pgxpool.ParseConfig(broker.DSN)
		if err != nil {
			// A malformed DSN surfaces at bootstrap as an empty BrokerMethods.
			log.Error(fmt.Sprintf("Failed to parse messagebus DSN: %s", err), "Init", "MessageBroker")
			return BrokerMethods{}
		}
		poolCfg.MaxConns = int32(envInt("MESSAGEBUS_MAX_CONNS", defaultPoolMaxConns))
		poolCfg.MinConns = 1

		pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to connect to messagebus: %s", err), "Init", "MessageBroker")
			return BrokerMethods{}
		}
		if err := ensureSchema(context.Background(), pool); err != nil {
			log.Error(fmt.Sprintf("Failed to bootstrap messagebus schema: %s", err), "Init", "MessageBroker")
		}
		state = brokerState{pool: pool, serviceName: broker.ServiceName, dsn: broker.DSN}
		go runCleanup(pool)
		go runPendingSampler(pool, broker.ServiceName)
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

// envInt reads a positive integer env var, falling back to def otherwise.
func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

// currentPool reads the live pool under stateMu; Close nils it under the same mutex.
func currentPool() *pgxpool.Pool {
	stateMu.Lock()
	defer stateMu.Unlock()
	return state.pool
}

// Ping verifies the messagebus pool is initialised and reachable within ctx.
func Ping(ctx context.Context) error {
	stateMu.Lock()
	pool := state.pool
	stateMu.Unlock()
	if pool == nil {
		return fmt.Errorf("messagebroker: pool not initialized")
	}
	return pool.Ping(ctx)
}

// Close releases the messagebus pool; safe when none was initialised.
func Close() {
	stateMu.Lock()
	defer stateMu.Unlock()
	if state.pool != nil {
		state.pool.Close()
		state.pool = nil
	}
}
