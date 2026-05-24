package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DBQueryDuration is observed by the GORM callback wrapper around every
// query, INSERT, UPDATE, DELETE, RAW or row. The operation label captures
// the GORM lifecycle hook so callers can pivot "where is the slowness
// coming from" between read and write paths. The table label is sourced
// from the GORM statement (`Schema.Table`) — bounded cardinality because
// it matches the small set of HERMATIC models.
//
//	histogram_quantile(0.99, sum by (operation, table, le) (rate(db_query_duration_seconds_bucket[5m])))
//
// catches N+1, full-table scans, and individual slow queries without
// requiring loglines per query.
var DBQueryDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "DB query duration partitioned by GORM operation + target table.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16), // 1ms → 32s
	},
	[]string{"service", "operation", "table"},
)

// DBQueriesTotal counts queries per operation + table. Combined with
// DBQueryDuration's count series it's redundant, but having an explicit
// counter makes "how many SELECTs per request" easier to express and
// alert on (e.g. spike in queries/request often = N+1 regression).
var DBQueriesTotal = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "db_queries_total",
		Help: "Total DB queries partitioned by operation + target table.",
	},
	[]string{"service", "operation", "table", "status"}, // status: ok | error
)

// DBConnectionsOpen mirrors sql.DBStats. Sampled every 15s by a background
// goroutine started by InstrumentGORM.
//
//	db_connections_in_use{service} / db_connections_max_open{service}  > 0.8
//
// is the canonical "pool saturating" alert.
var DBConnectionsOpen = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_connections_open",
		Help: "Open connections in the GORM pool (sql.DBStats.OpenConnections).",
	},
	[]string{"service"},
)

var DBConnectionsInUse = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_connections_in_use",
		Help: "Connections currently in use (sql.DBStats.InUse).",
	},
	[]string{"service"},
)

var DBConnectionsIdle = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_connections_idle",
		Help: "Idle connections in the pool (sql.DBStats.Idle).",
	},
	[]string{"service"},
)

var DBConnectionsMaxOpen = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_connections_max_open",
		Help: "Configured pool max (sql.DBStats.MaxOpenConnections). 0 means unlimited.",
	},
	[]string{"service"},
)

// DBWaitCount + DBWaitDuration capture pool starvation: when
// MaxOpenConnections is hit, new acquires block and these climb. A rising
// WaitDuration without a rising InUse cap means callers are holding
// connections too long (a leak or a transaction not closing).
//
// Modelled as gauges (not counters) because sql.DBStats exposes the raw
// cumulative totals; the gauge sampler simply Set()s the latest value,
// and PromQL's rate()/increase() over the gauge gives the per-second
// derivative we'd want from a counter. Treating it as a counter would
// require persisting incremental deltas across sampler ticks.
var DBWaitCount = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_wait_count",
		Help: "Cumulative number of times sql had to wait for a connection (sql.DBStats.WaitCount).",
	},
	[]string{"service"},
)

var DBWaitDurationSeconds = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "db_wait_duration_seconds",
		Help: "Cumulative time blocked waiting for a connection (sql.DBStats.WaitDuration).",
	},
	[]string{"service"},
)
