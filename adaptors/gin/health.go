package ginAdaptors

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/storage/memory"
	"gorm.io/gorm"
)

// readyCheckTimeout caps how long any single dependency check is allowed to
// hang. The whole readiness probe should complete inside the k8s probe
// timeout (typically 1s) — keep this generous enough for healthy systems
// but short enough that a stuck downstream is flagged quickly.
const readyCheckTimeout = 2 * time.Second

// HealthCheck is a single dependency check. Returning a non-nil error marks
// the service as not-ready. Implementations should be fast (sub-second) —
// the readiness probe is called on a tight cadence by k8s.
type HealthCheck func() error

// Liveness returns a gin handler suitable for the `/healthz` endpoint.
// Liveness only proves the process is running and the gin loop is responsive —
// it intentionally does NOT touch DB / Redis / broker, because those failures
// should NOT cause the pod to be killed and restarted. Use Readiness for that.
//
// Pair with k8s livenessProbe so a deadlocked process gets restarted but a
// transient downstream blip does not.
func Liveness() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	}
}

// Readiness returns a gin handler suitable for the `/readyz` endpoint.
// Runs each check sequentially; returns 503 with the failing check name on
// the first error. All checks pass → 200.
//
// k8s should call this on readinessProbe — pods that fail readiness are
// pulled out of the Service endpoint list but not restarted.
//
// Each service passes the checks for its real dependencies:
//
//	r.GET("/readyz", adapter.Readiness(
//	    adapter.Check("db", func() error { return server.DB.Exec("SELECT 1").Error }),
//	    adapter.Check("redis", server.Cache.Ping),
//	    adapter.Check("broker", func() error { return server.Broker.Ping(ctx) }),
//	))
func Readiness(checks ...NamedCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, nc := range checks {
			if err := nc.Check(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status":   "not ready",
					"failed":   nc.Name,
					"error":    err.Error(),
				})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}

// NamedCheck binds a check function to a human-readable name so the readyz
// response can identify which dependency is down.
type NamedCheck struct {
	Name  string
	Check HealthCheck
}

// Check is a small constructor for NamedCheck so call sites read like data:
//
//	adapter.Readiness(
//	    adapter.Check("db", dbPing),
//	    adapter.Check("redis", redisPing),
//	)
func Check(name string, check HealthCheck) NamedCheck {
	return NamedCheck{Name: name, Check: check}
}

// DBCheck builds a readiness check that pings the underlying sql.DB inside a
// short timeout. Pass nil to skip — useful for services without a DB (hermes).
func DBCheck(db *gorm.DB) NamedCheck {
	return NamedCheck{
		Name: "db",
		Check: func() error {
			if db == nil {
				return nil
			}
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), readyCheckTimeout)
			defer cancel()
			return sqlDB.PingContext(ctx)
		},
	}
}

// CacheCheck builds a readiness check that pings Redis through mercury's
// Memory wrapper.
func CacheCheck(cache *memory.Memory) NamedCheck {
	return NamedCheck{
		Name: "redis",
		Check: func() error {
			if cache == nil {
				return nil
			}
			return cache.Ping()
		},
	}
}

// BrokerCheck builds a readiness check that pings the shared messagebroker
// pool. The pool is package-level state inside mercury/messagebroker, so no
// argument is needed.
func BrokerCheck() NamedCheck {
	return NamedCheck{
		Name: "broker",
		Check: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), readyCheckTimeout)
			defer cancel()
			return messagebroker.Ping(ctx)
		},
	}
}
