package messagebroker

import (
	"context"
	"fmt"
	"time"

	"github.com/giovani-sirbu/mercury/log"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	cleanupInterval  = 6 * time.Hour
	cleanupRetention = "7 days"
)

// runCleanup periodically deletes processed messages older than the retention
// window. Keeps the outbox bounded in size without manual ops work.
func runCleanup(pool *pgxpool.Pool) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := pool.Exec(ctx,
			fmt.Sprintf(`DELETE FROM message_queue WHERE processed_at IS NOT NULL AND processed_at < NOW() - INTERVAL '%s'`, cleanupRetention))
		cancel()
		if err != nil {
			log.Error(fmt.Sprintf("cleanup failed: %s", err), "runCleanup", "MessageBroker")
		}
	}
}
