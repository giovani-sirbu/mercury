package messagebroker

import (
	"context"
	"fmt"

	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/metrics"
)

// Produce persists a message to the outbox and fires pg_notify as a wakeup
// signal. The write and notify run in a single transaction, so listeners never
// wake up for a row that isn't committed. The `key` parameter is accepted for
// API compatibility with the prior Kafka implementation but is unused.
func (m MessageBroker) Produce(topic string, key, value []byte, producer *Producer) error {
	return m.produceWithCorrelation(topic, value, "", producer, "Produce")
}

// ProduceWithCorrelation is Produce that also tags the row with a correlation
// id, so the downstream consumer (and its own outbound calls) can carry the
// same id through every hop. Pass "" to produce without an id.
func (m MessageBroker) ProduceWithCorrelation(topic string, value []byte, correlationID string, producer *Producer) error {
	return m.produceWithCorrelation(topic, value, correlationID, producer, "ProduceWithCorrelation")
}

// produceWithCorrelation is the shared implementation; the public methods are
// thin wrappers so the legacy Kafka-shaped signature stays intact.
func (m MessageBroker) produceWithCorrelation(topic string, value []byte, correlationID string, producer *Producer, trackName string) error {
	ctx := context.Background()
	prefixedTopic := topicWithPrefix(topic)
	registerTopic(prefixedTopic)

	tx, err := producer.Pool.Begin(ctx)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to begin tx: %s", err), trackName, "Producer", log.WithCorrelation(correlationID))
		return err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO message_queue (topic, payload, correlation_id) VALUES ($1, $2::jsonb, NULLIF($3, '')) RETURNING id`,
		prefixedTopic, string(value), correlationID,
	).Scan(&id)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to insert message: %s", err), trackName, "Producer", log.WithCorrelation(correlationID))
		return err
	}

	if _, err := tx.Exec(ctx, `SELECT pg_notify($1, $2)`, prefixedTopic, fmt.Sprint(id)); err != nil {
		log.Error(fmt.Sprintf("Failed to notify: %s", err), trackName, "Producer", log.WithCorrelation(correlationID))
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error(fmt.Sprintf("Failed to commit: %s", err), trackName, "Producer", log.WithCorrelation(correlationID))
		return err
	}

	// Count produced messages, partitioning by whether the correlation_id
	// is set. Helps track adoption as more callers migrate from Produce to
	// ProduceWithCorrelation.
	cidLabel := "false"
	if correlationID != "" {
		cidLabel = "true"
	}
	metrics.BrokerMessagesProduced.
		WithLabelValues(state.serviceName, prefixedTopic, cidLabel).
		Inc()

	log.Info(fmt.Sprintf("Produced on topic: %s", prefixedTopic), "", "Producer", log.WithCorrelation(correlationID))
	return nil
}
