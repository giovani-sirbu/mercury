package messagebroker

import (
	"context"
	"time"

	"github.com/giovani-sirbu/mercury/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pendingSampleInterval is how often the per-topic pending gauge is refreshed.
const pendingSampleInterval = 30 * time.Second

// runPendingSampler refreshes messagebroker_pending_messages per topic forever;
// a query error is retried next tick.
func runPendingSampler(pool *pgxpool.Pool, serviceName string) {
	t := time.NewTicker(pendingSampleInterval)
	defer t.Stop()
	for range t.C {
		samplePending(pool, serviceName)
	}
}

func samplePending(pool *pgxpool.Pool, serviceName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
        SELECT topic, COUNT(*) FROM message_queue
        WHERE processed_at IS NULL
        GROUP BY topic
    `)
	if err != nil {
		return
	}
	defer rows.Close()

	// Known topics absent from the result are reported as 0.
	seen := make(map[string]struct{})
	for rows.Next() {
		var topic string
		var n int64
		if err := rows.Scan(&topic, &n); err != nil {
			return
		}
		metrics.BrokerPendingMessages.WithLabelValues(serviceName, topic).Set(float64(n))
		seen[topic] = struct{}{}
	}

	for _, topic := range snapshotKnownTopics() {
		if _, hit := seen[topic]; hit {
			continue
		}
		metrics.BrokerPendingMessages.WithLabelValues(serviceName, topic).Set(0)
	}
}
