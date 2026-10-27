package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DBQueryDuration times every GORM operation, partitioned by operation and table.
var DBQueryDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "DB query duration partitioned by GORM operation + target table.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16), // 1ms → 32s
	},
	[]string{"service", "operation", "table"},
)

// DBQueriesTotal counts queries per operation, table and status.
var DBQueriesTotal = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "db_queries_total",
		Help: "Total DB queries partitioned by operation + target table.",
	},
	[]string{"service", "operation", "table", "status"}, // status: ok | error
)

// DBConnectionsOpen mirrors sql.DBStats, sampled by InstrumentGORM.
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

// DBWaitCount and DBWaitDurationSeconds capture pool starvation; gauges
// because sql.DBStats exposes cumulative totals.
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
