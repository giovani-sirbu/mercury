package messagebroker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	commonLog "github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/metrics"
	"github.com/jackc/pgx/v5"
)

// listen opens a LISTEN connection for prefixedTopic, drains any pending
// rows on startup, then loops on notifications and a ticker until the
// connection drops or the context is cancelled.
func (m MessageBroker) listen(prefixedTopic string, handler ContextHandler) error {
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
		defer func() {
			if rec := recover(); rec != nil {
				errCh <- fmt.Errorf("WaitForNotification panic: %v\n%s", rec, debug.Stack())
			}
		}()
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

func claimAndRun(ctx context.Context, topic, serviceName string, handler ContextHandler) {
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

func claimBatch(ctx context.Context, topic, serviceName string, handler ContextHandler) (int, error) {
	pool := currentPool()
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
        SELECT id, payload, COALESCE(correlation_id, ''), locked_at, created_at FROM message_queue
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
		id            int64
		payload       []byte
		correlationID string
		// staleReclaim is true when this row had a non-NULL locked_at at
		// claim time — meaning a previous worker locked it and never marked
		// it processed within staleLockAfter. Used to emit the
		// messagebroker_stale_lock_reclaimed_total counter so ops can see
		// when handlers are crashing or hanging.
		staleReclaim bool
		// createdAt is the row's produce timestamp from message_queue. Used
		// to observe BrokerE2ELag (produce → dispatch) per message so we
		// can tell pipeline lag apart from handler latency.
		createdAt time.Time
	}
	var claimed []claimedRow
	for rows.Next() {
		var id int64
		var raw json.RawMessage
		var cid string
		var lockedAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&id, &raw, &cid, &lockedAt, &createdAt); err != nil {
			rows.Close()
			return 0, err
		}
		claimed = append(claimed, claimedRow{
			id:            id,
			payload:       []byte(raw),
			correlationID: cid,
			staleReclaim:  lockedAt != nil,
			createdAt:     createdAt,
		})
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
		if r.staleReclaim {
			metrics.BrokerStaleLockReclaimed.WithLabelValues(serviceName, topic).Inc()
		}
		// Observe produce→dispatch lag before handing off to runHandler so
		// the metric reflects pipeline delay specifically — not handler work.
		metrics.BrokerE2ELag.
			WithLabelValues(serviceName, topic).
			Observe(time.Since(r.createdAt).Seconds())
		runHandler(ctx, topic, r.id, r.correlationID, r.payload, handler)
	}
	return len(claimed), nil
}

func runHandler(ctx context.Context, topic string, id int64, correlationID string, payload []byte, handler ContextHandler) {
	// Inject correlation id into the per-message context so the handler and
	// anything it calls downstream (log, ProduceWithCorrelation, outbound HTTP)
	// can fish it back out via log.CorrelationFromContext.
	ctx = commonLog.ContextWithCorrelation(ctx, correlationID)

	// Time the handler call. `outcome` is set after the fact below — either
	// "ok" on the success path or "panic" inside the recover. We observe in
	// a deferred closure so a panic doesn't bypass the metric (a panic that
	// skipped observation would leave the histogram silent during the very
	// incidents observability is supposed to catch).
	start := time.Now()
	outcome := "ok"

	serviceName := state.serviceName
	defer func() {
		metrics.BrokerHandlerDuration.
			WithLabelValues(serviceName, topic, outcome).
			Observe(time.Since(start).Seconds())
	}()

	defer func() {
		if rec := recover(); rec != nil {
			outcome = "panic"
			msg := fmt.Sprintf("handler panic on %s id=%d: %v\n%s", topic, id, rec, debug.Stack())
			commonLog.Error(msg, "runHandler", "Consumer", commonLog.WithCorrelation(correlationID))
			releaseLock(ctx, id, fmt.Sprintf("panic: %v", rec))
		}
	}()
	handler(ctx, payload)
	if err := markProcessed(ctx, id); err != nil {
		commonLog.Error(fmt.Sprintf("mark processed failed id=%d: %s", id, err), "runHandler", "Consumer", commonLog.WithCorrelation(correlationID))
		return
	}
	commonLog.Info(fmt.Sprintf("Consumed on topic: %s", topic), "", "Consumer", commonLog.WithCorrelation(correlationID))
}

func markProcessed(ctx context.Context, id int64) error {
	pool := currentPool()
	if pool == nil {
		return fmt.Errorf("broker pool not initialized")
	}
	_, err := pool.Exec(ctx,
		`UPDATE message_queue SET processed_at = NOW(), locked_at = NULL, locked_by = NULL WHERE id = $1`, id)
	return err
}

func releaseLock(ctx context.Context, id int64, errMsg string) {
	pool := currentPool()
	if pool == nil {
		return
	}
	_, _ = pool.Exec(ctx,
		`UPDATE message_queue SET locked_at = NULL, locked_by = NULL, last_error = $2 WHERE id = $1`,
		id, errMsg)
}
