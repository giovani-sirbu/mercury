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

// startKey is the typed context key of the per-query start time between the Before and After callbacks.
type startKey struct{}

// poolSampleInterval is how often sql.DBStats is sampled into the gauges.
const poolSampleInterval = 15 * time.Second

// InstrumentGORM wires the Prometheus metrics into a GORM DB: Before/After
// callbacks on every operation emit db_query_duration_seconds and
// db_queries_total, and a background goroutine samples sql.DBStats into the
// db_connections_* and db_wait_* gauges. Call it once after gorm.Open.
func InstrumentGORM(db *gorm.DB, serviceName string) {
	if db == nil {
		log.Error("InstrumentGORM called with nil db", "InstrumentGORM", "mercury/db")
		return
	}

	// GORM exposes one unexported callback chain per operation, so the hooks are attached inline.
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

	// The pool stats sampler; it exits on a panic in Stats.
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

// observe emits the duration and the counter of one GORM callback, from the After hook.
func observe(tx *gorm.DB, serviceName, op string) {
	start, ok := tx.Statement.Context.Value(startKey{}).(time.Time)
	if !ok {
		return
	}
	table := tx.Statement.Table
	if table == "" {
		// A raw query has no parsed table.
		table = "raw"
	}
	status := "ok"
	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		// A missing row is not an error.
		status = "error"
	}

	metrics.DBQueryDuration.
		WithLabelValues(serviceName, op, table).
		Observe(time.Since(start).Seconds())

	metrics.DBQueriesTotal.
		WithLabelValues(serviceName, op, table, status).
		Inc()
}

// samplePool snapshots sql.DBStats into the connection-pool gauges.
func samplePool(sqlDB *sql.DB, serviceName string) {
	s := sqlDB.Stats()
	metrics.DBConnectionsOpen.WithLabelValues(serviceName).Set(float64(s.OpenConnections))
	metrics.DBConnectionsInUse.WithLabelValues(serviceName).Set(float64(s.InUse))
	metrics.DBConnectionsIdle.WithLabelValues(serviceName).Set(float64(s.Idle))
	metrics.DBConnectionsMaxOpen.WithLabelValues(serviceName).Set(float64(s.MaxOpenConnections))
	metrics.DBWaitCount.WithLabelValues(serviceName).Set(float64(s.WaitCount))
	metrics.DBWaitDurationSeconds.WithLabelValues(serviceName).Set(s.WaitDuration.Seconds())
}
