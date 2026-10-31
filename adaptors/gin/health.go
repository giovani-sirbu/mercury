package ginAdaptors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	commonLog "github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/storage/memory"
	"gorm.io/gorm"
)

// readyCheckTimeout caps one dependency check.
const readyCheckTimeout = 2 * time.Second

// HealthCheck is one dependency check; a non-nil error marks the service not ready.
type HealthCheck func() error

// Liveness is the /healthz handler: it proves the process runs and touches no dependency.
func Liveness() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	}
}

// Readiness is the /readyz handler: the checks run in order and the first
// failure answers 503 with its name.
func Readiness(checks ...NamedCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, nc := range checks {
			if err := nc.Check(); err != nil {
				// Logged server-side; never leaked to the caller.
				commonLog.Error(err.Error(), "Readiness", nc.Name)
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status": "not ready",
					"failed": nc.Name,
				})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}

// NamedCheck binds a check to the name the readyz response reports.
type NamedCheck struct {
	Name  string
	Check HealthCheck
}

// Check builds a NamedCheck.
func Check(name string, check HealthCheck) NamedCheck {
	return NamedCheck{Name: name, Check: check}
}

// DBCheck pings the sql.DB inside readyCheckTimeout; nil skips.
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

// CacheCheck pings Redis through the Memory wrapper.
func CacheCheck(cache *memory.Memory) NamedCheck {
	return NamedCheck{
		Name: "redis",
		Check: func() error {
			if cache == nil {
				return nil
			}
			// Ping has no context variant, so cap it here.
			done := make(chan error, 1)
			go func() { done <- cache.Ping() }()
			select {
			case err := <-done:
				return err
			case <-time.After(readyCheckTimeout):
				return fmt.Errorf("redis ping timed out after %s", readyCheckTimeout)
			}
		},
	}
}

// BrokerCheck pings the shared messagebroker pool.
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
