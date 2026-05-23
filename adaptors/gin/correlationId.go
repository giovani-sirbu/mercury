package ginAdaptors

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CorrelationIDHeader is the HTTP header carrying the per-request correlation id
// across service hops. Clients (web, mobile) and upstream services set it; this
// service forwards it on outbound HTTP calls and on pub/sub messages it produces.
const CorrelationIDHeader = "X-Correlation-ID"

// CorrelationIDKey is the gin context / context.Context key under which the
// resolved correlation id is stored.
const CorrelationIDKey = "correlation_id"

// CorrelationID is gin middleware that resolves the request's correlation id.
// It reads X-Correlation-ID from the incoming request if present, otherwise
// generates a new UUID v4. The id is stored on the gin context for downstream
// handlers and echoed back on the response so clients can log the same id.
//
// Register globally in routes.Init before any other middleware so logs from
// auth / business handlers all carry the id.
func CorrelationID(c *gin.Context) {
	id := c.GetHeader(CorrelationIDHeader)
	if id == "" {
		id = uuid.NewString()
	}
	c.Set(CorrelationIDKey, id)
	c.Header(CorrelationIDHeader, id)
	c.Next()
}

// GetCorrelationID returns the correlation id stored on the gin context, or
// an empty string if the middleware did not run for this request.
func GetCorrelationID(c *gin.Context) string {
	if v, ok := c.Get(CorrelationIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
