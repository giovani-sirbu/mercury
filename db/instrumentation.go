// Package db provides GORM instrumentation that emits db_query_duration_seconds,
// db_queries_total, and the db_connections_* gauges used by the HERMATIC Database
// dashboard. One InstrumentGORM call wires the whole picture per service.
package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/metrics"
	"gorm.io/gorm"
)

// startKey is the context key under which we stash the per-query start
// timestamp between the Before and After GORM callbacks. Using a typed
// key (not a bare string) avoids collisions with other packages' context
// values.
type startKey struct{}

// poolSampleInterval is how often we snapshot sql.DBStats into the gauges.
// 15s matches Prometheus's default scrape interval — anything faster
// would be wasted (the scraper can't tell the difference) and slower
// would mean alerts on pool saturation react late.
const poolSampleInterval = 15 * time.Second

// InstrumentGORM wires Prometheus metrics into a GORM DB:
//
//   - Per-operation Before/After callbacks emit db_query_duration_seconds
//     and db_queries_total{operation,table,status}.
//   - A background goroutine samples sql.DBStats every poolSampleInterval
//     and updates the db_connections_* / db_wait_* gauges.
//
// Call once after gorm.Open returns, passing the service name. Safe to
// call multiple times (idempotent name registration via GORM's plugin
// pattern would be over-engineered for our case — we just register the
// callbacks with unique names).
func InstrumentGORM(db *gorm.DB, serviceName string) {
	if db == nil {
		log.Error("InstrumentGORM called with nil db", "InstrumentGORM", "mercury/db")
		return
	}

	// Register Before/After on each lifecycle. GORM exposes a separate
	// callback chain per operation (Query, Create, Update, Delete, Row,
	// Raw) and the chain's concrete type is unexported, so we can't
	// abstract this through an interface — inline calls per operation.
	// Each chain only fires when GORM is doing that specific operation,
	// so the same hook code is safe to attach to all six.
	before := func(tx *gorm.DB) {
		tx.Statement.Context = context.WithValue(tx.Statement.Context, startKey{}, time.Now())
	}
	after := func(op string) func(*gorm.DB) {
		return func(tx *gorm.DB) { observe(tx, serviceName, op) }
	}
	_ = db.Callback().Query().Before("gorm:query").Register("mercury:before:query", before)
	_ = db.Callback().Query().After("gorm:query").Register("mercury:after:query", after("query"))
	_ = db.Callback().Create().Before("gorm:create").Register("mercury:before:create", before)
	_ = db.Callback().Create().After("gorm:create").Register("mercury:after:create", after("create"))
	_ = db.Callback().Update().Before("gorm:update").Register("mercury:before:update", before)
	_ = db.Callback().Update().After("gorm:update").Register("mercury:after:update", after("update"))
	_ = db.Callback().Delete().Before("gorm:delete").Register("mercury:before:delete", before)
	_ = db.Callback().Delete().After("gorm:delete").Register("mercury:after:delete", after("delete"))
	_ = db.Callback().Row().Before("gorm:row").Register("mercury:before:row", before)
	_ = db.Callback().Row().After("gorm:row").Register("mercury:after:row", after("row"))
	_ = db.Callback().Raw().Before("gorm:raw").Register("mercury:before:raw", before)
	_ = db.Callback().Raw().After("gorm:raw").Register("mercury:after:raw", after("raw"))

	// Kick off pool stats sampler. Bound the goroutine to the DB pool's
	// lifetime by letting it exit if sqlDB.Stats() panics (which it won't
	// in practice — we just defer-recover for safety).
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("could not get sql.DB from gorm: "+err.Error(), "InstrumentGORM", "mercury/db")
		return
	}
	go func() {
		defer func() { _ = recover() }()
		t := time.NewTicker(poolSampleInterval)
		defer t.Stop()
		samplePool(sqlDB, serviceName) // immediate sample so /metrics shows data at boot
		for range t.C {
			samplePool(sqlDB, serviceName)
		}
	}()
}

// observe emits the duration histogram + counter for a single GORM
// callback invocation. Called from the After hook for each lifecycle.
func observe(tx *gorm.DB, serviceName, op string) {
	start, ok := tx.Statement.Context.Value(startKey{}).(time.Time)
	if !ok {
		return
	}
	table := tx.Statement.Table
	if table == "" {
		// Raw queries (`db.Raw(...)`) don't always have a parsed table.
		// Use a placeholder so the metric still emits without exploding
		// the cardinality of "unknown" forever.
		table = "raw"
	}
	status := "ok"
	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		// ErrRecordNotFound is the common "no row" case — not a real
		// error from an ops perspective. Treat it as ok so alerts on
		// status=error fire on real failures only.
		status = "error"
	}

	metrics.DBQueryDuration.
		WithLabelValues(serviceName, op, table).
		Observe(time.Since(start).Seconds())

	metrics.DBQueriesTotal.
		WithLabelValues(serviceName, op, table, status).
		Inc()
}

// samplePool snapshots sql.DBStats into the connection-pool gauges. The
// stats struct is cheap to retrieve (atomic loads) so 15s sampling is
// effectively free.
func samplePool(sqlDB *sql.DB, serviceName string) {
	s := sqlDB.Stats()
	metrics.DBConnectionsOpen.WithLabelValues(serviceName).Set(float64(s.OpenConnections))
	metrics.DBConnectionsInUse.WithLabelValues(serviceName).Set(float64(s.InUse))
	metrics.DBConnectionsIdle.WithLabelValues(serviceName).Set(float64(s.Idle))
	metrics.DBConnectionsMaxOpen.WithLabelValues(serviceName).Set(float64(s.MaxOpenConnections))
	metrics.DBWaitCount.WithLabelValues(serviceName).Set(float64(s.WaitCount))
	metrics.DBWaitDurationSeconds.WithLabelValues(serviceName).Set(s.WaitDuration.Seconds())
}
