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
	// Only honor a client-supplied id if it is a bounded, safe token; otherwise
	// generate a fresh UUID. This keeps cross-service tracing intact while
	// preventing arbitrary client-controlled values from landing in logs and
	// response headers (log forging / header injection).
	if !isValidCorrelationID(id) {
		id = uuid.NewString()
	}
	c.Set(CorrelationIDKey, id)
	c.Header(CorrelationIDHeader, id)
	c.Next()
}

// isValidCorrelationID accepts only short alphanumeric/dash/underscore ids so a
// forwarded correlation id cannot smuggle control characters or unbounded data
// into logs or the response header.
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
