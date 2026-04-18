package messagebroker

import (
	"context"
	"fmt"

	"github.com/giovani-sirbu/mercury/log"
)

// Produce persists a message to the outbox and fires pg_notify as a wakeup
// signal. The write and notify run in a single transaction, so listeners never
// wake up for a row that isn't committed. The `key` parameter is accepted for
// API compatibility with the prior Kafka implementation but is unused.
func (m MessageBroker) Produce(topic string, key, value []byte, producer *Producer) error {
	ctx := context.Background()
	prefixedTopic := topicWithPrefix(topic)

	tx, err := producer.Pool.Begin(ctx)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to begin tx: %s", err), "Produce", "Producer")
		return err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO message_queue (topic, payload) VALUES ($1, $2::jsonb) RETURNING id`,
		prefixedTopic, string(value),
	).Scan(&id)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to insert message: %s", err), "Produce", "Producer")
		return err
	}

	if _, err := tx.Exec(ctx, `SELECT pg_notify($1, $2)`, prefixedTopic, fmt.Sprint(id)); err != nil {
		log.Error(fmt.Sprintf("Failed to notify: %s", err), "Produce", "Producer")
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error(fmt.Sprintf("Failed to commit: %s", err), "Produce", "Producer")
		return err
	}

	log.Info(fmt.Sprintf("Produced on topic: %s", prefixedTopic), "", "Producer")
	return nil
}
