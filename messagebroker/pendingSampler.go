package messagebroker

import (
	"context"
	"time"

	"github.com/giovani-sirbu/mercury/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pendingSampleInterval is how often the sampler refreshes the per-topic
// pending-message gauge. Postgres COUNT(*) over message_queue with the
// existing `idx_mq_pending` index is fast (single-digit ms even at millions
// of rows), but no need to hammer it — 30s is more than fine for alerts.
const pendingSampleInterval = 30 * time.Second

// runPendingSampler refreshes messagebroker_pending_messages{topic} on a
// fixed cadence. Without this gauge, "broker is falling behind" only shows
// up as user-visible delay; with it, Grafana plots queue depth per topic and
// alerts fire before users notice.
//
// Runs forever — no shutdown signal is needed because the goroutine just
// exits when the process exits. If the pool is closed (graceful shutdown via
// messagebroker.Close), the query returns an error which we swallow and
// retry next tick.
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

	// Track which topics the query returned so we can emit 0 for known
	// topics that are absent from the result (empty queue for that topic).
	// Without this fallback a brand-new or fully-drained topic never
	// surfaces a series and the Grafana panel shows "No data".
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
