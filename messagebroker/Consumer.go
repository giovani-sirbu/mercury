package messagebroker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	commonLog "github.com/giovani-sirbu/mercury/log"
	"github.com/jackc/pgx/v5"
)

const (
	claimBatchSize      = 10
	pollInterval        = 5 * time.Second
	staleLockAfter      = 5 * time.Minute
	reconnectMaxBackoff = 30 * time.Second
)

// Consumer subscribes to a topic via LISTEN/NOTIFY and dispatches payloads to
// handler. Claims rows competitively (SELECT FOR UPDATE SKIP LOCKED) so each
// message is processed by exactly one replica. Runs forever; reconnects with
// exponential backoff on connection loss.
func (m MessageBroker) Consumer(topic string, handler fn) {
	prefixedTopic := topicWithPrefix(topic)
	commonLog.Info(fmt.Sprintf("Consumer started on topic: %s", prefixedTopic), "", "Consumer")

	backoff := time.Second
	for {
		err := m.listen(prefixedTopic, handler)
		if err != nil {
			commonLog.Error(fmt.Sprintf("Listener lost on %s: %s (reconnecting in %s)", prefixedTopic, err, backoff), "", "Consumer")
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > reconnectMaxBackoff {
			backoff = reconnectMaxBackoff
		}
	}
}

func (m MessageBroker) listen(prefixedTopic string, handler fn) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := pgx.Connect(ctx, m.DSN)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	ident := pgx.Identifier{prefixedTopic}.Sanitize()
	if _, err := conn.Exec(ctx, fmt.Sprintf("LISTEN %s", ident)); err != nil {
		return fmt.Errorf("LISTEN: %w", err)
	}

	// Drain any backlog on startup (pre-subscription messages or
	// messages left locked by a crashed worker past the stale-lock window).
	claimAndRun(ctx, prefixedTopic, m.ServiceName, handler)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	notifCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	go func() {
		for {
			if _, err := conn.WaitForNotification(ctx); err != nil {
				errCh <- err
				return
			}
			select {
			case notifCh <- struct{}{}:
			default:
			}
		}
	}()

	for {
		select {
		case <-notifCh:
			claimAndRun(ctx, prefixedTopic, m.ServiceName, handler)
		case <-ticker.C:
			claimAndRun(ctx, prefixedTopic, m.ServiceName, handler)
		case err := <-errCh:
			return err
		}
	}
}

func claimAndRun(ctx context.Context, topic, serviceName string, handler fn) {
	for {
		claimed, err := claimBatch(ctx, topic, serviceName, handler)
		if err != nil {
			commonLog.Error(fmt.Sprintf("claim failed on %s: %s", topic, err), "claimAndRun", "Consumer")
			return
		}
		if claimed == 0 {
			return
		}
	}
}

func claimBatch(ctx context.Context, topic, serviceName string, handler fn) (int, error) {
	pool := state.pool
	if pool == nil {
		return 0, fmt.Errorf("broker pool not initialized")
	}
	lockedBy := fmt.Sprintf("%s:%d", serviceName, os.Getpid())
	staleBefore := time.Now().Add(-staleLockAfter)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
        SELECT id, payload FROM message_queue
        WHERE topic = $1
          AND processed_at IS NULL
          AND (locked_at IS NULL OR locked_at < $3)
        ORDER BY created_at
        LIMIT $2
        FOR UPDATE SKIP LOCKED
    `, topic, claimBatchSize, staleBefore)
	if err != nil {
		return 0, err
	}

	type claimedRow struct {
		id      int64
		payload []byte
	}
	var claimed []claimedRow
	for rows.Next() {
		var id int64
		var raw json.RawMessage
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return 0, err
		}
		claimed = append(claimed, claimedRow{id: id, payload: []byte(raw)})
	}
	rows.Close()

	if len(claimed) == 0 {
		return 0, tx.Commit(ctx)
	}

	ids := make([]int64, 0, len(claimed))
	for _, r := range claimed {
		ids = append(ids, r.id)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE message_queue SET locked_at = NOW(), locked_by = $2, attempts = attempts + 1
         WHERE id = ANY($1)`, ids, lockedBy); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	for _, r := range claimed {
		runHandler(ctx, topic, r.id, r.payload, handler)
	}
	return len(claimed), nil
}

func runHandler(ctx context.Context, topic string, id int64, payload []byte, handler fn) {
	defer func() {
		if rec := recover(); rec != nil {
			msg := fmt.Sprintf("handler panic on %s id=%d: %v\n%s", topic, id, rec, debug.Stack())
			commonLog.Error(msg, "runHandler", "Consumer")
			releaseLock(ctx, id, fmt.Sprintf("panic: %v", rec))
		}
	}()
	handler(payload)
	if err := markProcessed(ctx, id); err != nil {
		commonLog.Error(fmt.Sprintf("mark processed failed id=%d: %s", id, err), "runHandler", "Consumer")
		return
	}
	commonLog.Info(fmt.Sprintf("Consumed on topic: %s", topic), "", "Consumer")
}

func markProcessed(ctx context.Context, id int64) error {
	_, err := state.pool.Exec(ctx,
		`UPDATE message_queue SET processed_at = NOW(), locked_at = NULL, locked_by = NULL WHERE id = $1`, id)
	return err
}

func releaseLock(ctx context.Context, id int64, errMsg string) {
	_, _ = state.pool.Exec(ctx,
		`UPDATE message_queue SET locked_at = NULL, locked_by = NULL, last_error = $2 WHERE id = $1`,
		id, errMsg)
}
