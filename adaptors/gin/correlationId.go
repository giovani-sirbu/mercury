package ginAdaptors

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CorrelationIDHeader carries the per-request correlation id across service hops.
const CorrelationIDHeader = "X-Correlation-ID"

// CorrelationIDKey is the gin context key the resolved correlation id is stored under.
const CorrelationIDKey = "correlation_id"

// CorrelationID is the middleware that resolves the request's correlation id:
// the incoming header when it is a safe token, else a fresh UUID, stored on
// the context and echoed on the response. Register it before any other middleware.
func CorrelationID(c *gin.Context) {
	id := c.GetHeader(CorrelationIDHeader)
	// A client-supplied id is honoured only as a bounded, safe token (no log forging).
	if !isValidCorrelationID(id) {
		id = uuid.NewString()
	}
	c.Set(CorrelationIDKey, id)
	c.Header(CorrelationIDHeader, id)
	c.Next()
}

// isValidCorrelationID accepts only short alphanumeric, dash or underscore ids.
func isValidCorrelationID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// GetCorrelationID returns the correlation id on the gin context, or "" when the middleware did not run.
func GetCorrelationID(c *gin.Context) string {
	if v, ok := c.Get(CorrelationIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
