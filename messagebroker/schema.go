package messagebroker

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS message_queue (
    id             BIGSERIAL PRIMARY KEY,
    topic          TEXT        NOT NULL,
    payload        JSONB       NOT NULL,
    correlation_id TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at      TIMESTAMPTZ,
    locked_by      TEXT,
    processed_at   TIMESTAMPTZ,
    attempts       INT         NOT NULL DEFAULT 0,
    last_error     TEXT
);
-- Backfill the column on databases created before correlation_id existed.
-- IF NOT EXISTS makes the statement idempotent across restarts and downgrades.
ALTER TABLE message_queue ADD COLUMN IF NOT EXISTS correlation_id TEXT;
CREATE INDEX IF NOT EXISTS idx_mq_pending
    ON message_queue (topic, created_at)
    WHERE processed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_mq_cleanup
    ON message_queue (processed_at)
    WHERE processed_at IS NOT NULL;
`

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schemaSQL)
	return err
}
