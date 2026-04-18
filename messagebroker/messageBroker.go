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
		Producer: &Producer{Pool: state.pool},
		Produce:  broker.Produce,
		Consumer: broker.Consumer,
	}
}

func topicWithPrefix(topic string) string {
	return fmt.Sprintf("%s%s", os.Getenv("TOPIC_PREFIX"), topic)
}
